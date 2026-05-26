package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/metrics"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/tracing"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// HandleBackupNowTask shells out to backup.sh and records the result in
// backup_runs. The task does not retry on failure — a second run is
// triggered by the user or the daily timer.
func (h *TaskHandler) HandleBackupNowTask(ctx context.Context, t *asynq.Task) error {
	ctx, span := tracing.Tracer().Start(ctx, "backup.run")
	defer span.End()

	scriptPath := "/opt/paas/scripts/backup.sh"
	if h.Config != nil && h.Config.BackupScriptPath != "" {
		scriptPath = h.Config.BackupScriptPath
	}

	var run generated.BackupRun
	if h.Queries != nil {
		var err error
		run, err = h.Queries.InsertBackupRun(ctx)
		if err != nil {
			slog.Warn("backup_now: failed to insert run record", "error", err)
		}
		if run.ID.Valid {
			span.SetAttributes(attribute.String("backup.run_id", formatUUID(run.ID)))
		}
	}

	start := time.Now()

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		msg := fmt.Sprintf("backup script not found at %s; use 'systemctl start paas-backup.service' instead", scriptPath)
		h.finaliseRun(ctx, run.ID, 0, pgtype.Text{}, msg)
		notFoundErr := errors.New(msg)
		metrics.RecordBackupRun("local", notFoundErr, time.Since(start))
		recordSpanErr(span, notFoundErr)
		return errors.Join(notFoundErr, asynq.SkipRetry)
	}

	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	out, execErr := cmd.CombinedOutput()

	if execErr != nil {
		errMsg := fmt.Sprintf("%v\n%s", execErr, string(out))
		h.finaliseRun(ctx, run.ID, 0, pgtype.Text{}, errMsg)
		metrics.RecordBackupRun("local", execErr, time.Since(start))
		recordSpanErr(span, execErr)
		return errors.Join(fmt.Errorf("backup script failed: %w", execErr), asynq.SkipRetry)
	}

	slog.Info("backup_now: script completed successfully", "output_bytes", len(out))

	archivePath, remoteKeyStr := parseBackupOutput(string(out))
	var sizeBytes int64
	if archivePath != "" {
		if info, err := os.Stat(archivePath); err == nil {
			sizeBytes = info.Size()
		} else {
			slog.Warn("backup_now: could not stat archive", "path", archivePath, "error", err)
		}
	}
	remoteKey := pgtype.Text{}
	if remoteKeyStr != "" {
		remoteKey = pgtype.Text{String: remoteKeyStr, Valid: true}
	}

	h.finaliseRun(ctx, run.ID, sizeBytes, remoteKey, "")

	destination := "local"
	if remoteKeyStr != "" {
		destination = "remote"
	}
	metrics.RecordBackupRun(destination, nil, time.Since(start))
	span.SetAttributes(
		attribute.String("backup.destination", destination),
		attribute.Int64("backup.size_bytes", sizeBytes),
	)
	return nil
}

// HandleBackupRotateTask applies the retention policy: deletes remote objects
// older than BackupRetainDays that are beyond the BackupRetainCount newest.
func (h *TaskHandler) HandleBackupRotateTask(ctx context.Context, t *asynq.Task) error {
	if h.BackupService == nil || !h.BackupService.Enabled() {
		slog.Debug("backup_rotate: remote backup not enabled, skipping")
		return nil
	}

	deleted, err := h.BackupService.Rotate(ctx)
	metrics.RecordBackupRotate(err)
	if err != nil {
		return fmt.Errorf("backup rotate: %w", err)
	}

	if len(deleted) > 0 {
		slog.Info("backup_rotate: removed old backups", "count", len(deleted))
	} else {
		slog.Debug("backup_rotate: nothing to prune")
	}
	return nil
}

// finaliseRun updates the backup_run record with the final status and
// optional error message. Logs and swallows DB errors — the backup itself
// already succeeded or failed; we don't want a DB hiccup to mask that.
func (h *TaskHandler) finaliseRun(ctx context.Context, id pgtype.UUID, sizeBytes int64, remoteKey pgtype.Text, errMsg string) {
	if h.Queries == nil || !id.Valid {
		return
	}

	status := "succeeded"
	errText := pgtype.Text{}
	if errMsg != "" {
		status = "failed"
		errText = pgtype.Text{String: errMsg, Valid: true}
	}

	if err := h.Queries.UpdateBackupRun(ctx, generated.UpdateBackupRunParams{
		ID:         id,
		FinishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Status:     status,
		RemoteKey:  remoteKey,
		SizeBytes:  sizeBytes,
		Error:      errText,
	}); err != nil {
		slog.Warn("backup_now: failed to update run record", "error", err)
	}
}

// parseBackupOutput scans combined script output for the local archive path
// and (optionally) the S3 remote key. Both may be empty if not present.
//
// backup.sh's success() emits: "  [ok]    Backup complete: <path>"
// backup-upload CLI emits:     "uploaded: <key>"
func parseBackupOutput(output string) (archivePath, remoteKey string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "[ok]"); ok {
			after = strings.TrimSpace(after)
			if p, ok2 := strings.CutPrefix(after, "Backup complete: "); ok2 {
				archivePath = strings.TrimSpace(p)
			}
		}
		if k, ok := strings.CutPrefix(line, "uploaded: "); ok {
			remoteKey = strings.TrimSpace(k)
		}
	}
	return
}
