package worker

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// HandleDownsampleMetrics1sTask aggregates 1s→1m for data older than 1h,
// then deletes the raw 1s rows. Runs every 5 minutes.
func (h *TaskHandler) HandleDownsampleMetrics1sTask(ctx context.Context, t *asynq.Task) error {
	now := time.Now()
	cutoff1h := now.Add(-1 * time.Hour)
	cutoff2h := now.Add(-2 * time.Hour) // only process a rolling 1h window to avoid re-processing

	if err := h.Queries.DownsampleMetrics1m(ctx, generated.DownsampleMetrics1mParams{
		RecordedAt:   pgtype.Timestamptz{Time: cutoff1h, Valid: true},
		RecordedAt_2: pgtype.Timestamptz{Time: cutoff2h, Valid: true},
	}); err != nil {
		slog.Error("failed to downsample 1s→1m", "error", err)
	}

	if err := h.Queries.DeleteOldMetricSnapshots(ctx, generated.DeleteOldMetricSnapshotsParams{
		Granularity: "1s",
		RecordedAt:  pgtype.Timestamptz{Time: cutoff1h, Valid: true},
	}); err != nil {
		slog.Error("failed to delete old 1s data", "error", err)
	}

	slog.Info("1s metrics downsample completed")
	return nil
}

// HandleDownsampleMetricsTask aggregates 1m→5m (>24h) and 5m→1h (>7d),
// then prunes data older than the configured retention period.
func (h *TaskHandler) HandleDownsampleMetricsTask(ctx context.Context, t *asynq.Task) error {
	now := time.Now()

	// Read retention setting (default 30 days)
	retentionDays := 30
	setting, err := h.Queries.GetSetting(ctx, "metrics_retention_days")
	if err == nil {
		if d, err := strconv.Atoi(setting.Value); err == nil && d > 0 {
			retentionDays = d
		}
	}

	slog.Info("starting metrics downsample", "retention_days", retentionDays)

	// Downsample 1m → 5m for data older than 24h but within 7 days
	cutoff24h := now.Add(-24 * time.Hour)
	cutoff7d := now.Add(-7 * 24 * time.Hour)

	if err := h.Queries.DownsampleMetrics5m(ctx, generated.DownsampleMetrics5mParams{
		RecordedAt:   pgtype.Timestamptz{Time: cutoff24h, Valid: true},
		RecordedAt_2: pgtype.Timestamptz{Time: cutoff7d, Valid: true},
	}); err != nil {
		slog.Error("failed to downsample 1m→5m", "error", err)
	}

	// Delete 1m data older than 24h (now represented as 5m)
	if err := h.Queries.DeleteOldMetricSnapshots(ctx, generated.DeleteOldMetricSnapshotsParams{
		Granularity: "1m",
		RecordedAt:  pgtype.Timestamptz{Time: cutoff24h, Valid: true},
	}); err != nil {
		slog.Error("failed to delete old 1m data", "error", err)
	}

	// Downsample 5m → 1h for data older than 7 days but within retention
	retentionCutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)

	if err := h.Queries.DownsampleMetrics1h(ctx, generated.DownsampleMetrics1hParams{
		RecordedAt:   pgtype.Timestamptz{Time: cutoff7d, Valid: true},
		RecordedAt_2: pgtype.Timestamptz{Time: retentionCutoff, Valid: true},
	}); err != nil {
		slog.Error("failed to downsample 5m→1h", "error", err)
	}

	// Delete 5m data older than 7 days (now represented as 1h)
	if err := h.Queries.DeleteOldMetricSnapshots(ctx, generated.DeleteOldMetricSnapshotsParams{
		Granularity: "5m",
		RecordedAt:  pgtype.Timestamptz{Time: cutoff7d, Valid: true},
	}); err != nil {
		slog.Error("failed to delete old 5m data", "error", err)
	}

	// Delete 1h data older than retention
	if err := h.Queries.DeleteOldMetricSnapshots(ctx, generated.DeleteOldMetricSnapshotsParams{
		Granularity: "1h",
		RecordedAt:  pgtype.Timestamptz{Time: retentionCutoff, Valid: true},
	}); err != nil {
		slog.Error("failed to delete old 1h data", "error", err)
	}

	slog.Info("metrics downsample completed")
	return nil
}
