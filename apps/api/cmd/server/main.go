package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ungweiliang/selfhost-paas/internal/app"
	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/logger"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/tracing"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		logLevel = slog.LevelInfo
	}
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(logger.NewRedactHandler(jsonHandler)))

	traceShutdown, err := tracing.Init(context.Background(), tracing.Config{
		Endpoint:       cfg.OTLPEndpoint,
		Insecure:       cfg.OTLPInsecure,
		ServiceName:    "selfhost-paas-api",
		ServiceVersion: "v0.0.8-alpha",
	})
	if err != nil {
		slog.Error("failed to init tracing", "error", err)
		os.Exit(1)
	}

	a, err := app.New(cfg)
	if err != nil {
		slog.Error("failed to initialise application", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := a.Run(ctx); err != nil {
		slog.Error("application error", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	if err := traceShutdown(shutdownCtx); err != nil {
		slog.Error("tracing shutdown error", "error", err)
	}
}
