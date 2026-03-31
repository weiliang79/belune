package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// TaskHandler holds dependencies needed by async task handlers.
type TaskHandler struct {
	Runtime        runtime.ContainerRuntime
	Proxy          proxy.ProxyManager
	DB             *pgxpool.Pool
	Queries        *generated.Queries
	Chain          *build.Chain
	EncryptionKey  string
	RedisClient    *redis.Client
	Config         *config.Config
	MetricsService *service.MetricsService
}

type Worker struct {
	server   *asynq.Server
	handler  *TaskHandler
	redisOpt asynq.RedisConnOpt
}

func New(redisOpt asynq.RedisConnOpt, handler *TaskHandler) *Worker {
	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// Exponential backoff: 30s, 2m, 4.5m
				return time.Duration(n*n) * 30 * time.Second
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				retried, _ := asynq.GetRetryCount(ctx)
				maxRetry, _ := asynq.GetMaxRetry(ctx)
				slog.Error("task failed",
					"type", task.Type(),
					"retry", retried,
					"max_retry", maxRetry,
					"error", err,
				)
			}),
		},
	)

	return &Worker{server: srv, handler: handler, redisOpt: redisOpt}
}

func (w *Worker) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeDeploy, w.handler.HandleDeployTask)
	mux.HandleFunc(TypeBuild, w.handler.HandleBuildTask)
	mux.HandleFunc(TypeCleanup, w.handler.HandleCleanupTask)
	mux.HandleFunc(TypeProvisionDB, w.handler.HandleProvisionDBTask)
	mux.HandleFunc(TypeRetentionCleanup, func(ctx context.Context, t *asynq.Task) error {
		w.handler.HandleRetentionCleanup(ctx)
		return nil
	})

	slog.Info("starting worker server")
	return w.server.Start(mux)
}

func (w *Worker) StartScheduler() (*asynq.Scheduler, error) {
	scheduler := asynq.NewScheduler(
		w.redisOpt,
		nil,
	)

	payload, _ := json.Marshal(cleanupPayload{RetainCount: 3})
	task := asynq.NewTask(TypeCleanup, payload)

	if _, err := scheduler.Register("@every 24h", task, asynq.Queue("low")); err != nil {
		return nil, err
	}

	// Schedule metrics retention cleanup daily
	retentionTask := asynq.NewTask(TypeRetentionCleanup, nil)
	if _, err := scheduler.Register("@every 24h", retentionTask, asynq.Queue("low")); err != nil {
		return nil, err
	}

	slog.Info("starting scheduler (cleanup: 24h, retention: 24h)")
	if err := scheduler.Start(); err != nil {
		return nil, err
	}

	return scheduler, nil
}

func (w *Worker) Stop() {
	w.server.Stop()
}
