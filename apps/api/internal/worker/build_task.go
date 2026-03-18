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
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type buildPayload struct {
	ServiceID    string `json:"service_id"`
	DeploymentID string `json:"deployment_id"`
}

func (h *TaskHandler) HandleBuildTask(ctx context.Context, t *asynq.Task) error {
	var payload buildPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal build payload: %w", err)
	}

	slog.Info("handling build task", "service_id", payload.ServiceID, "deployment_id", payload.DeploymentID)

	serviceID := parseUUID(payload.ServiceID)
	deploymentID := parseUUID(payload.DeploymentID)

	// Update deployment status to building
	h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
		ID:     deploymentID,
		Status: "building",
	})

	// Fetch service details
	svc, err := h.Queries.GetService(ctx, serviceID)
	if err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("fetch service: %v", err))
		return fmt.Errorf("get service: %w", err)
	}

	// Image-type services have nothing to build
	if svc.Type == "image" {
		slog.Info("skipping build for image-type service", "service_id", payload.ServiceID)
		h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:     deploymentID,
			Status: "success",
		})
		return nil
	}

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
	slog.Info("cloned repository", "commit", cloneResult.CommitSHA)

	// Fetch and decrypt env vars for build-time use
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

	// Parse custom buildpacks
	var customBuildpacks []string
	if len(svc.CustomBuildpacks) > 0 {
		_ = json.Unmarshal(svc.CustomBuildpacks, &customBuildpacks)
	}

	imageName := fmt.Sprintf("paas-%s:%s", payload.ServiceID[:8], payload.DeploymentID[:8])

	buildOpts := build.BuildOptions{
		SourceDir:      tmpDir,
		ImageTag:       imageName,
		DockerfilePath: svc.DockerfilePath.String,
		BuilderImage:   svc.BuilderImage.String,
		Buildpacks:     customBuildpacks,
		Env:            env,
	}

	slog.Info("building image", "tag", imageName)
	result, err := h.Chain.Build(ctx, buildOpts)
	if err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("build: %v", err))
		return fmt.Errorf("build: %w", err)
	}

	// Store build logs
	if result.Logs != "" {
		h.Queries.UpdateDeploymentBuildLogs(ctx, generated.UpdateDeploymentBuildLogsParams{
			ID:        deploymentID,
			BuildLogs: pgtype.Text{String: result.Logs, Valid: true},
		})
	}

	// Mark as success — no container creation
	h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
		ID:     deploymentID,
		Status: "success",
	})

	slog.Info("build completed",
		"service_id", payload.ServiceID,
		"deployment_id", payload.DeploymentID,
		"image", result.ImageTag,
		"commit", cloneResult.CommitSHA,
	)
	return nil
}
