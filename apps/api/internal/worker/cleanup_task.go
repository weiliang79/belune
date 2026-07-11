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

	"github.com/weiliang79/belune/internal/naming"
	"github.com/weiliang79/belune/internal/store/generated"
)

// settingDailyCleanup gates the periodic (scheduled) cleanup run. Absent or any
// value other than "false" means enabled — preserving the historical always-on
// behaviour.
const settingDailyCleanup = "daily_cleanup_enabled"

type cleanupPayload struct {
	ApplicationID string `json:"application_id,omitempty"`
	RetainCount   int    `json:"retain_count,omitempty"`
	// Scheduled marks the periodic run enqueued by the scheduler. Only scheduled
	// runs are gated by the daily_cleanup_enabled setting; manual and per-app
	// cleanups always run.
	Scheduled bool `json:"scheduled,omitempty"`
	// Actions selects which cleanup steps to run. Empty = full cleanup (all
	// steps), which the periodic run and the "Run full cleanup" button use.
	// Values: "deployments" | "images" | "volumes" | "containers" | "build_cache".
	Actions []string `json:"actions,omitempty"`
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

	// Scheduled runs honour the daily-cleanup toggle; manual/per-app runs ignore it.
	if payload.Scheduled && !h.dailyCleanupEnabled(ctx) {
		slog.Info("daily cleanup disabled by setting; skipping scheduled run")
		return nil
	}

	full := len(payload.Actions) == 0
	wants := func(action string) bool {
		if full {
			return true
		}
		for _, a := range payload.Actions {
			if a == action {
				return true
			}
		}
		return false
	}

	slog.Info("handling cleanup task", "retain_count", payload.RetainCount,
		"application_id", payload.ApplicationID, "actions", payload.Actions)

	totalRemoved := 0
	appsProcessed := 0

	if wants("deployments") {
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
	}

	if wants("images") {
		if err := h.Runtime.PruneImages(ctx); err != nil {
			slog.Warn("failed to prune images", "error", err)
		}
	}
	if wants("volumes") {
		if err := h.Runtime.PruneVolumes(ctx); err != nil {
			slog.Warn("failed to prune volumes", "error", err)
		}
	}
	if wants("build_cache") {
		if err := h.Runtime.PruneBuildCache(ctx); err != nil {
			slog.Warn("failed to prune build cache", "error", err)
		}
	}
	if wants("containers") {
		// Remove orphan containers: managed containers with no matching application in DB.
		h.cleanupOrphanContainers(ctx)
	}

	// Reap idle preview apps only in a full bulk cleanup — never for a targeted
	// single-app or single-action run.
	if full && payload.ApplicationID == "" {
		h.cleanupStalePreviews(ctx)
	}

	slog.Info("cleanup completed",
		"applications_processed", appsProcessed,
		"deployments_removed", totalRemoved,
	)
	return nil
}

// dailyCleanupEnabled reports whether the periodic cleanup should run. Absent or
// unreadable setting defaults to enabled (preserving prior always-on behaviour).
func (h *TaskHandler) dailyCleanupEnabled(ctx context.Context) bool {
	s, err := h.Queries.GetSetting(ctx, settingDailyCleanup)
	if err != nil {
		return true
	}
	return s.Value != "false"
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
			oldImageName := fmt.Sprintf("belune-%s:%s", applicationIDStr[:8], deploymentIDStr[:8])
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
