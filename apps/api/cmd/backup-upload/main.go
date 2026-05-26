// backup-upload is a single-shot helper that uploads one local backup archive
// to the configured S3-compatible object store.
//
// Usage: backup-upload <local-path>
//
// Exits 0 on success, 1 on any error. Reads all BACKUP_* and other required
// config from environment variables (same as the API server).
//
// The helper is copied out of the API container image to ${INSTALL_DIR}/bin/
// by install.sh and update.sh so it runs on the host, outside Docker.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/service/backup"
)

func main() {
	if len(os.Args) != 2 {
		slog.Error("usage: backup-upload <local-path>")
		os.Exit(1)
	}
	localPath := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	svc := backup.New(cfg)
	if !svc.Enabled() {
		slog.Error("remote backup is not enabled", "hint", "set BACKUP_REMOTE_ENABLED=true and BACKUP_S3_BUCKET")
		os.Exit(1)
	}

	ctx := context.Background()
	key, err := svc.Upload(ctx, localPath)
	if err != nil {
		slog.Error("upload failed", "path", localPath, "error", err)
		os.Exit(1)
	}

	fmt.Printf("uploaded: %s\n", key)
}
