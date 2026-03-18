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

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/build/buildpacks"
	"github.com/ungweiliang/selfhost-paas/internal/build/dockerfile"
	"github.com/ungweiliang/selfhost-paas/internal/build/image"
	"github.com/ungweiliang/selfhost-paas/internal/build/nixpacks"
	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/proxy/caddy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime/docker"
	"github.com/ungweiliang/selfhost-paas/internal/server"
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

	// Database
	db, err := store.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

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

	// Build chain: Dockerfile → Buildpacks → Nixpacks
	buildChain := build.NewChain(
		dockerfile.New(dockerClient),
		buildpacks.New(dockerClient),
		nixpacks.New(),
		image.New(dockerClient),
	)

	// Worker for background tasks
	taskHandler := &worker.TaskHandler{
		Runtime:       dockerClient,
		Proxy:         caddyClient,
		Queries:       queries,
		Chain:         buildChain,
		EncryptionKey: cfg.EncryptionKey,
	}

	w := worker.New(redisOpt, taskHandler)
	go func() {
		if err := w.Start(); err != nil {
			slog.Error("worker failed to start", "error", err)
		}
	}()
	defer w.Stop()

	// Cleanup scheduler (runs every 24h)
	scheduler, err := w.StartScheduler()
	if err != nil {
		slog.Warn("failed to start cleanup scheduler", "error", err)
	} else {
		defer scheduler.Shutdown()
	}

	// HTTP server
	srv := server.New(cfg, db, queries, asynqClient, dockerClient, caddyClient)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
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
