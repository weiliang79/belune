package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/weiliang79/belune/internal/app"
	"github.com/weiliang79/belune/internal/config"
	"github.com/weiliang79/belune/internal/pkg/logger"
	"github.com/weiliang79/belune/internal/pkg/tracing"
	"github.com/weiliang79/belune/internal/version"
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
	// Console by default — the JSON handler is unreadable in a terminal. Both
	// stay wrapped in RedactHandler, which strips secrets (deploy hook tokens
	// and friends) from messages and attributes: dropping that wrapper would
	// print them.
	var base slog.Handler
	if strings.EqualFold(cfg.LogFormat, "json") {
		base = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		opts := &slog.HandlerOptions{Level: logLevel}
		switch strings.ToLower(cfg.LogColor) {
		case "always":
			base = logger.NewConsoleHandlerWithColor(os.Stdout, opts, true)
		case "never":
			base = logger.NewConsoleHandlerWithColor(os.Stdout, opts, false)
		default: // auto — colour only when stdout is a terminal
			base = logger.NewConsoleHandler(os.Stdout, opts)
		}
	}
	slog.SetDefault(slog.New(logger.NewRedactHandler(base)))

	traceShutdown, err := tracing.Init(context.Background(), tracing.Config{
		Endpoint:       cfg.OTLPEndpoint,
		Insecure:       cfg.OTLPInsecure,
		ServiceName:    "belune-api",
		ServiceVersion: version.Version,
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
