package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/naming"
	"github.com/weiliang79/belune/internal/pkg/sse"
)

// StreamHostMetrics streams live host metric points via SSE.
// Subscribes to Redis pub/sub channel published by the metrics ticker.
// GET /api/metrics/host/stream
func (h *Handler) StreamHostMetrics(w http.ResponseWriter, r *http.Request) {
	writer, err := sse.NewWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ctx := r.Context()

	if err := writer.SendComment("ping"); err != nil {
		return
	}

	// Subscribe to the Redis pub/sub channel for live host metrics
	pubsub := h.rdb.Subscribe(ctx, "host:metrics:live")
	defer pubsub.Close()
	ch := pubsub.Channel()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if err := writer.SendComment("ping"); err != nil {
				return
			}
		case msg := <-ch:
			if err := writer.SendData(msg.Payload); err != nil {
				return
			}
		}
	}
}

// StreamApplicationMetrics streams live container metrics for a single application via SSE.
// Queries Docker stats API on-demand every 2 seconds — no database storage.
// GET /api/projects/{projectId}/applications/{applicationId}/metrics/stream
func (h *Handler) StreamApplicationMetrics(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var appUUID pgtype.UUID
	if err := appUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	if !h.canAccessApplication(r, appUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Look up container name from application record
	appRow, err := h.queries.GetApplicationWithProjectSlug(r.Context(), appUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	containerName := naming.ContainerName(appRow.ProjectSlug, appRow.Slug, applicationID)

	writer, sseErr := sse.NewWriter(w)
	if sseErr != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ctx := r.Context()

	if err := writer.SendComment("ping"); err != nil {
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if err := writer.SendComment("ping"); err != nil {
				return
			}
		case <-ticker.C:
			stats, err := h.runtime.ContainerStats(ctx, containerName)
			if err != nil {
				slog.Debug("failed to collect container stats for stream", "container", containerName, "error", err)
				continue
			}

			point := appMetricPoint{
				CPUPercent:     &stats.CPUPercent,
				MemoryUsage:    &stats.MemoryUsage,
				MemoryLimit:    &stats.MemoryLimit,
				NetworkRxBytes: &stats.NetworkRxBytes,
				NetworkTxBytes: &stats.NetworkTxBytes,
				RecordedAt:     time.Now().Format(time.RFC3339),
			}
			data, _ := json.Marshal(point)
			if err := writer.SendData(string(data)); err != nil {
				return
			}
		}
	}
}
