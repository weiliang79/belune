package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

// AsynqQueuePoller runs a background ticker that refreshes
// paas_asynq_queue_size from asynq's Redis-backed inspector.
//
// Ownership: the caller owns the inspector and must Close it. Run blocks
// until ctx is cancelled.
func AsynqQueuePoller(ctx context.Context, inspector *asynq.Inspector, interval time.Duration) {
	if inspector == nil {
		slog.Warn("metrics: asynq queue poller disabled (nil inspector)")
		return
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}

	t := time.NewTicker(interval)
	defer t.Stop()

	// Refresh immediately so /metrics is populated on the first scrape.
	refreshAsynqQueues(ctx, inspector)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			refreshAsynqQueues(ctx, inspector)
		}
	}
}

func refreshAsynqQueues(ctx context.Context, inspector *asynq.Inspector) {
	queues, err := inspector.Queues()
	if err != nil {
		slog.Debug("metrics: asynq inspector Queues failed", "error", err)
		return
	}
	for _, q := range queues {
		info, err := inspector.GetQueueInfo(q)
		if err != nil {
			slog.Debug("metrics: asynq GetQueueInfo failed", "queue", q, "error", err)
			continue
		}
		SetAsynqQueueSize(q, "pending", info.Pending)
		SetAsynqQueueSize(q, "active", info.Active)
		SetAsynqQueueSize(q, "scheduled", info.Scheduled)
		SetAsynqQueueSize(q, "retry", info.Retry)
		SetAsynqQueueSize(q, "archived", info.Archived)
		SetAsynqQueueSize(q, "completed", info.Completed)
	}
	_ = ctx // reserved: asynq.Inspector does not currently accept a ctx
}
