package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/robfig/cron/v3"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// HandleBackupScheduleSweep runs every minute: for each enabled per-database
// backup config whose cron schedule has elapsed since its last run, it enqueues
// a backup_db task and stamps last_run_at. Best-effort — a failed enqueue leaves
// last_run_at untouched so the next sweep retries.
func (h *TaskHandler) HandleBackupScheduleSweep(ctx context.Context) {
	if h.Enqueuer == nil {
		return
	}

	configs, err := h.Queries.ListEnabledDatabaseBackupConfigs(ctx)
	if err != nil {
		slog.Warn("backup sweep: list configs", "error", err)
		return
	}

	now := time.Now()
	for _, cfg := range configs {
		if !backupConfigDue(cfg, now) {
			continue
		}

		// Skip if a backup is already running for this database to avoid
		// concurrent archive writes against the same container.
		if h.databaseBackupInProgress(ctx, cfg.DatabaseID) {
			continue
		}

		payload, mErr := json.Marshal(backupDBPayload{
			DatabaseID:     formatUUID(cfg.DatabaseID),
			BackupConfigID: formatUUID(cfg.ID),
		})
		if mErr != nil {
			slog.Warn("backup sweep: marshal payload", "config_id", formatUUID(cfg.ID), "error", mErr)
			continue
		}

		if _, eErr := h.Enqueuer.Enqueue(asynq.NewTask(TypeBackupDB, payload),
			asynq.Queue("default"), asynq.MaxRetry(1)); eErr != nil {
			slog.Warn("backup sweep: enqueue backup", "config_id", formatUUID(cfg.ID), "error", eErr)
			continue
		}

		if tErr := h.Queries.TouchDatabaseBackupConfigRun(ctx, generated.TouchDatabaseBackupConfigRunParams{
			ID:        cfg.ID,
			LastRunAt: pgtype.Timestamptz{Time: now, Valid: true},
		}); tErr != nil {
			slog.Warn("backup sweep: stamp last_run_at", "config_id", formatUUID(cfg.ID), "error", tErr)
		}
	}
}

// backupConfigDue reports whether cfg's schedule has fired since its last run.
// The baseline is last_run_at, or created_at for a config that has never run
// (so a new config fires at its next scheduled occurrence, not immediately).
// An unparseable schedule is treated as not-due (validated at create/update).
func backupConfigDue(cfg generated.DatabaseBackupConfig, now time.Time) bool {
	sched, err := cron.ParseStandard(cfg.Schedule)
	if err != nil {
		slog.Warn("backup sweep: invalid schedule", "config_id", formatUUID(cfg.ID), "schedule", cfg.Schedule, "error", err)
		return false
	}
	baseline := cfg.CreatedAt.Time
	if cfg.LastRunAt.Valid {
		baseline = cfg.LastRunAt.Time
	}
	next := sched.Next(baseline)
	return !next.After(now)
}

// databaseBackupInProgress reports whether the most recent backup run for a
// database is still running.
func (h *TaskHandler) databaseBackupInProgress(ctx context.Context, dbID pgtype.UUID) bool {
	recent, err := h.Queries.ListDatabaseBackups(ctx, generated.ListDatabaseBackupsParams{
		DatabaseID: dbID,
		Limit:      1,
	})
	if err != nil {
		// On a query error, be conservative and skip this tick.
		slog.Warn("backup sweep: check in-progress", "database_id", formatUUID(dbID), "error", err)
		return true
	}
	return len(recent) > 0 && recent[0].Status == "running"
}
