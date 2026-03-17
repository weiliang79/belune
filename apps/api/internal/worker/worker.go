package worker

import (
	"log/slog"

	"github.com/hibiken/asynq"
)

type Worker struct {
	server *asynq.Server
}

func New(redisURL string) *Worker {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisURL},
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)

	return &Worker{server: srv}
}

func (w *Worker) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeDeploy, HandleDeployTask)
	mux.HandleFunc(TypeBuild, HandleBuildTask)
	mux.HandleFunc(TypeCleanup, HandleCleanupTask)

	slog.Info("starting worker server")
	return w.server.Start(mux)
}

func (w *Worker) Stop() {
	w.server.Stop()
}
