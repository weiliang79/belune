package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// hostMetricPoint is published to Redis and sent via SSE.
type hostMetricPoint struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryUsed  int64   `json:"memory_used"`
	MemoryTotal int64   `json:"memory_total"`
	DiskUsed    int64   `json:"disk_used"`
	DiskTotal   int64   `json:"disk_total"`
	RecordedAt  string  `json:"recorded_at"`
}

// collectHostStats gathers host CPU, memory, and disk usage via gopsutil.
func collectHostStats(ctx context.Context) hostMetricPoint {
	now := time.Now()
	point := hostMetricPoint{
		RecordedAt: now.Format(time.RFC3339),
	}

	cpuPercents, err := cpu.PercentWithContext(ctx, 0, false)
	if err == nil && len(cpuPercents) > 0 {
		point.CPUPercent = cpuPercents[0]
	} else if err != nil {
		slog.Warn("failed to collect host CPU", "error", err)
	}

	memInfo, err := mem.VirtualMemoryWithContext(ctx)
	if err == nil {
		point.MemoryUsed = int64(memInfo.Used)
		point.MemoryTotal = int64(memInfo.Total)
	} else {
		slog.Warn("failed to collect host memory", "error", err)
	}

	diskInfo, err := disk.UsageWithContext(ctx, "/")
	if err == nil {
		point.DiskUsed = int64(diskInfo.Used)
		point.DiskTotal = int64(diskInfo.Total)
	} else {
		slog.Warn("failed to collect host disk", "error", err)
	}

	return point
}

// persistHostMetric inserts a host metric row into the database.
func (h *TaskHandler) persistHostMetric(ctx context.Context, point hostMetricPoint) {
	recordedAt, _ := time.Parse(time.RFC3339, point.RecordedAt)
	if err := h.Queries.InsertHostMetric(ctx, generated.InsertHostMetricParams{
		CpuPercent:  point.CPUPercent,
		MemoryUsed:  point.MemoryUsed,
		MemoryTotal: point.MemoryTotal,
		DiskUsed:    point.DiskUsed,
		DiskTotal:   point.DiskTotal,
		RecordedAt:  pgtype.Timestamptz{Time: recordedAt, Valid: true},
	}); err != nil {
		slog.Error("failed to insert host metric", "error", err)
	}
}

// HandleRetentionCleanup deletes host metrics older than the configured retention period.
func (h *TaskHandler) HandleRetentionCleanup(ctx context.Context) {
	retentionDays := 14
	setting, err := h.Queries.GetSetting(ctx, "metrics_retention_days")
	if err == nil {
		var days int
		if _, scanErr := fmt.Sscanf(setting.Value, "%d", &days); scanErr == nil && days > 0 {
			retentionDays = days
		}
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	if err := h.Queries.DeleteOldHostMetrics(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true}); err != nil {
		slog.Error("failed to delete old host metrics", "error", err)
	} else {
		slog.Info("metrics retention cleanup completed", "retention_days", retentionDays)
	}
}

// StartMetricsTicker collects host metrics every 2 seconds, publishes to Redis
// for live SSE consumers, and persists to DB every 60 seconds.
func (w *Worker) StartMetricsTicker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var tickCount int
	const persistEvery = 30 // 30 ticks * 2s = 60s

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickCount++
			point := collectHostStats(ctx)

			// Publish to Redis for live SSE consumers
			data, err := json.Marshal(point)
			if err == nil {
				if pubErr := w.handler.RedisClient.Publish(ctx, "host:metrics:live", data).Err(); pubErr != nil {
					slog.Debug("failed to publish host metrics to Redis", "error", pubErr)
				}
			}

			// Persist to DB every 60 seconds
			if tickCount%persistEvery == 0 {
				w.handler.persistHostMetric(ctx, point)
			}
		}
	}
}
