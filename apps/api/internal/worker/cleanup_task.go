package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type cleanupPayload struct {
	ApplicationID string `json:"application_id,omitempty"`
	RetainCount   int    `json:"retain_count,omitempty"`
}

func (h *TaskHandler) HandleCleanupTask(ctx context.Context, t *asynq.Task) error {
	var payload cleanupPayload
	if len(t.Payload()) > 0 {
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			slog.Debug("could not parse cleanup payload, using defaults", "error", err)
		}
	}

	if payload.RetainCount <= 0 {
		payload.RetainCount = 3
	}

	slog.Info("handling cleanup task", "retain_count", payload.RetainCount, "application_id", payload.ApplicationID)

	var applications []generated.Application
	var err error

	if payload.ApplicationID != "" {
		appID, err := parseUUID(payload.ApplicationID)
		if err != nil {
			return errors.Join(fmt.Errorf("invalid application_id (permanent): %w", err), asynq.SkipRetry)
		}
		app, err := h.Queries.GetApplication(ctx, appID)
		if err != nil {
			return fmt.Errorf("get application: %w", err)
		}
		applications = []generated.Application{app}
	} else {
		applications, err = h.Queries.ListAllApplications(ctx)
		if err != nil {
			return fmt.Errorf("list applications: %w", err)
		}
	}

	totalRemoved := 0

	for _, app := range applications {
		applicationIDStr := formatUUID(app.ID)
		if applicationIDStr == "" {
			continue
		}

		// Get project slug for image naming
		project, err := h.Queries.GetProject(ctx, app.ProjectID)
		if err != nil {
			slog.Warn("failed to get project for application", "application_id", applicationIDStr, "error", err)
			continue
		}

		// Get deployments beyond the retain count
		oldDeployments, err := h.Queries.ListOldDeployments(ctx, generated.ListOldDeploymentsParams{
			ApplicationID: app.ID,
			Offset:        int32(payload.RetainCount),
		})
		if err != nil {
			slog.Warn("failed to list old deployments", "application_id", applicationIDStr, "error", err)
			continue
		}

		for _, dep := range oldDeployments {
			deploymentIDStr := formatUUID(dep.ID)

			// Try to remove the build image (try both old and new naming)
			if app.Type == "git" && deploymentIDStr != "" {
				imageName := naming.ImageTag(project.Slug, app.Slug, applicationIDStr, deploymentIDStr)
				oldImageName := fmt.Sprintf("paas-%s:%s", applicationIDStr[:8], deploymentIDStr[:8])
				if err := h.Runtime.RemoveImage(ctx, imageName); err != nil {
					slog.Debug("could not remove image", "image", imageName, "error", err)
				} else {
					slog.Info("removed old image", "image", imageName)
				}
				if err := h.Runtime.RemoveImage(ctx, oldImageName); err != nil {
					slog.Debug("could not remove legacy image", "image", oldImageName, "error", err)
				}
			}

			// Delete deployment record
			if err := h.Queries.DeleteDeployment(ctx, dep.ID); err != nil {
				slog.Warn("failed to delete deployment", "deployment_id", deploymentIDStr, "error", err)
			} else {
				totalRemoved++
			}
		}
	}

	// Prune dangling images and volumes
	if err := h.Runtime.PruneImages(ctx); err != nil {
		slog.Warn("failed to prune images", "error", err)
	}
	if err := h.Runtime.PruneVolumes(ctx); err != nil {
		slog.Warn("failed to prune volumes", "error", err)
	}

	// Remove orphan containers: managed containers with no matching application in DB.
	h.cleanupOrphanContainers(ctx)

	// Reap idle preview apps. Skipped entirely when targeting a single app
	// (the caller wanted a narrow cleanup) or when the feature is disabled.
	if payload.ApplicationID == "" {
		h.cleanupStalePreviews(ctx)
	}

	slog.Info("cleanup completed",
		"applications_processed", len(applications),
		"deployments_removed", totalRemoved,
	)
	return nil
}

