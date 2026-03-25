package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/sse"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// StreamHostMetrics streams live host metric points via SSE.
// Bootstraps the last 60 seconds of 1s data, then pushes new points as they arrive.
// GET /api/metrics/host/stream
func (h *Handler) StreamHostMetrics(w http.ResponseWriter, r *http.Request) {
	writer, err := sse.NewWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ctx := r.Context()

	// Send a ping comment immediately so the proxy flushes the response headers
	// and the browser fires onopen, even when there is no data yet.
	if err := writer.SendComment("ping"); err != nil {
		return
	}

	since := time.Now().Add(-60 * time.Second)
	lastSeen := since

	// Bootstrap: send the last 60 seconds of existing data
	rows, err := h.queries.GetHostMetrics(ctx, generated.GetHostMetricsParams{
		Granularity: "1s",
		RecordedAt:  pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err == nil {
		for _, row := range rows {
			point := hostMetricPoint{
				CPUPercent:  ptrFloat8(row.HostCpuPercent),
				MemoryUsed:  ptrInt8(row.HostMemoryUsed),
				MemoryTotal: ptrInt8(row.HostMemoryTotal),
				DiskUsed:    ptrInt8(row.HostDiskUsed),
				DiskTotal:   ptrInt8(row.HostDiskTotal),
				RecordedAt:  row.RecordedAt.Time.Format(time.RFC3339),
			}
			data, _ := json.Marshal(point)
			if err := writer.SendData(string(data)); err != nil {
				return
			}
			if row.RecordedAt.Time.After(lastSeen) {
				lastSeen = row.RecordedAt.Time
			}
		}
	}

	ticker := time.NewTicker(time.Second)
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
			rows, err := h.queries.GetHostMetrics(ctx, generated.GetHostMetricsParams{
				Granularity: "1s",
				RecordedAt:  pgtype.Timestamptz{Time: lastSeen, Valid: true},
			})
			if err != nil {
				continue
			}
			for _, row := range rows {
				if !row.RecordedAt.Time.After(lastSeen) {
					continue
				}
				point := hostMetricPoint{
					CPUPercent:  ptrFloat8(row.HostCpuPercent),
					MemoryUsed:  ptrInt8(row.HostMemoryUsed),
					MemoryTotal: ptrInt8(row.HostMemoryTotal),
					DiskUsed:    ptrInt8(row.HostDiskUsed),
					DiskTotal:   ptrInt8(row.HostDiskTotal),
					RecordedAt:  row.RecordedAt.Time.Format(time.RFC3339),
				}
				data, _ := json.Marshal(point)
				if err := writer.SendData(string(data)); err != nil {
					return
				}
				lastSeen = row.RecordedAt.Time
			}
		}
	}
}

// StreamApplicationMetrics streams live application metric points via SSE.
// Bootstraps the last 60 seconds of 1s data, then pushes new points as they arrive.
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

	writer, err := sse.NewWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ctx := r.Context()

	// Send a ping comment immediately so the proxy flushes the response headers
	// and the browser fires onopen, even when there is no data yet.
	if err := writer.SendComment("ping"); err != nil {
		return
	}

	since := time.Now().Add(-60 * time.Second)
	lastSeen := since

	// Bootstrap: send the last 60 seconds of existing data
	rows, err := h.queries.GetApplicationMetrics(ctx, generated.GetApplicationMetricsParams{
		ApplicationID: appUUID,
		Granularity:   "1s",
		RecordedAt:    pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err == nil {
		for _, row := range rows {
			point := appMetricPoint{
				CPUPercent:     ptrFloat8(row.CpuPercent),
				MemoryUsage:    ptrInt8(row.MemoryUsage),
				MemoryLimit:    ptrInt8(row.MemoryLimit),
				NetworkRxBytes: ptrInt8(row.NetworkRxBytes),
				NetworkTxBytes: ptrInt8(row.NetworkTxBytes),
				RecordedAt:     row.RecordedAt.Time.Format(time.RFC3339),
			}
			data, _ := json.Marshal(point)
			if err := writer.SendData(string(data)); err != nil {
				return
			}
			if row.RecordedAt.Time.After(lastSeen) {
				lastSeen = row.RecordedAt.Time
			}
		}
	}

	ticker := time.NewTicker(time.Second)
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
			rows, err := h.queries.GetApplicationMetrics(ctx, generated.GetApplicationMetricsParams{
				ApplicationID: appUUID,
				Granularity:   "1s",
				RecordedAt:    pgtype.Timestamptz{Time: lastSeen, Valid: true},
			})
			if err != nil {
				continue
			}
			for _, row := range rows {
				if !row.RecordedAt.Time.After(lastSeen) {
					continue
				}
				point := appMetricPoint{
					CPUPercent:     ptrFloat8(row.CpuPercent),
					MemoryUsage:    ptrInt8(row.MemoryUsage),
					MemoryLimit:    ptrInt8(row.MemoryLimit),
					NetworkRxBytes: ptrInt8(row.NetworkRxBytes),
					NetworkTxBytes: ptrInt8(row.NetworkTxBytes),
					RecordedAt:     row.RecordedAt.Time.Format(time.RFC3339),
				}
				data, _ := json.Marshal(point)
				if err := writer.SendData(string(data)); err != nil {
					return
				}
				lastSeen = row.RecordedAt.Time
			}
		}
	}
}
