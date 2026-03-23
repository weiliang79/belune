package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type hostMetricPoint struct {
	CPUPercent  *float64 `json:"cpu_percent"`
	MemoryUsed *int64   `json:"memory_used"`
	MemoryTotal *int64  `json:"memory_total"`
	DiskUsed   *int64   `json:"disk_used"`
	DiskTotal  *int64   `json:"disk_total"`
	RecordedAt string   `json:"recorded_at"`
}

type appMetricPoint struct {
	CPUPercent     *float64 `json:"cpu_percent"`
	MemoryUsage    *int64   `json:"memory_usage"`
	MemoryLimit    *int64   `json:"memory_limit"`
	NetworkRxBytes *int64   `json:"network_rx_bytes"`
	NetworkTxBytes *int64   `json:"network_tx_bytes"`
	RecordedAt     string   `json:"recorded_at"`
}

// parseRange returns the granularity and since time for a given range query param.
func parseRange(rangeParam string) (granularity string, since time.Time) {
	now := time.Now()
	switch rangeParam {
	case "1h":
		return "1m", now.Add(-1 * time.Hour)
	case "24h":
		return "1m", now.Add(-24 * time.Hour)
	case "7d":
		return "5m", now.Add(-7 * 24 * time.Hour)
	case "30d":
		return "1h", now.Add(-30 * 24 * time.Hour)
	default:
		return "1m", now.Add(-1 * time.Hour)
	}
}

func ptrFloat8(f pgtype.Float8) *float64 {
	if !f.Valid {
		return nil
	}
	return &f.Float64
}

func ptrInt8(i pgtype.Int8) *int64 {
	if !i.Valid {
		return nil
	}
	return &i.Int64
}

// GetHostHistoricalMetrics returns historical host metrics.
// GET /api/metrics/host?range=1h|24h|7d|30d
func (h *Handler) GetHostHistoricalMetrics(w http.ResponseWriter, r *http.Request) {
	rangeParam := r.URL.Query().Get("range")
	granularity, since := parseRange(rangeParam)

	rows, err := h.queries.GetHostMetrics(r.Context(), generated.GetHostMetricsParams{
		Granularity: granularity,
		RecordedAt:  pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch host metrics")
		return
	}

	result := make([]hostMetricPoint, 0, len(rows))
	for _, row := range rows {
		result = append(result, hostMetricPoint{
			CPUPercent:  ptrFloat8(row.HostCpuPercent),
			MemoryUsed: ptrInt8(row.HostMemoryUsed),
			MemoryTotal: ptrInt8(row.HostMemoryTotal),
			DiskUsed:   ptrInt8(row.HostDiskUsed),
			DiskTotal:  ptrInt8(row.HostDiskTotal),
			RecordedAt: row.RecordedAt.Time.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// GetApplicationHistoricalMetrics returns historical metrics for a specific application.
// GET /api/projects/{projectId}/applications/{applicationId}/metrics?range=1h|24h|7d|30d
func (h *Handler) GetApplicationHistoricalMetrics(w http.ResponseWriter, r *http.Request) {
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

	rangeParam := r.URL.Query().Get("range")
	granularity, since := parseRange(rangeParam)

	rows, err := h.queries.GetApplicationMetrics(r.Context(), generated.GetApplicationMetricsParams{
		ApplicationID: appUUID,
		Granularity:   granularity,
		RecordedAt:    pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch application metrics")
		return
	}

	result := make([]appMetricPoint, 0, len(rows))
	for _, row := range rows {
		result = append(result, appMetricPoint{
			CPUPercent:     ptrFloat8(row.CpuPercent),
			MemoryUsage:    ptrInt8(row.MemoryUsage),
			MemoryLimit:    ptrInt8(row.MemoryLimit),
			NetworkRxBytes: ptrInt8(row.NetworkRxBytes),
			NetworkTxBytes: ptrInt8(row.NetworkTxBytes),
			RecordedAt:     row.RecordedAt.Time.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, result)
}
