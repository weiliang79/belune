package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

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
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/runtime/docker"
	"github.com/ungweiliang/selfhost-paas/internal/server"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/worker"
	"github.com/ungweiliang/selfhost-paas/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Configure structured JSON logging with configurable level
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		logLevel = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	// Database
	db, err := store.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run database migrations
	if err := store.RunMigrations(cfg.DatabaseURL, migrations.Files); err != nil {
		slog.Error("failed to run database migrations", "error", err)
		os.Exit(1)
	}

	queries := generated.New(db)

	// Docker runtime
	dockerClient, err := docker.New()
	if err != nil {
		slog.Error("failed to create docker client", "error", err)
		os.Exit(1)
	}
	defer dockerClient.Close()

	// Caddy proxy
	caddyClient := caddy.New(cfg.CaddyAdminURL)

	// Asynq client for enqueuing tasks
	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to parse redis URL", "error", err)
		os.Exit(1)
	}
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	// Redis client for build log pub/sub
	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to parse redis URL for pub/sub", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(redisOptions)
	defer rdb.Close()

	// Build chain: Dockerfile → Buildpacks → Railpack
	buildChain := build.NewChain(
		dockerfile.New(dockerClient),
		buildpacks.New(dockerClient),
		railpack.New(),
		image.New(dockerClient),
	)

	// Services
	metricsSvc := service.NewMetricsService(queries, rdb)

	// Worker for background tasks
	taskHandler := &worker.TaskHandler{
		Runtime:        dockerClient,
		Proxy:          caddyClient,
		DB:             db,
		Queries:        queries,
		Chain:          buildChain,
		EncryptionKey:  cfg.EncryptionKey,
		RedisClient:    rdb,
		Config:         cfg,
		MetricsService: metricsSvc,
	}

	var wg sync.WaitGroup

	w := worker.New(redisOpt, taskHandler)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := w.Start(); err != nil {
			slog.Error("worker failed to start — job queue unavailable, shutting down", "error", err)
			os.Exit(1)
		}
	}()

	// Metrics collection on a precise 1s ticker (not via asynq scheduler)
	metricsCtx, cancelMetrics := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.StartMetricsTicker(metricsCtx)
	}()

	// Normalise Caddy route ordering: domain-specific routes first, catch-all last.
	caddyClient.InitCatchAll(context.Background())

	// Recover proxy routes for all running applications (in case Caddy restarted).
	recoverProxyRoutes(context.Background(), queries, caddyClient)

	// Configure Caddy access logging AFTER route setup — InitCatchAll patches srv0
	// with {listen, routes} which clears the logs key if done in the wrong order.
	if err := caddyClient.ConfigureAccessLogs(context.Background()); err != nil {
		slog.Warn("failed to configure Caddy access logs", "error", err)
	}

	tailerCtx, cancelTailer := context.WithCancel(context.Background())
	accessLogTailer := logtailer.New(cfg.AccessLogPath, queries, rdb)
	wg.Add(1)
	go func() {
		defer wg.Done()
		accessLogTailer.Run(tailerCtx)
	}()

	// Application log collector: follows container logs and persists to DB
	collectorCtx, cancelCollector := context.WithCancel(context.Background())
	appLogCollector := logcollector.New(dockerClient, queries, rdb)
	wg.Add(1)
	go func() {
		defer wg.Done()
		appLogCollector.Run(collectorCtx)
	}()

	// Cleanup scheduler (runs every 24h)
	scheduler, err := w.StartScheduler()
	if err != nil {
		slog.Warn("failed to start cleanup scheduler", "error", err)
	} else {
		defer scheduler.Shutdown()
	}

	// WebSocket hub
	hub := ws.NewHub()
	hubCtx, cancelHub := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		hub.Run(hubCtx)
	}()

	// Redis → WebSocket adapters
	adapterCtx, cancelAdapters := context.WithCancel(context.Background())
	redisAdapter := ws.NewRedisAdapter(rdb, hub)
	wg.Add(5)
	go func() { defer wg.Done(); redisAdapter.RunBuildLogAdapter(adapterCtx) }()
	go func() { defer wg.Done(); redisAdapter.RunHostMetricsAdapter(adapterCtx) }()
	go func() { defer wg.Done(); redisAdapter.RunRequestLogAdapter(adapterCtx) }()
	go func() { defer wg.Done(); redisAdapter.RunAppLogAdapter(adapterCtx) }()
	go func() { defer wg.Done(); runAppMetricsBroadcaster(adapterCtx, hub, dockerClient, queries) }()

	// Docker event watcher for real-time container status
	broadcaster := ws.NewContainerStatusBroadcaster(hub)
	eventWatcher := eventwatcher.New(dockerClient, queries, broadcaster)
	watcherCtx, cancelWatcher := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventWatcher.Run(watcherCtx)
	}()

	// Audit logging service (non-blocking, buffered)
	auditSvc := service.NewAuditService(queries)
	auditCtx, cancelAudit := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		auditSvc.Run(auditCtx)
	}()

	// HTTP server
	srv := server.New(cfg, db, queries, asynqClient, dockerClient, caddyClient, rdb, hub, auditSvc)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // Disabled to support SSE streaming endpoints
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// Stop HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	// Cancel background goroutines
	cancelWatcher()
	cancelAdapters()
	cancelHub()
	cancelAudit()
	cancelMetrics()
	cancelTailer()
	cancelCollector()
	w.Stop()

	// Wait for goroutines to finish with a timeout
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		slog.Info("all goroutines stopped")
	case <-time.After(10 * time.Second):
		slog.Warn("goroutines did not finish within timeout")
	}

	slog.Info("server stopped")
}

