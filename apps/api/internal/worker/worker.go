package worker

import (
	"encoding/json"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// TaskHandler holds dependencies needed by async task handlers.
type TaskHandler struct {
	Runtime       runtime.ContainerRuntime
	Proxy         proxy.ProxyManager
	Queries       *generated.Queries
	Chain         *build.Chain
	EncryptionKey string
}

type Worker struct {
	server   *asynq.Server
	handler  *TaskHandler
	redisURL string
}

func New(redisURL string, handler *TaskHandler) *Worker {
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

	return &Worker{server: srv, handler: handler, redisURL: redisURL}
}

func (w *Worker) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeDeploy, w.handler.HandleDeployTask)
	mux.HandleFunc(TypeBuild, w.handler.HandleBuildTask)
	mux.HandleFunc(TypeCleanup, w.handler.HandleCleanupTask)
	mux.HandleFunc(TypeProvisionDB, w.handler.HandleProvisionDBTask)

	slog.Info("starting worker server")
	return w.server.Start(mux)
}

func (w *Worker) StartScheduler() (*asynq.Scheduler, error) {
	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: w.redisURL},
		nil,
	)

	payload, _ := json.Marshal(cleanupPayload{RetainCount: 3})
	task := asynq.NewTask(TypeCleanup, payload)

	if _, err := scheduler.Register("@every 24h", task, asynq.Queue("low")); err != nil {
		return nil, err
	}

	slog.Info("starting cleanup scheduler (every 24h)")
	if err := scheduler.Start(); err != nil {
		return nil, err
	}

	return scheduler, nil
}

func (w *Worker) Stop() {
	w.server.Stop()
}
