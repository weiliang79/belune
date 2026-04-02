package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/git"
	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/buildlog"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type deployPayload struct {
	ApplicationID    string `json:"application_id"`
	DeploymentID     string `json:"deployment_id"`
	RollbackImageTag string `json:"rollback_image_tag,omitempty"` // non-empty = skip build, redeploy this image
}

func (h *TaskHandler) HandleDeployTask(ctx context.Context, t *asynq.Task) error {
	var payload deployPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal deploy payload: %w", err)
	}

	slog.Info("handling deploy task", "application_id", payload.ApplicationID, "deployment_id", payload.DeploymentID)

	applicationID := parseUUID(payload.ApplicationID)
	deploymentID := parseUUID(payload.DeploymentID)

	// Fetch application details with project slug
	appRow, err := h.Queries.GetApplicationWithProjectSlug(ctx, applicationID)
	if err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("fetch application: %v", err))
		return fmt.Errorf("get application (permanent): %w: %w", err, asynq.SkipRetry)
	}
	// Map row to a usable application reference
	app := generated.Application{
		ID: appRow.ID, ProjectID: appRow.ProjectID, Name: appRow.Name,
		Type: appRow.Type, SourceRepo: appRow.SourceRepo, SourceImage: appRow.SourceImage,
		DockerfilePath: appRow.DockerfilePath, BuildType: appRow.BuildType,
		BuilderImage: appRow.BuilderImage, CustomBuildpacks: appRow.CustomBuildpacks,
		Status: appRow.Status,
		CpuLimit: appRow.CpuLimit, MemoryLimit: appRow.MemoryLimit,
		GitCredentialsEncrypted: appRow.GitCredentialsEncrypted,
		HealthCheckPath:         appRow.HealthCheckPath,
	}

	containerName := naming.ContainerName(appRow.ProjectSlug, appRow.Slug, payload.ApplicationID)

	// Stop and remove existing container if any (try all naming formats)
	intermediateContainerName := naming.IntermediateContainerName(appRow.ProjectSlug, payload.ApplicationID)
	oldContainerName := naming.OldContainerName(payload.ApplicationID)
	for _, name := range []string{oldContainerName, intermediateContainerName, containerName} {
		if err := h.Runtime.StopContainer(ctx, name); err != nil {
			slog.Debug("could not stop container before deploy (may not exist)", "container", name, "error", err)
		}
		if err := h.Runtime.RemoveContainer(ctx, name); err != nil {
			slog.Debug("could not remove container before deploy (may not exist)", "container", name, "error", err)
		}
	}

	// Ensure project-scoped network exists (idempotent)
	projectNetwork := naming.ProjectNetworkName(appRow.ProjectSlug)
	if err := h.Runtime.CreateNetwork(ctx, projectNetwork); err != nil {
		slog.Debug("could not create project network (may already exist)", "network", projectNetwork, "error", err)
	}
	// Ensure shared infra network exists (used by Caddy to reach containers)
	if err := h.Runtime.CreateNetwork(ctx, "paas-infra"); err != nil {
		slog.Debug("could not create paas-infra network (may already exist)", "error", err)
	}

	// Fetch and decrypt env vars: project-level first (base), then app-level (override)
	env := make(map[string]string)

	projectEnvVars, err := h.Queries.ListProjectEnvVars(ctx, app.ProjectID)
	if err != nil {
		slog.Warn("failed to fetch project env vars, continuing without them", "error", err)
	}
	for _, ev := range projectEnvVars {
		decrypted, err := crypto.Decrypt(ev.ValueEncrypted, h.EncryptionKey)
		if err != nil {
			slog.Warn("failed to decrypt project env var, skipping", "key", ev.Key, "error", err)
			continue
		}
		env[ev.Key] = string(decrypted)
	}

	appEnvVars, err := h.Queries.ListEnvVarsByApplication(ctx, applicationID)
	if err != nil {
		slog.Warn("failed to fetch app env vars, continuing without them", "error", err)
	}
	for _, ev := range appEnvVars {
		decrypted, err := crypto.Decrypt(ev.ValueEncrypted, h.EncryptionKey)
		if err != nil {
			slog.Warn("failed to decrypt env var, skipping", "key", ev.Key, "error", err)
			continue
		}
		env[ev.Key] = string(decrypted)
	}

	var imageName string
	var commitSHA string

	if payload.RollbackImageTag != "" {
		// Rollback: skip build entirely, redeploy a previously stored image.
		imageName = payload.RollbackImageTag
		slog.Info("rolling back to image", "image", imageName, "application_id", payload.ApplicationID)
		h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:     deploymentID,
			Status: status.DeploymentDeploying,
		})
	} else {
	switch app.Type {
	case "image":
		// Image apps skip build — go straight to deploying
		h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:     deploymentID,
			Status: status.DeploymentDeploying,
		})
		imageName = app.SourceImage.String
		slog.Info("pulling image", "image", imageName)
		pullCtx, pullCancel := context.WithTimeout(ctx, time.Duration(h.Config.ImagePullTimeoutMinutes)*time.Minute)
		defer pullCancel()
		if err := h.Runtime.PullImage(pullCtx, imageName); err != nil {
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

		buildCtx, buildCancel := context.WithTimeout(ctx, time.Duration(h.Config.BuildTimeoutMinutes)*time.Minute)
		defer buildCancel()

		// Decrypt git token if present (for private repositories)
		var gitToken string
		if len(app.GitCredentialsEncrypted) > 0 {
			tokenBytes, decErr := crypto.Decrypt(app.GitCredentialsEncrypted, h.EncryptionKey)
			if decErr != nil {
				slog.Warn("failed to decrypt git credentials, cloning without token", "error", decErr)
			} else {
				gitToken = string(tokenBytes)
			}
		}

		slog.Info("cloning repository", "repo", app.SourceRepo.String, "dest", tmpDir)
		cloneResult, err := git.Clone(buildCtx, app.SourceRepo.String, tmpDir, "", gitToken)
		if err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("git clone: %v", err))
			return fmt.Errorf("git clone: %w", err)
		}
		commitSHA = cloneResult.CommitSHA
		slog.Info("cloned repository", "commit", commitSHA)

		// Parse custom buildpacks from application config
		var customBuildpacks []string
		if len(app.CustomBuildpacks) > 0 {
			if err := json.Unmarshal(app.CustomBuildpacks, &customBuildpacks); err != nil {
				slog.Debug("could not parse custom buildpacks, using defaults", "app_id", payload.ApplicationID, "error", err)
			}
		}

		// Set up build log streaming via Redis pub/sub
		pub := buildlog.NewPublisher(h.RedisClient, payload.DeploymentID)
		logWriter := buildlog.NewLineWriter(pub, buildCtx)

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

		// Update status to building before build starts
		h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:     deploymentID,
			Status: status.DeploymentBuilding,
		})

		slog.Info("building image", "tag", imageName)
		result, err := h.Chain.Build(buildCtx, buildOpts)
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

		// Build done — now transition to deploying before container creation
		h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:     deploymentID,
			Status: status.DeploymentDeploying,
		})

	default:
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("unknown application type: %s", app.Type))
		return fmt.Errorf("unknown application type %s (permanent): %w", app.Type, asynq.SkipRetry)
	}
	} // end else (non-rollback)

	// Store the image tag for future rollback support.
	h.Queries.UpdateDeploymentImageTag(ctx, generated.UpdateDeploymentImageTagParams{
		ID:       deploymentID,
		ImageTag: pgtype.Text{String: imageName, Valid: true},
	})

	// Create and start container
	containerID, err := h.Runtime.CreateContainer(ctx, runtime.ContainerConfig{
		Name:            containerName,
		Image:           imageName,
		Env:             env,
		Ports:           map[string]string{},
		Network:         naming.ProjectNetworkName(appRow.ProjectSlug),
		Labels:          map[string]string{"application-id": payload.ApplicationID},
		CPULimit:        app.CpuLimit,
		MemoryLimit:     app.MemoryLimit,
		HealthCheckPath: app.HealthCheckPath.String,
	})
	if err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("create container: %v", err))
		return fmt.Errorf("create container: %w", err)
	}

	if err := h.Runtime.StartContainer(ctx, containerID); err != nil {
		h.failDeployment(ctx, deploymentID, fmt.Sprintf("start container: %v", err))
		return fmt.Errorf("start container: %w", err)
	}

	// Connect to shared infra network so Caddy can reach the container
	if err := h.Runtime.ConnectContainerToNetwork(ctx, containerID, "paas-infra"); err != nil {
		slog.Warn("could not connect container to paas-infra network", "container", containerID, "error", err)
	}

	// Health check polling: poll the container's health endpoint until healthy or timeout
	if app.HealthCheckPath.Valid && app.HealthCheckPath.String != "" {
		healthURL := fmt.Sprintf("http://%s:%d%s", containerName, app.Port, app.HealthCheckPath.String)
		slog.Info("waiting for container health check", "url", healthURL)
		if err := pollHealthCheck(ctx, healthURL, 60*time.Second); err != nil {
			h.failDeployment(ctx, deploymentID, fmt.Sprintf("health check failed: %v", err))
			return fmt.Errorf("health check: %w", err)
		}
		slog.Info("container health check passed", "container", containerName)
	}

	// Add proxy route if the application has domains
	domains, err := h.Queries.ListDomainsByApplication(ctx, applicationID)
	if err == nil {
		for _, domain := range domains {
			h.Proxy.AddRoute(ctx, proxy.RouteConfig{
				Hostname:  domain.Hostname,
				TargetURL: fmt.Sprintf("http://%s:%d", containerName, app.Port),
				TLS:       domain.SslEnabled,
			})
		}
	}

	// Mark deployment as success and application as running — both or neither
	if err := store.WithTx(ctx, h.DB, func(q *generated.Queries) error {
		q.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:     deploymentID,
			Status: status.DeploymentSuccess,
		})
		q.UpdateApplicationStatus(ctx, generated.UpdateApplicationStatusParams{
			ID:     applicationID,
			Status: status.ApplicationRunning,
		})
		return nil
	}); err != nil {
		slog.Error("failed to commit deploy success status", "application_id", payload.ApplicationID, "error", err)
	}

	slog.Info("deploy completed",
		"application_id", payload.ApplicationID,
		"deployment_id", payload.DeploymentID,
		"container", containerID,
		"commit", commitSHA,
	)
	return nil
}

func (h *TaskHandler) failDeployment(ctx context.Context, deploymentID pgtype.UUID, errMsg string) {
	retried, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)

	slog.Error("deployment failed",
		"deployment_id", fmt.Sprintf("%v", deploymentID),
		"error", errMsg,
		"retry", retried,
		"max_retry", maxRetry,
	)

	// Only mark as permanently failed after all retries are exhausted
	if retried >= maxRetry {
		h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:           deploymentID,
			Status:       status.DeploymentFailed,
			ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		})
	}
}

func parseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	u.Scan(s)
	return u
}

// pollHealthCheck repeatedly GETs url until a 2xx response is received or deadline expires.
func pollHealthCheck(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url) //nolint:noctx
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("health check at %s did not return 2xx within %s", url, timeout)
}
