package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/build/buildpacks"
	"github.com/ungweiliang/selfhost-paas/internal/build/dockerfile"
	"github.com/ungweiliang/selfhost-paas/internal/build/image"
	"github.com/ungweiliang/selfhost-paas/internal/build/railpack"
	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/migrations"
	"github.com/ungweiliang/selfhost-paas/internal/proxy/caddy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime/docker"
	"github.com/ungweiliang/selfhost-paas/internal/server"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/worker"
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

	w := worker.New(redisOpt, taskHandler)
	go func() {
		if err := w.Start(); err != nil {
			slog.Error("worker failed to start", "error", err)
		}
	}()
	defer w.Stop()

	// Metrics collection on a precise 1s ticker (not via asynq scheduler)
	metricsCtx, cancelMetrics := context.WithCancel(context.Background())
	defer cancelMetrics()
	go w.StartMetricsTicker(metricsCtx)

	// Cleanup scheduler (runs every 24h)
	scheduler, err := w.StartScheduler()
	if err != nil {
		slog.Warn("failed to start cleanup scheduler", "error", err)
	} else {
		defer scheduler.Shutdown()
	}

	// HTTP server
	srv := server.New(cfg, db, queries, asynqClient, dockerClient, caddyClient, rdb)

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server stopped")
}
