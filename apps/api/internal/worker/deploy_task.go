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
	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/buildlog"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type deployPayload struct {
	ApplicationID string `json:"application_id"`
	DeploymentID  string `json:"deployment_id"`
}

func (h *TaskHandler) HandleDeployTask(ctx context.Context, t *asynq.Task) error {
	var payload deployPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal deploy payload: %w", err)
	}

	slog.Info("handling deploy task", "application_id", payload.ApplicationID, "deployment_id", payload.DeploymentID)

	applicationID := parseUUID(payload.ApplicationID)
	deploymentID := parseUUID(payload.DeploymentID)

	// Update deployment status to deploying
	h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
		ID:     deploymentID,
		Status: "deploying",
	})

	// Fetch application details with project slug
	appRow, err := h.Queries.GetApplicationWithProjectSlug(ctx, applicationID)
	if err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("fetch application: %v", err))
		return fmt.Errorf("get application: %w", err)
	}
	// Map row to a usable application reference
	app := generated.Application{
		ID: appRow.ID, ProjectID: appRow.ProjectID, Name: appRow.Name,
		Type: appRow.Type, SourceRepo: appRow.SourceRepo, SourceImage: appRow.SourceImage,
		DockerfilePath: appRow.DockerfilePath, BuildType: appRow.BuildType,
		BuilderImage: appRow.BuilderImage, CustomBuildpacks: appRow.CustomBuildpacks,
		Status: appRow.Status,
	}

	containerName := naming.ContainerName(appRow.ProjectSlug, appRow.Slug, payload.ApplicationID)

	// Stop and remove existing container if any (try all naming formats)
	intermediateContainerName := naming.IntermediateContainerName(appRow.ProjectSlug, payload.ApplicationID)
	oldContainerName := naming.OldContainerName(payload.ApplicationID)
	_ = h.Runtime.StopContainer(ctx, oldContainerName)
	_ = h.Runtime.RemoveContainer(ctx, oldContainerName)
	_ = h.Runtime.StopContainer(ctx, intermediateContainerName)
	_ = h.Runtime.RemoveContainer(ctx, intermediateContainerName)
	_ = h.Runtime.StopContainer(ctx, containerName)
	_ = h.Runtime.RemoveContainer(ctx, containerName)

	// Ensure the paas network exists
	_ = h.Runtime.CreateNetwork(ctx, "paas-net")

	// Fetch and decrypt env vars (needed for both build-time and runtime)
	envVars, err := h.Queries.ListEnvVarsByApplication(ctx, applicationID)
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

	var imageName string
	var commitSHA string

	switch app.Type {
	case "image":
		imageName = app.SourceImage.String
		slog.Info("pulling image", "image", imageName)
		if err := h.Runtime.PullImage(ctx, imageName); err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("pull image: %v", err))
			return fmt.Errorf("pull image: %w", err)
		}

	case "git":
		imageName = naming.ImageTag(appRow.ProjectSlug, appRow.Slug, payload.ApplicationID, payload.DeploymentID)

		// Clone the repository
		tmpDir, err := os.MkdirTemp("", "paas-build-*")
		if err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("create temp dir: %v", err))
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		slog.Info("cloning repository", "repo", app.SourceRepo.String, "dest", tmpDir)
		cloneResult, err := git.Clone(ctx, app.SourceRepo.String, tmpDir, "")
		if err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("git clone: %v", err))
			return fmt.Errorf("git clone: %w", err)
		}
		commitSHA = cloneResult.CommitSHA
		slog.Info("cloned repository", "commit", commitSHA)

		// Parse custom buildpacks from application config
		var customBuildpacks []string
		if len(app.CustomBuildpacks) > 0 {
			_ = json.Unmarshal(app.CustomBuildpacks, &customBuildpacks)
		}

		// Set up build log streaming via Redis pub/sub
		pub := buildlog.NewPublisher(h.RedisClient, payload.DeploymentID)
		logWriter := buildlog.NewLineWriter(pub, ctx)

		buildOpts := build.BuildOptions{
			SourceDir:      tmpDir,
			ImageTag:       imageName,
			DockerfilePath: app.DockerfilePath.String,
			BuilderImage:   app.BuilderImage.String,
			Buildpacks:     customBuildpacks,
			Env:            env,
			BuildType:      app.BuildType,
			LogWriter:      logWriter,
		}

		// Update status to building
		h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:     deploymentID,
			Status: "building",
		})

		slog.Info("building image", "tag", imageName)
		result, err := h.Chain.Build(ctx, buildOpts)
		logWriter.Flush()
		pub.Close(ctx)
		if err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("build: %v", err))
			return fmt.Errorf("build: %w", err)
		}

		// Store build logs in deployment record
		if result.Logs != "" {
			h.Queries.UpdateDeploymentBuildLogs(ctx, generated.UpdateDeploymentBuildLogsParams{
				ID:        deploymentID,
				BuildLogs: pgtype.Text{String: result.Logs, Valid: true},
			})
		}

		imageName = result.ImageTag
		slog.Info("build completed", "image", imageName)

	default:
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("unknown application type: %s", app.Type))
		return fmt.Errorf("unknown application type: %s", app.Type)
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

	// Add proxy route if the application has domains
	domains, err := h.Queries.ListDomainsByApplication(ctx, applicationID)
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

	// Update application status to running
	h.Queries.UpdateApplicationStatus(ctx, generated.UpdateApplicationStatusParams{
		ID:     applicationID,
		Status: "running",
	})

	slog.Info("deploy completed",
		"application_id", payload.ApplicationID,
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
