package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// HandleCollectMetricsTask collects host stats (gopsutil) and per-container stats (Docker API),
// then inserts 1m granularity snapshots into the database.
func (h *TaskHandler) HandleCollectMetricsTask(ctx context.Context, t *asynq.Task) error {
	now := time.Now()
	recordedAt := pgtype.Timestamptz{Time: now, Valid: true}

	// Collect host stats
	cpuPercents, err := cpu.PercentWithContext(ctx, 0, false)
	var hostCPU float64
	if err == nil && len(cpuPercents) > 0 {
		hostCPU = cpuPercents[0]
	} else if err != nil {
		slog.Warn("failed to collect host CPU", "error", err)
	}

	memInfo, err := mem.VirtualMemoryWithContext(ctx)
	var hostMemUsed, hostMemTotal int64
	if err == nil {
		hostMemUsed = int64(memInfo.Used)
		hostMemTotal = int64(memInfo.Total)
	} else {
		slog.Warn("failed to collect host memory", "error", err)
	}

	diskInfo, err := disk.UsageWithContext(ctx, "/")
	var hostDiskUsed, hostDiskTotal int64
	if err == nil {
		hostDiskUsed = int64(diskInfo.Used)
		hostDiskTotal = int64(diskInfo.Total)
	} else {
		slog.Warn("failed to collect host disk", "error", err)
	}

	// Insert host-level snapshot (no application_id)
	if err := h.Queries.InsertMetricSnapshot(ctx, generated.InsertMetricSnapshotParams{
		Granularity:     "1m",
		HostCpuPercent:  pgtype.Float8{Float64: hostCPU, Valid: true},
		HostMemoryUsed:  pgtype.Int8{Int64: hostMemUsed, Valid: true},
		HostMemoryTotal: pgtype.Int8{Int64: hostMemTotal, Valid: true},
		HostDiskUsed:    pgtype.Int8{Int64: hostDiskUsed, Valid: true},
		HostDiskTotal:   pgtype.Int8{Int64: hostDiskTotal, Valid: true},
		RecordedAt:      recordedAt,
	}); err != nil {
		slog.Error("failed to insert host metric snapshot", "error", err)
	}

	// Collect per-container stats
	containers, err := h.Runtime.ListContainers(ctx)
	if err != nil {
		slog.Error("failed to list containers for metrics", "error", err)
		return nil
	}

	for _, ctr := range containers {
		if ctr.Status != "running" {
			continue
		}

		appIDStr, ok := ctr.Labels["application-id"]
		if !ok {
			continue
		}

		var appUUID pgtype.UUID
		if err := appUUID.Scan(appIDStr); err != nil {
			continue
		}

		stats, err := h.Runtime.ContainerStats(ctx, ctr.ID)
		if err != nil {
			slog.Warn("failed to collect container stats", "container", ctr.Name, "error", err)
			continue
		}

		if err := h.Queries.InsertMetricSnapshot(ctx, generated.InsertMetricSnapshotParams{
			ApplicationID:  appUUID,
			Granularity:    "1m",
			CpuPercent:     pgtype.Float8{Float64: stats.CPUPercent, Valid: true},
			MemoryUsage:    pgtype.Int8{Int64: stats.MemoryUsage, Valid: true},
			MemoryLimit:    pgtype.Int8{Int64: stats.MemoryLimit, Valid: true},
			NetworkRxBytes: pgtype.Int8{Int64: stats.NetworkRxBytes, Valid: true},
			NetworkTxBytes: pgtype.Int8{Int64: stats.NetworkTxBytes, Valid: true},
			RecordedAt:     recordedAt,
		}); err != nil {
			slog.Error("failed to insert container metric snapshot", "container", ctr.Name, "error", err)
		}
	}

	slog.Info("metrics collection completed", "containers_checked", len(containers))
	return nil
}
