package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

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
	RedisClient   *redis.Client
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
	mux.HandleFunc(TypeDownsampleMetrics1s, w.handler.HandleDownsampleMetrics1sTask)
	mux.HandleFunc(TypeDownsampleMetrics, w.handler.HandleDownsampleMetricsTask)

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

	// Schedule 1s→1m downsample every 5 minutes
	downsample1sTask := asynq.NewTask(TypeDownsampleMetrics1s, nil)
	if _, err := scheduler.Register("@every 5m", downsample1sTask, asynq.Queue("low")); err != nil {
		return nil, err
	}

	// Schedule 1m→5m→1h downsample every 1 hour
	downsampleTask := asynq.NewTask(TypeDownsampleMetrics, nil)
	if _, err := scheduler.Register("@every 1h", downsampleTask, asynq.Queue("low")); err != nil {
		return nil, err
	}

	slog.Info("starting scheduler (cleanup: 24h, downsample-1s: 5m, downsample: 1h)")
	if err := scheduler.Start(); err != nil {
		return nil, err
	}

	return scheduler, nil
}

func (w *Worker) Stop() {
	w.server.Stop()
}

// StartMetricsTicker runs metrics collection on a precise 1-second ticker,
// bypassing the Redis-backed scheduler to avoid jitter and double-firing.
func (w *Worker) StartMetricsTicker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.handler.HandleCollectMetricsTask(ctx, nil); err != nil {
				slog.Error("metrics ticker: collection failed", "error", err)
			}
		}
	}
}
