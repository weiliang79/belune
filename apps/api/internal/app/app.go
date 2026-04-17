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

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/build/buildpacks"
	"github.com/ungweiliang/selfhost-paas/internal/build/dockerfile"
	"github.com/ungweiliang/selfhost-paas/internal/build/image"
	"github.com/ungweiliang/selfhost-paas/internal/build/railpack"
	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/eventwatcher"
	"github.com/ungweiliang/selfhost-paas/internal/logcollector"
	"github.com/ungweiliang/selfhost-paas/internal/logtailer"
	"github.com/ungweiliang/selfhost-paas/internal/migrations"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/proxy/caddy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime/docker"
	"github.com/ungweiliang/selfhost-paas/internal/server"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/terminal"
	"github.com/ungweiliang/selfhost-paas/internal/worker"
	"github.com/ungweiliang/selfhost-paas/internal/ws"
)

// App holds all application dependencies and owns their lifecycle.
type App struct {
	cfg          *config.Config
	db           *pgxpool.Pool
	queries      *generated.Queries
	dockerClient *docker.Client
	caddyClient  *caddy.Client
	asynqClient  *asynq.Client
	rdb          *redis.Client
	hub          *ws.Hub
	auditSvc     *service.AuditService
	termMgr      *terminal.Manager
	worker       *worker.Worker
	scheduler    *asynq.Scheduler
	httpServer   *http.Server
	reconciler   *proxy.Reconciler
	logTailer    *logtailer.Tailer
	logCollector *logcollector.Collector
	redisAdapter *ws.RedisAdapter
	eventWatcher *eventwatcher.Watcher
}

// New initialises all application dependencies in dependency order.
// Returns an error if any required resource (DB, Docker, Redis) is unavailable.
func New(cfg *config.Config) (*App, error) {
	db, err := store.Connect(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := store.RunMigrations(cfg.DatabaseURL, migrations.Files); err != nil {
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

	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
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
	termMgr := terminal.NewManager(cfg.MaxTerminalSessionsPerUser)
	hub := ws.NewHub(cfg.MaxWebSocketConnsPerUser)

	taskHandler := &worker.TaskHandler{
		Runtime:        dockerClient,
		Proxy:          caddyClient,
		DB:             db,
		Queries:        queries,
		Chain:          buildChain,
		Keyring:        cfg.Keyring,
		RedisClient:    rdb,
		Config:         cfg,
		MetricsService: metricsSvc,
	}

	w := worker.New(redisOpt, taskHandler)

	scheduler, err := w.StartScheduler()
	if err != nil {
		slog.Warn("failed to start cleanup scheduler", "error", err)
	}

	caddyClient.InitCatchAll(context.Background())
	reconciler := proxy.NewReconciler(queries, caddyClient, 30*time.Second)

	if err := caddyClient.ConfigureAccessLogs(context.Background()); err != nil {
		slog.Warn("failed to configure Caddy access logs", "error", err)
	}

	broadcaster := ws.NewContainerStatusBroadcaster(hub)
	httpSrv := server.New(cfg, db, queries, asynqClient, dockerClient, caddyClient, reconciler, rdb, hub, auditSvc, termMgr)

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
		reconciler:   reconciler,
		logTailer:    logtailer.New(cfg.AccessLogPath, queries, rdb),
		logCollector: logcollector.New(dockerClient, queries, rdb),
		redisAdapter: ws.NewRedisAdapter(rdb, hub),
		eventWatcher: eventwatcher.New(dockerClient, queries, broadcaster),
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
	g.Go(func() error { a.redisAdapter.RunAppLogAdapter(gctx); return nil })
	g.Go(func() error { ws.RunAppMetricsBroadcaster(gctx, a.hub, a.dockerClient, a.queries); return nil })
	g.Go(func() error { a.eventWatcher.Run(gctx); return nil })
	g.Go(func() error { a.auditSvc.Run(gctx); return nil })

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

		// Stop the worker (unblocks the worker.Start goroutine above)
		a.worker.Stop()

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
	a.rdb.Close()
	a.dockerClient.Close()
	a.db.Close()

	slog.Info("server stopped")
	return nil
}
