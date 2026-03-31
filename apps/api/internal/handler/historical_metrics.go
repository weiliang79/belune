package handler

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type hostMetricPoint struct {
	CPUPercent  *float64 `json:"cpu_percent"`
	MemoryUsed  *int64   `json:"memory_used"`
	MemoryTotal *int64   `json:"memory_total"`
	DiskUsed    *int64   `json:"disk_used"`
	DiskTotal   *int64   `json:"disk_total"`
	RecordedAt  string   `json:"recorded_at"`
}

type appMetricPoint struct {
	CPUPercent     *float64 `json:"cpu_percent"`
	MemoryUsage    *int64   `json:"memory_usage"`
	MemoryLimit    *int64   `json:"memory_limit"`
	NetworkRxBytes *int64   `json:"network_rx_bytes"`
	NetworkTxBytes *int64   `json:"network_tx_bytes"`
	RecordedAt     string   `json:"recorded_at"`
}

// parseRangeDuration returns the since time for a given range query param.
func parseRangeDuration(rangeParam string) time.Time {
	now := time.Now()
	switch rangeParam {
	case "1h":
		return now.Add(-1 * time.Hour)
	case "6h":
		return now.Add(-6 * time.Hour)
	case "24h":
		return now.Add(-24 * time.Hour)
	case "7d":
		return now.Add(-7 * 24 * time.Hour)
	case "14d":
		return now.Add(-14 * 24 * time.Hour)
	default:
		return now.Add(-1 * time.Hour)
	}
}

// GetHostHistoricalMetrics returns historical host metrics stored at 1-minute intervals.
// GET /api/metrics/host?range=1h|6h|24h|7d|14d
func (h *Handler) GetHostHistoricalMetrics(w http.ResponseWriter, r *http.Request) {
	rangeParam := r.URL.Query().Get("range")
	since := parseRangeDuration(rangeParam)

	rows, err := h.queries.GetHostMetrics(r.Context(), pgtype.Timestamptz{Time: since, Valid: true})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch host metrics")
		return
	}

	result := make([]hostMetricPoint, 0, len(rows))
	for _, row := range rows {
		result = append(result, hostMetricPoint{
			CPUPercent:  &row.CpuPercent,
			MemoryUsed:  &row.MemoryUsed,
			MemoryTotal: &row.MemoryTotal,
			DiskUsed:    &row.DiskUsed,
			DiskTotal:   &row.DiskTotal,
			RecordedAt:  row.RecordedAt.Time.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, result)
}
