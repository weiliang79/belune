package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/weiling79/belune/internal/build"
	"github.com/weiling79/belune/internal/config"
	"github.com/weiling79/belune/internal/pkg/crypto"
	"github.com/weiling79/belune/internal/proxy"
	"github.com/weiling79/belune/internal/quota"
	"github.com/weiling79/belune/internal/runtime"
	"github.com/weiling79/belune/internal/service"
	"github.com/weiling79/belune/internal/service/backup"
	"github.com/weiling79/belune/internal/service/email"
	"github.com/weiling79/belune/internal/store/generated"
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
	Runtime               runtime.ContainerRuntime
	Proxy                 proxy.ProxyManager
	DB                    *pgxpool.Pool
	Queries               *generated.Queries
	Chain                 *build.Chain
	Keyring               *crypto.Keyring
	RedisClient           *redis.Client
	Config                *config.Config
	MetricsService        *service.MetricsService
	AppService            *service.ApplicationService
	GitIntegrationService *service.GitIntegrationService
	QuotaService          *quota.Service
	EmailService          *email.Service
	BackupService         *backup.Service
	BackupDestinations    *service.BackupDestinationService
	AuditLog              auditLogger
	Notifier              notifier
	Enqueuer              TaskEnqueuer
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
	mux.HandleFunc(TypeHostMetricsCleanup, func(ctx context.Context, t *asynq.Task) error {
		w.handler.HandleHostMetricsCleanup(ctx)
		return nil
	})
	mux.HandleFunc(TypeEmailSend, w.handler.HandleEmailSendTask)
	mux.HandleFunc(TypeAuthTokenCleanup, w.handler.HandleAuthTokenCleanup)
	mux.HandleFunc(TypeQuotaThresholdSweep, w.handler.HandleQuotaThresholdSweep)
	mux.HandleFunc(TypeBackupNow, w.handler.HandleBackupNowTask)
	mux.HandleFunc(TypeBackupVolume, w.handler.HandleBackupVolumeTask)
	mux.HandleFunc(TypeRestoreVolume, w.handler.HandleRestoreVolumeTask)
	mux.HandleFunc(TypeBackupRotate, w.handler.HandleBackupRotateTask)
	mux.HandleFunc(TypeBackupSchedSweep, func(ctx context.Context, t *asynq.Task) error {
		w.handler.HandleBackupScheduleSweep(ctx)
		return nil
	})
	mux.HandleFunc(TypeTLSStatusSweep, func(ctx context.Context, t *asynq.Task) error {
		w.handler.HandleTLSStatusSweep(ctx)
		return nil
	})
	mux.HandleFunc(TypeTLSProbe, func(ctx context.Context, t *asynq.Task) error {
		return w.handler.HandleTLSProbeTask(ctx, t.Payload())
	})

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

	// Scheduled=true so HandleCleanupTask honours the daily_cleanup_enabled toggle
	// (manual/per-app cleanups leave it false and always run).
	payload, _ := json.Marshal(cleanupPayload{RetainCount: 3, Scheduled: true})
	task := asynq.NewTask(TypeCleanup, payload)

	if _, err := scheduler.Register("@every 24h", task, asynq.Queue("low")); err != nil {
		return nil, err
	}

	// Schedule log retention cleanup daily
	retentionTask := asynq.NewTask(TypeRetentionCleanup, nil)
	if _, err := scheduler.Register("@every 24h", retentionTask, asynq.Queue("low")); err != nil {
		return nil, err
	}

	// Host metrics are stored at 1-second granularity — prune hourly to keep the
	// table bounded near its (hours-based) retention window.
	hostMetricsCleanupTask := asynq.NewTask(TypeHostMetricsCleanup, nil)
	if _, err := scheduler.Register("@every 1h", hostMetricsCleanupTask, asynq.Queue("low")); err != nil {
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

	// Per-database scheduled-backup sweep every minute — enqueues due configs.
	backupSweepTask := asynq.NewTask(TypeBackupSchedSweep, nil)
	if _, err := scheduler.Register("@every 1m", backupSweepTask, asynq.Queue("low")); err != nil {
		return nil, err
	}

	// TLS status sweep every minute — probes what the proxy actually serves for
	// each domain, so a stuck certificate surfaces in the UI within a minute
	// rather than never.
	tlsSweepTask := asynq.NewTask(TypeTLSStatusSweep, nil)
	if _, err := scheduler.Register("@every 1m", tlsSweepTask, asynq.Queue("low")); err != nil {
		return nil, err
	}

	slog.Info("starting scheduler (cleanup: 24h, retention: 24h, host-metrics-cleanup: 1h, auth-token-cleanup: 1h, quota-sweep: 6h, backup-rotate: 24h, backup-sched-sweep: 1m, tls-status-sweep: 1m)")
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
