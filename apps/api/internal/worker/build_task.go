package worker

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
)

func HandleBuildTask(ctx context.Context, t *asynq.Task) error {
	slog.Info("handling build task", "payload", string(t.Payload()))
	// TODO: Run build via builder chain (Dockerfile → CNB → Nixpacks)
	return nil
}
