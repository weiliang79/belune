package worker

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
)

func (h *TaskHandler) HandleCleanupTask(ctx context.Context, t *asynq.Task) error {
	slog.Info("handling cleanup task", "payload", string(t.Payload()))
	// TODO: Remove old containers, images, volumes
	return nil
}
