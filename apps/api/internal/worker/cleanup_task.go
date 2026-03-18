package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type cleanupPayload struct {
	ServiceID   string `json:"service_id,omitempty"`
	RetainCount int    `json:"retain_count,omitempty"`
}

func (h *TaskHandler) HandleCleanupTask(ctx context.Context, t *asynq.Task) error {
	var payload cleanupPayload
	if len(t.Payload()) > 0 {
		_ = json.Unmarshal(t.Payload(), &payload)
	}

	if payload.RetainCount <= 0 {
		payload.RetainCount = 3
	}

	slog.Info("handling cleanup task", "retain_count", payload.RetainCount, "service_id", payload.ServiceID)

	var services []generated.Service
	var err error

	if payload.ServiceID != "" {
		svc, err := h.Queries.GetService(ctx, parseUUID(payload.ServiceID))
		if err != nil {
			return fmt.Errorf("get service: %w", err)
		}
		services = []generated.Service{svc}
	} else {
		services, err = h.Queries.ListAllServices(ctx)
		if err != nil {
			return fmt.Errorf("list services: %w", err)
		}
	}

	totalRemoved := 0

	for _, svc := range services {
		serviceIDStr := formatUUID(svc.ID)
		if serviceIDStr == "" {
			continue
		}

		// Get project slug for image naming
		project, err := h.Queries.GetProject(ctx, svc.ProjectID)
		if err != nil {
			slog.Warn("failed to get project for service", "service_id", serviceIDStr, "error", err)
			continue
		}

		// Get deployments beyond the retain count
		oldDeployments, err := h.Queries.ListOldDeployments(ctx, generated.ListOldDeploymentsParams{
			ServiceID: svc.ID,
			Offset:    int32(payload.RetainCount),
		})
		if err != nil {
			slog.Warn("failed to list old deployments", "service_id", serviceIDStr, "error", err)
			continue
		}

		for _, dep := range oldDeployments {
			deploymentIDStr := formatUUID(dep.ID)

			// Try to remove the build image (try both old and new naming)
			if svc.Type == "git" && deploymentIDStr != "" {
				imageName := naming.ImageTag(project.Slug, svc.Slug, serviceIDStr, deploymentIDStr)
				oldImageName := fmt.Sprintf("paas-%s:%s", serviceIDStr[:8], deploymentIDStr[:8])
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

	slog.Info("cleanup completed",
		"services_processed", len(services),
		"deployments_removed", totalRemoved,
	)
	return nil
}

func formatUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
