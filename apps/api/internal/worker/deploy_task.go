package worker

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
)

func HandleDeployTask(ctx context.Context, t *asynq.Task) error {
	slog.Info("handling deploy task", "payload", string(t.Payload()))
	// TODO: Orchestrate deploy (clone → build → run → update proxy)
	return nil
}
