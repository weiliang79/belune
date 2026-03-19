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
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type buildPayload struct {
	ApplicationID string `json:"application_id"`
	DeploymentID  string `json:"deployment_id"`
}

func (h *TaskHandler) HandleBuildTask(ctx context.Context, t *asynq.Task) error {
	var payload buildPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal build payload: %w", err)
	}

	slog.Info("handling build task", "application_id", payload.ApplicationID, "deployment_id", payload.DeploymentID)

	applicationID := parseUUID(payload.ApplicationID)
	deploymentID := parseUUID(payload.DeploymentID)

	// Update deployment status to building
	h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
		ID:     deploymentID,
		Status: "building",
	})

	// Fetch application details with project slug
	appRow, err := h.Queries.GetApplicationWithProjectSlug(ctx, applicationID)
	if err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("fetch application: %v", err))
		return fmt.Errorf("get application: %w", err)
	}
	app := generated.Application{
		ID: appRow.ID, ProjectID: appRow.ProjectID, Name: appRow.Name,
		Type: appRow.Type, SourceRepo: appRow.SourceRepo, DockerfilePath: appRow.DockerfilePath,
		BuildType: appRow.BuildType, BuilderImage: appRow.BuilderImage,
		CustomBuildpacks: appRow.CustomBuildpacks, Status: appRow.Status,
	}

	// Image-type applications have nothing to build
	if app.Type == "image" {
		slog.Info("skipping build for image-type application", "application_id", payload.ApplicationID)
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

	slog.Info("cloning repository", "repo", app.SourceRepo.String, "dest", tmpDir)
	cloneResult, err := git.Clone(ctx, app.SourceRepo.String, tmpDir, "")
	if err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("git clone: %v", err))
		return fmt.Errorf("git clone: %w", err)
	}
	slog.Info("cloned repository", "commit", cloneResult.CommitSHA)

	// Fetch and decrypt env vars for build-time use
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

	// Parse custom buildpacks
	var customBuildpacks []string
	if len(app.CustomBuildpacks) > 0 {
		_ = json.Unmarshal(app.CustomBuildpacks, &customBuildpacks)
	}

	imageName := naming.ImageTag(appRow.ProjectSlug, appRow.Slug, payload.ApplicationID, payload.DeploymentID)

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
		LogWriter:      logWriter,
	}

	slog.Info("building image", "tag", imageName)
	result, err := h.Chain.Build(ctx, buildOpts)
	logWriter.Flush()
	pub.Close(ctx)
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
		"application_id", payload.ApplicationID,
		"deployment_id", payload.DeploymentID,
		"image", result.ImageTag,
		"commit", cloneResult.CommitSHA,
	)
	return nil
}