// cleanupStalePreviews deletes preview applications whose last_activity_at is
// older than PreviewIdleDays. Delegates to ApplicationService.Delete which
// stops the container and cascades DB rows (domains, env, deployments); Caddy
// routes are reaped by the proxy reconciler on its next tick.
func (h *TaskHandler) cleanupStalePreviews(ctx context.Context) {
	if h.Config == nil || h.Config.PreviewIdleDays <= 0 || h.AppService == nil {
		return
	}
	cutoff := time.Now().Add(-time.Duration(h.Config.PreviewIdleDays) * 24 * time.Hour)
	stale, err := h.Queries.ListStalePreviews(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		slog.Warn("preview GC: failed to list stale previews", "error", err)
		return
	}
	if len(stale) == 0 {
		return
	}

	removed := 0
	for _, app := range stale {
		project, err := h.Queries.GetProject(ctx, app.ProjectID)
		if err != nil {
			slog.Warn("preview GC: failed to fetch project",
				"application_id", formatUUID(app.ID), "error", err,
			)
			continue
		}
		if err := h.AppService.Delete(ctx, app.ID, project.Slug, app.Slug); err != nil {
			slog.Warn("preview GC: failed to delete preview",
				"application", app.Name, "error", err,
			)
			continue
		}
		slog.Info("preview GC: removed idle preview",
			"application", app.Name,
			"branch", app.Branch.String,
			"last_activity_at", app.LastActivityAt.Time,
		)
		removed++
	}
	if removed > 0 {
		slog.Info("preview GC: reaped idle previews", "count", removed, "cutoff_days", h.Config.PreviewIdleDays)
	}
}

// cleanupOrphanContainers removes managed containers whose name does not match
// any known application. Only containers older than 1 hour are considered to
// avoid racing with in-progress deployments.
func (h *TaskHandler) cleanupOrphanContainers(ctx context.Context) {
	const orphanAge = time.Hour

	containers, err := h.Runtime.ListContainers(ctx)
	if err != nil {
		slog.Warn("orphan cleanup: failed to list containers", "error", err)
		return
	}

	// Build set of all valid container names from the database.
	allApps, err := h.Queries.ListAllApplications(ctx)
	if err != nil {
		slog.Warn("orphan cleanup: failed to list applications", "error", err)
		return
	}

	known := make(map[string]bool, len(allApps))
	for _, app := range allApps {
		appIDStr := formatUUID(app.ID)
		if appIDStr == "" {
			continue
		}
		project, err := h.Queries.GetProject(ctx, app.ProjectID)
		if err != nil {
			continue
		}
		known[naming.ContainerName(project.Slug, app.Slug, appIDStr)] = true
		// Also mark old naming formats so we don't delete legacy containers.
		known[naming.IntermediateContainerName(project.Slug, appIDStr)] = true
		known[naming.OldContainerName(appIDStr)] = true
	}

	removed := 0
	for _, ctr := range containers {
		if known[ctr.Name] {
			continue
		}
		if time.Since(ctr.CreatedAt) < orphanAge {
			slog.Debug("orphan cleanup: skipping recent container", "container", ctr.Name, "age", time.Since(ctr.CreatedAt).Round(time.Second))
			continue
		}
		slog.Info("orphan cleanup: removing container", "container", ctr.Name, "created_at", ctr.CreatedAt)
		if err := h.Runtime.StopContainer(ctx, ctr.Name); err != nil {
			slog.Debug("orphan cleanup: stop failed (may already be stopped)", "container", ctr.Name, "error", err)
		}
		if err := h.Runtime.RemoveContainer(ctx, ctr.Name); err != nil {
			slog.Warn("orphan cleanup: failed to remove container", "container", ctr.Name, "error", err)
			continue
		}
		removed++
	}

	if removed > 0 {
		slog.Info("orphan cleanup: removed containers", "count", removed)
	}
}

func formatUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
