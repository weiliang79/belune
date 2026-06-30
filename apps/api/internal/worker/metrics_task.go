package worker

import (
	"context"
)

// HandleRetentionCleanup delegates to MetricsService.RetentionCleanup.
func (h *TaskHandler) HandleRetentionCleanup(ctx context.Context) {
	h.MetricsService.RetentionCleanup(ctx)
}

// HandleHostMetricsCleanup delegates to MetricsService.HostMetricsCleanup.
func (h *TaskHandler) HandleHostMetricsCleanup(ctx context.Context) {
	h.MetricsService.HostMetricsCleanup(ctx)
}

// StartMetricsTicker delegates to MetricsService.StartTicker.
// Blocks until ctx is cancelled.
func (w *Worker) StartMetricsTicker(ctx context.Context) {
	w.handler.MetricsService.StartTicker(ctx)
}
