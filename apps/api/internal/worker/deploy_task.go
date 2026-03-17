package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/git"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type deployPayload struct {
	ServiceID    string `json:"service_id"`
	DeploymentID string `json:"deployment_id"`
}

func (h *TaskHandler) HandleDeployTask(ctx context.Context, t *asynq.Task) error {
	var payload deployPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal deploy payload: %w", err)
	}

	slog.Info("handling deploy task", "service_id", payload.ServiceID, "deployment_id", payload.DeploymentID)

	serviceID := parseUUID(payload.ServiceID)
	deploymentID := parseUUID(payload.DeploymentID)

	// Update deployment status to deploying
	h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
		ID:     deploymentID,
		Status: "deploying",
	})

	// Fetch service details
	svc, err := h.Queries.GetService(ctx, serviceID)
	if err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("fetch service: %v", err))
		return fmt.Errorf("get service: %w", err)
	}

	containerName := fmt.Sprintf("paas-%s", payload.ServiceID[:8])

	// Stop and remove existing container if any (ignore errors)
	_ = h.Runtime.StopContainer(ctx, containerName)
	_ = h.Runtime.RemoveContainer(ctx, containerName)

	// Ensure the paas network exists
	_ = h.Runtime.CreateNetwork(ctx, "paas-net")

	var imageName string
	var commitSHA string

	switch svc.Type {
	case "image":
		imageName = svc.SourceImage.String
		slog.Info("pulling image", "image", imageName)
		if err := h.Runtime.PullImage(ctx, imageName); err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("pull image: %v", err))
			return fmt.Errorf("pull image: %w", err)
		}

	case "git":
		imageName = fmt.Sprintf("paas-%s:%s", payload.ServiceID[:8], payload.DeploymentID[:8])

		// Clone the repository
		tmpDir, err := os.MkdirTemp("", "paas-build-*")
		if err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("create temp dir: %v", err))
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		slog.Info("cloning repository", "repo", svc.SourceRepo.String, "dest", tmpDir)
		cloneResult, err := git.Clone(ctx, svc.SourceRepo.String, tmpDir, "")
		if err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("git clone: %v", err))
			return fmt.Errorf("git clone: %w", err)
		}
		commitSHA = cloneResult.CommitSHA
		slog.Info("cloned repository", "commit", commitSHA)

		// Determine builder: use override if set, otherwise auto-detect
		buildOpts := build.BuildOptions{
			SourceDir:      tmpDir,
			ImageTag:       imageName,
			DockerfilePath: svc.DockerfilePath.String,
			BuilderImage:   svc.BuilderImage.String,
			Env:            map[string]string{},
		}

		// Update status to building
		h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:     deploymentID,
			Status: "building",
		})

		slog.Info("building image", "tag", imageName)
		result, err := h.Chain.Build(ctx, buildOpts)
		if err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("build: %v", err))
			return fmt.Errorf("build: %w", err)
		}
		imageName = result.ImageTag
		slog.Info("build completed", "image", imageName)

	default:
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("unknown service type: %s", svc.Type))
		return fmt.Errorf("unknown service type: %s", svc.Type)
	}

	// Fetch env vars for the service
	envVars, err := h.Queries.ListEnvVarsByService(ctx, serviceID)
	if err != nil {
		slog.Warn("failed to fetch env vars, continuing without them", "error", err)
	}

	env := make(map[string]string)
	for _, ev := range envVars {
		decrypted, err := crypto.Decrypt(ev.ValueEncrypted, h.EncryptionKey)
		if err != nil {
			slog.Warn("failed to decrypt env var, skipping", "key", ev.Key, "error", err)
			continue
		}
		env[ev.Key] = string(decrypted)
	}

	// Create and start container
	containerID, err := h.Runtime.CreateContainer(ctx, runtime.ContainerConfig{
		Name:    containerName,
		Image:   imageName,
		Env:     env,
		Ports:   map[string]string{},
		Network: "paas-net",
	})
	if err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("create container: %v", err))
		return fmt.Errorf("create container: %w", err)
	}

	if err := h.Runtime.StartContainer(ctx, containerID); err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("start container: %v", err))
		return fmt.Errorf("start container: %w", err)
	}

	// Add proxy route if the service has domains
	domains, err := h.Queries.ListDomainsByService(ctx, serviceID)
	if err == nil {
		for _, domain := range domains {
			h.Proxy.AddRoute(ctx, proxy.RouteConfig{
				Hostname:  domain.Hostname,
				TargetURL: fmt.Sprintf("http://%s:8080", containerName),
				TLS:       domain.SslEnabled,
			})
		}
	}

	// Mark deployment as success, include commit SHA if available
	h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
		ID:     deploymentID,
		Status: "success",
	})

	// Update service status to running
	h.Queries.UpdateServiceStatus(ctx, generated.UpdateServiceStatusParams{
		ID:     serviceID,
		Status: "running",
	})

	slog.Info("deploy completed",
		"service_id", payload.ServiceID,
		"deployment_id", payload.DeploymentID,
		"container", containerID,
		"commit", commitSHA,
	)
	return nil
}

func (h *TaskHandler) failDeployment(ctx context.Context, deploymentID pgtype.UUID, errMsg string) {
	slog.Error("deployment failed", "deployment_id", fmt.Sprintf("%v", deploymentID), "error", errMsg)
	h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
		ID:           deploymentID,
		Status:       "failed",
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
	})
}

func parseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	u.Scan(s)
	return u
}
