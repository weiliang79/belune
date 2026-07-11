// Package app wires all application dependencies and manages their lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/weiling79/belune/internal/build"
	"github.com/weiling79/belune/internal/build/buildpacks"
	"github.com/weiling79/belune/internal/build/dockerfile"
	"github.com/weiling79/belune/internal/build/image"
	"github.com/weiling79/belune/internal/build/railpack"
	"github.com/weiling79/belune/internal/config"
	"github.com/weiling79/belune/internal/eventwatcher"
	"github.com/weiling79/belune/internal/logcollector"
	"github.com/weiling79/belune/internal/logtailer"
	"github.com/weiling79/belune/internal/migrations"
	"github.com/weiling79/belune/internal/pkg/metrics"
	"github.com/weiling79/belune/internal/proxy"
	"github.com/weiling79/belune/internal/proxy/caddy"
	"github.com/weiling79/belune/internal/quota"
	"github.com/weiling79/belune/internal/runtime/docker"
	"github.com/weiling79/belune/internal/server"
	"github.com/weiling79/belune/internal/service"
	"github.com/weiling79/belune/internal/service/backup"
	"github.com/weiling79/belune/internal/service/email"
	"github.com/weiling79/belune/internal/store"
	"github.com/weiling79/belune/internal/store/generated"
	"github.com/weiling79/belune/internal/terminal"
	"github.com/weiling79/belune/internal/worker"
	"github.com/weiling79/belune/internal/ws"
)

// App holds all application dependencies and owns their lifecycle.
type App struct {
	cfg            *config.Config
	db             *pgxpool.Pool
	queries        *generated.Queries
	dockerClient   *docker.Client
	caddyClient    *caddy.Client
	asynqClient    *asynq.Client
	rdb            *redis.Client
	hub            *ws.Hub
	auditSvc       *service.AuditService
	notifySvc      *service.NotificationService
	termMgr        *terminal.Manager
	worker         *worker.Worker
	scheduler      *asynq.Scheduler
	httpServer     *http.Server
	metricsServer  *http.Server // optional separate listener (METRICS_BIND)
	asynqInspector *asynq.Inspector
	reconciler     *proxy.Reconciler
	logTailer      *logtailer.Tailer
	logCollector   *logcollector.Collector
	redisAdapter   *ws.RedisAdapter
	eventWatcher   *eventwatcher.Watcher
}

// New initialises all application dependencies in dependency order.
// Returns an error if any required resource (DB, Docker, Redis) is unavailable.
func New(cfg *config.Config) (*App, error) {
	db, err := store.Connect(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if cfg.SkipMigrations {
		slog.Warn("BELUNE_SKIP_MIGRATIONS=true — auto-migration disabled, schema may be ahead or behind binary expectations")
	} else if err := store.RunMigrations(cfg.DatabaseURL, migrations.Files); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	queries := generated.New(db)

	dockerClient, err := docker.New()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	caddyClient := caddy.New(cfg.CaddyAdminURL)

	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		dockerClient.Close()
		db.Close()
		return nil, fmt.Errorf("parse redis URL for asynq: %w", err)
	}
	asynqClient := asynq.NewClient(redisOpt)
	asynqInspector := asynq.NewInspector(redisOpt)

	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		asynqInspector.Close()
		asynqClient.Close()
		dockerClient.Close()
		db.Close()
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	rdb := redis.NewClient(redisOptions)

	buildChain := build.NewChain(
		dockerfile.New(dockerClient),
		buildpacks.New(dockerClient),
		railpack.New(),
		image.New(dockerClient),
	)

	metricsSvc := service.NewMetricsService(queries, rdb)
	auditSvc := service.NewAuditService(queries)
	notifySvc := service.NewNotificationService(queries, rdb)
	termMgr := terminal.NewManager(cfg.MaxTerminalSessionsPerUser)
	hub := ws.NewHub(cfg.MaxWebSocketConnsPerUser)

	appSvc := service.NewApplicationService(db, queries, dockerClient, cfg.Keyring, cfg.FileMountsDir)
	gitProviderSvc := service.NewGitProviderConfigService(queries, cfg.Keyring)
	gitIntegrationSvc := service.NewGitIntegrationService(queries, cfg.Keyring, gitProviderSvc)
	quotaSvc := quota.NewService(queries)
	emailSvc, err := email.New(cfg)
	if err != nil {
		rdb.Close()
		asynqInspector.Close()
		asynqClient.Close()
		dockerClient.Close()
		db.Close()
		return nil, fmt.Errorf("load email templates: %w", err)
	}
	backupSvc := backup.New(cfg)
	backupDestSvc := service.NewBackupDestinationService(queries, cfg.Keyring)
	taskHandler := &worker.TaskHandler{
		Runtime:               dockerClient,
		Proxy:                 caddyClient,
		DB:                    db,
		Queries:               queries,
		Chain:                 buildChain,
		Keyring:               cfg.Keyring,
		RedisClient:           rdb,
		Config:                cfg,
		MetricsService:        metricsSvc,
		AppService:            appSvc,
		GitIntegrationService: gitIntegrationSvc,
		QuotaService:          quotaSvc,
		EmailService:          emailSvc,
		BackupService:         backupSvc,
		BackupDestinations:    backupDestSvc,
		AuditLog:              auditSvc,
		Notifier:              notifySvc,
		Enqueuer:              asynqClient,
	}

	w := worker.New(redisOpt, taskHandler)

	scheduler, err := w.StartScheduler()
	if err != nil {
		slog.Warn("failed to start cleanup scheduler", "error", err)
	}

	caddyClient.InitCatchAll(context.Background())
	reconciler := proxy.NewReconciler(queries, caddyClient, cfg.Keyring, 30*time.Second)

	if err := caddyClient.ConfigureAccessLogs(context.Background()); err != nil {
		slog.Warn("failed to configure Caddy access logs", "error", err)
	}

	broadcaster := ws.NewContainerStatusBroadcaster(hub)
	httpSrv := server.New(cfg, db, queries, asynqClient, asynqInspector, dockerClient, caddyClient, reconciler, rdb, hub, auditSvc, notifySvc, termMgr, emailSvc)

	// Optional Prometheus-friendly bind. Serves /metrics without auth on the
	// configured address (typically loopback). Keeps the main /metrics route
	// gated behind admin auth for UI browsers.
	var metricsServer *http.Server
	if cfg.MetricsBind != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		metricsServer = &http.Server{
			Addr:              cfg.MetricsBind,
			Handler:           mux,
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
		}
	}

	return &App{
		cfg:          cfg,
		db:           db,
		queries:      queries,
		dockerClient: dockerClient,
		caddyClient:  caddyClient,
		asynqClient:  asynqClient,
		rdb:          rdb,
		hub:          hub,
		auditSvc:     auditSvc,
		notifySvc:    notifySvc,
		termMgr:      termMgr,
		worker:       w,
		scheduler:    scheduler,
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      httpSrv.Router(),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 0, // Disabled to support SSE streaming endpoints
			IdleTimeout:  60 * time.Second,
		},
		metricsServer:  metricsServer,
		asynqInspector: asynqInspector,
		reconciler:     reconciler,
		logTailer:      logtailer.New(cfg.AccessLogPath, queries, rdb),
		logCollector:   logcollector.New(dockerClient, queries, rdb),
		redisAdapter:   ws.NewRedisAdapter(rdb, hub),
		eventWatcher:   eventwatcher.New(dockerClient, queries, broadcaster),
	}, nil
}

