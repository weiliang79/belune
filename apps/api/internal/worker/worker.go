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
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/quota"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/service/backup"
	"github.com/ungweiliang/selfhost-paas/internal/service/email"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// auditLogger is the narrow interface the worker needs to emit audit entries.
type auditLogger interface {
	Log(userID, ip, action, resourceType, resourceID string, details map[string]any)
}

// notifier is the narrow interface the worker needs to emit user-facing
// notifications. Sibling of auditLogger — emitted in parallel, never chained.
type notifier interface {
	Notify(userID, notifType, title, body, link string)
}

// TaskEnqueuer is the narrow interface used by worker tasks that need to
// enqueue follow-on work (e.g. sending alert emails from async tasks).
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// TaskHandler holds dependencies needed by async task handlers.
type TaskHandler struct {
	Runtime        runtime.ContainerRuntime
	Proxy          proxy.ProxyManager
	DB             *pgxpool.Pool
	Queries        *generated.Queries
	Chain          *build.Chain
	Keyring        *crypto.Keyring
	RedisClient    *redis.Client
	Config         *config.Config
	MetricsService *service.MetricsService
	AppService     *service.ApplicationService
	QuotaService   *quota.Service
	EmailService   *email.Service
	BackupService  *backup.Service
	AuditLog       auditLogger
	Notifier       notifier
	Enqueuer       TaskEnqueuer
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
	mux.HandleFunc(TypeReconfigureDB, w.handler.HandleReconfigureDBTask)
	mux.HandleFunc(TypeBackupDB, w.handler.HandleBackupDBTask)
	mux.HandleFunc(TypeRestoreDB, w.handler.HandleRestoreDBTask)
	mux.HandleFunc(TypeUpgradeDB, w.handler.HandleUpgradeDBTask)
	mux.HandleFunc(TypeRetentionCleanup, func(ctx context.Context, t *asynq.Task) error {
		w.handler.HandleRetentionCleanup(ctx)
		return nil
	})
	mux.HandleFunc(TypeEmailSend, w.handler.HandleEmailSendTask)
	mux.HandleFunc(TypeAuthTokenCleanup, w.handler.HandleAuthTokenCleanup)
	mux.HandleFunc(TypeQuotaThresholdSweep, w.handler.HandleQuotaThresholdSweep)
	mux.HandleFunc(TypeBackupNow, w.handler.HandleBackupNowTask)
	mux.HandleFunc(TypeBackupRotate, w.handler.HandleBackupRotateTask)

	// Reconcile any database left mid-upgrade by a previous crash/restart before
	// accepting new work — upgrades never auto-retry, so these would otherwise sit
	// permanently in the "upgrading" state.
	w.handler.ReconcileInterruptedUpgrades(context.Background())

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

	// Hourly auth-token cleanup: expired password-reset and invitation tokens.
	authCleanupTask := asynq.NewTask(TypeAuthTokenCleanup, nil)
	if _, err := scheduler.Register("@every 1h", authCleanupTask, asynq.Queue("low")); err != nil {
		return nil, err
	}

	// Quota threshold sweep every 6h — walks all projects and fires alerts.
	quotaSweepTask := asynq.NewTask(TypeQuotaThresholdSweep, nil)
	if _, err := scheduler.Register("@every 6h", quotaSweepTask, asynq.Queue("low")); err != nil {
		return nil, err
	}

	// Backup rotation daily — deletes remote objects beyond the retention policy.
	backupRotateTask := asynq.NewTask(TypeBackupRotate, nil)
	if _, err := scheduler.Register("@every 24h", backupRotateTask, asynq.Queue("low")); err != nil {
		return nil, err
	}

	slog.Info("starting scheduler (cleanup: 24h, retention: 24h, auth-token-cleanup: 1h, quota-sweep: 6h, backup-rotate: 24h)")
	if err := scheduler.Start(); err != nil {
		return nil, err
	}

	return scheduler, nil
}

func (w *Worker) Stop() {
	w.server.Stop()
}

// Shutdown blocks until all in-flight tasks have completed and the asynq
// server has fully stopped. Always call after Stop.
func (w *Worker) Shutdown() {
	w.server.Shutdown()
}