// runAppMetricsBroadcaster polls Docker container stats for applications that
// have active WebSocket subscribers on "metrics:app:{applicationId}" and
// broadcasts them to the hub. Runs until ctx is cancelled.
func runAppMetricsBroadcaster(
	ctx context.Context,
	hub *ws.Hub,
	rt runtime.ContainerRuntime,
	queries *generated.Queries,
) {
	const channelPrefix = "metrics:app:"
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			channels := hub.ActiveChannelsWithPrefix(channelPrefix)
			for _, ch := range channels {
				appIDStr := ch[len(channelPrefix):]
				var appUUID pgtype.UUID
				if err := appUUID.Scan(appIDStr); err != nil {
					continue
				}
				row, err := queries.GetApplicationWithProjectSlug(ctx, appUUID)
				if err != nil {
					continue
				}
				containerName := fmt.Sprintf("%s-%s-%s", row.ProjectSlug, row.Slug, appIDStr[:8])
				stats, err := rt.ContainerStats(ctx, containerName)
				if err != nil {
					slog.Debug("app metrics: stats fetch failed", "container", containerName, "error", err)
					continue
				}
				point := map[string]any{
					"cpu_percent":      stats.CPUPercent,
					"memory_usage":     stats.MemoryUsage,
					"memory_limit":     stats.MemoryLimit,
					"network_rx_bytes": stats.NetworkRxBytes,
					"network_tx_bytes": stats.NetworkTxBytes,
					"recorded_at":      time.Now().Format(time.RFC3339),
				}
				data, err := json.Marshal(point)
				if err != nil {
					continue
				}
				hub.Broadcast(ch, "metrics", data)
			}
		}
	}
}

// recoverProxyRoutes re-registers Caddy routes for all running applications.
// This ensures routes survive Caddy restarts since dynamically added routes
// are not persisted in the Caddyfile.
func recoverProxyRoutes(ctx context.Context, queries *generated.Queries, proxyClient *caddy.Client) {
	apps, err := queries.ListAllApplications(ctx)
	if err != nil {
		slog.Warn("failed to list applications for route recovery", "error", err)
		return
	}

	var recovered int
	for _, app := range apps {
		if app.Status != "running" {
			continue
		}

		appIDStr := fmt.Sprintf("%x-%x-%x-%x-%x",
			app.ID.Bytes[0:4], app.ID.Bytes[4:6], app.ID.Bytes[6:8],
			app.ID.Bytes[8:10], app.ID.Bytes[10:16])

		row, err := queries.GetApplicationWithProjectSlug(ctx, app.ID)
		if err != nil {
			slog.Warn("route recovery: failed to get project slug", "app", appIDStr, "error", err)
			continue
		}

		containerName := fmt.Sprintf("%s-%s-%s", row.ProjectSlug, app.Slug, appIDStr[:8])

		domains, err := queries.ListDomainsByApplication(ctx, app.ID)
		if err != nil {
			slog.Warn("route recovery: failed to list domains", "app", appIDStr, "error", err)
			continue
		}

		for _, domain := range domains {
			var port int32 = 8080
			if domain.ContainerPort.Valid {
				port = domain.ContainerPort.Int32
			}

			// Load route features
			var features []proxy.RouteFeature
			dbFeatures, fErr := queries.ListRouteFeaturesByDomain(ctx, domain.ID)
			if fErr == nil {
				for _, f := range dbFeatures {
					var cfg map[string]any
					if len(f.Config) > 0 {
						json.Unmarshal(f.Config, &cfg)
					}
					features = append(features, proxy.RouteFeature{
						Type:    f.FeatureType,
						Config:  cfg,
						Enabled: f.Enabled,
					})
				}
			}

			if err := proxyClient.AddRoute(ctx, proxy.RouteConfig{
				Hostname:       domain.Hostname,
				TargetURL:      fmt.Sprintf("http://%s:%d", containerName, port),
				TLS:            domain.SslEnabled,
				ForceHTTPS:     domain.ForceHttps,
				SSLMode:        domain.SslMode,
				SSLProvider:    domain.SslProvider.String,
				CertPath:       domain.CertPath.String,
				KeyPath:        domain.KeyPath.String,
				Features:       features,
				AdvancedConfig: domain.AdvancedConfig,
			}); err != nil {
				slog.Warn("route recovery: failed to add route", "hostname", domain.Hostname, "error", err)
				continue
			}
			recovered++
		}
	}

	if recovered > 0 {
		slog.Info("recovered proxy routes", "count", recovered)
	}
}
