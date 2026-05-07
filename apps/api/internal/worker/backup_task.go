package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// HandleBackupNowTask shells out to backup.sh and records the result in
// backup_runs. The task does not retry on failure — a second run is
// triggered by the user or the daily timer.
func (h *TaskHandler) HandleBackupNowTask(ctx context.Context, t *asynq.Task) error {
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
	}

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		msg := fmt.Sprintf("backup script not found at %s; use 'systemctl start paas-backup.service' instead", scriptPath)
		h.finaliseRun(ctx, run.ID, msg)
		return fmt.Errorf("%s: %w", msg, asynq.SkipRetry)
	}

	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	out, execErr := cmd.CombinedOutput()

	if execErr != nil {
		errMsg := fmt.Sprintf("%v\n%s", execErr, string(out))
		h.finaliseRun(ctx, run.ID, errMsg)
		return fmt.Errorf("backup script failed: %w: %w", execErr, asynq.SkipRetry)
	}

	slog.Info("backup_now: script completed successfully", "output_bytes", len(out))
	h.finaliseRun(ctx, run.ID, "")
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
func (h *TaskHandler) finaliseRun(ctx context.Context, id pgtype.UUID, errMsg string) {
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
		RemoteKey:  pgtype.Text{},
		SizeBytes:  0,
		Error:      errText,
	}); err != nil {
		slog.Warn("backup_now: failed to update run record", "error", err)
	}
}