// Run starts all background goroutines and the HTTP server. It blocks until ctx
// is cancelled or a goroutine fails fatally, then coordinates an ordered shutdown
// before returning. The caller should invoke Shutdown to release remaining resources.
func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	// HTTP server
	g.Go(func() error {
		slog.Info("server starting", "port", a.cfg.Port)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	// Worker — blocks until stopped
	g.Go(func() error {
		if err := a.worker.Start(); err != nil {
			return fmt.Errorf("worker: %w", err)
		}
		return nil
	})

	// Context-driven goroutines — all return when gctx is cancelled
	g.Go(func() error { a.worker.StartMetricsTicker(gctx); return nil })
	g.Go(func() error { a.reconciler.Run(gctx); return nil })
	g.Go(func() error { a.logTailer.Run(gctx); return nil })
	g.Go(func() error { a.logCollector.Run(gctx); return nil })
	g.Go(func() error { a.hub.Run(gctx); return nil })
	g.Go(func() error { a.redisAdapter.RunBuildLogAdapter(gctx); return nil })
	g.Go(func() error { a.redisAdapter.RunHostMetricsAdapter(gctx); return nil })
	g.Go(func() error { a.redisAdapter.RunRequestLogAdapter(gctx); return nil })
	g.Go(func() error { a.redisAdapter.RunContainerLogAdapter(gctx); return nil })
	g.Go(func() error { ws.RunAppMetricsBroadcaster(gctx, a.hub, a.dockerClient, a.queries); return nil })
	g.Go(func() error { a.eventWatcher.Run(gctx); return nil })
	g.Go(func() error { a.auditSvc.Run(gctx); return nil })
	g.Go(func() error { a.notifySvc.Run(gctx); return nil })
	g.Go(func() error { metrics.AsynqQueuePoller(gctx, a.asynqInspector, 15*time.Second); return nil })

	if a.metricsServer != nil {
		g.Go(func() error {
			slog.Info("metrics server starting", "addr", a.metricsServer.Addr)
			if err := a.metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("metrics server: %w", err)
			}
			return nil
		})
	}

	// Shutdown coordinator — triggered when gctx is cancelled (signal or fatal error)
	g.Go(func() error {
		<-gctx.Done()
		slog.Info("shutting down server...")

		// Stop HTTP server first (drain in-flight requests)
		httpCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.httpServer.Shutdown(httpCtx); err != nil {
			slog.Error("http server forced to shutdown", "error", err)
		}
		if a.metricsServer != nil {
			if err := a.metricsServer.Shutdown(httpCtx); err != nil {
				slog.Error("metrics server forced to shutdown", "error", err)
			}
		}

		// Stop the worker (no new tasks), then drain in-flight tasks.
		// Note: the asynq Scheduler is shut down in Shutdown() after g.Wait()
		// returns. A scheduler tick that fires in this window may enqueue a
		// task that the stopped processor won't pick up until next start — this
		// is acceptable (tasks are idempotent and will run on next restart).
		a.worker.Stop()
		a.worker.Shutdown()

		return nil
	})

	return g.Wait()
}

// Shutdown releases remaining resources after Run has returned. It should be
// called with a timeout context.
func (a *App) Shutdown(ctx context.Context) error {
	a.termMgr.CloseAll()

	if a.scheduler != nil {
		a.scheduler.Shutdown()
	}

	a.asynqClient.Close()
	if a.asynqInspector != nil {
		a.asynqInspector.Close()
	}
	a.rdb.Close()
	a.dockerClient.Close()
	a.db.Close()

	slog.Info("server stopped")
	return nil
}
