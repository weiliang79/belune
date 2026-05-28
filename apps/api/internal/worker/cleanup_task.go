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

	totalRemoved := 0
	appsProcessed := 0

	if payload.ApplicationID != "" {
		// Single-app cleanup: one JOIN query instead of GetApplication + GetProject.
		appID, err := parseUUID(payload.ApplicationID)
		if err != nil {
			return errors.Join(fmt.Errorf("invalid application_id (permanent): %w", err), asynq.SkipRetry)
		}
		row, err := h.Queries.GetApplicationWithProjectSlug(ctx, appID)
		if err != nil {
			return fmt.Errorf("get application: %w", err)
		}
		h.cleanupAppDeployments(ctx, row.ID, row.Type, row.Slug, row.ProjectSlug, payload.RetainCount, &totalRemoved)
		appsProcessed = 1
	} else {
		// Bulk cleanup: single JOIN query — no per-row GetProject call.
		rows, err := h.Queries.ListAllApplicationsWithProjectSlug(ctx)
		if err != nil {
			return fmt.Errorf("list applications: %w", err)
		}
		for _, row := range rows {
			h.cleanupAppDeployments(ctx, row.ID, row.Type, row.Slug, row.ProjectSlug, payload.RetainCount, &totalRemoved)
		}
		appsProcessed = len(rows)
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
		"applications_processed", appsProcessed,
		"deployments_removed", totalRemoved,
	)
	return nil
}

// cleanupAppDeployments removes images and DB records for deployments of a
// single application beyond the retain count. Errors are logged as warnings
// and never returned — cleanup is best-effort and must not abort sibling apps.
func (h *TaskHandler) cleanupAppDeployments(ctx context.Context, appID pgtype.UUID, appType, appSlug, projectSlug string, retainCount int, totalRemoved *int) {
	applicationIDStr := formatUUID(appID)
	if applicationIDStr == "" {
		return
	}

	oldDeployments, err := h.Queries.ListOldDeployments(ctx, generated.ListOldDeploymentsParams{
		ApplicationID: appID,
		Offset:        int32(retainCount),
	})
	if err != nil {
		slog.Warn("cleanup: failed to list old deployments", "application_id", applicationIDStr, "error", err)
		return
	}

	for _, dep := range oldDeployments {
		deploymentIDStr := formatUUID(dep.ID)

		if appType == "git" && deploymentIDStr != "" {
			imageName := naming.ImageTag(projectSlug, appSlug, applicationIDStr, deploymentIDStr)
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

		if err := h.Queries.DeleteDeployment(ctx, dep.ID); err != nil {
			slog.Warn("failed to delete deployment", "deployment_id", deploymentIDStr, "error", err)
		} else {
			*totalRemoved++
		}
	}
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
	stale, err := h.Queries.ListStalePreviewsWithProjectSlug(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		slog.Warn("preview GC: failed to list stale previews", "error", err)
		return
	}
	if len(stale) == 0 {
		return
	}

	removed := 0
	for _, row := range stale {
		if err := h.AppService.Delete(ctx, row.ID, row.ProjectSlug, row.Slug); err != nil {
			slog.Warn("preview GC: failed to delete preview",
				"application", row.Name, "error", err,
			)
			continue
		}
		slog.Info("preview GC: removed idle preview",
			"application", row.Name,
			"branch", row.Branch.String,
			"last_activity_at", row.LastActivityAt.Time,
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

	// Build set of all valid container names from the database (single JOIN query).
	allApps, err := h.Queries.ListAllApplicationsWithProjectSlug(ctx)
	if err != nil {
		slog.Warn("orphan cleanup: failed to list applications", "error", err)
		return
	}

	known := make(map[string]bool, len(allApps))
	for _, row := range allApps {
		appIDStr := formatUUID(row.ID)
		if appIDStr == "" {
			continue
		}
		known[naming.ContainerName(row.ProjectSlug, row.Slug, appIDStr)] = true
		// Also mark old naming formats so we don't delete legacy containers.
		known[naming.IntermediateContainerName(row.ProjectSlug, appIDStr)] = true
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
