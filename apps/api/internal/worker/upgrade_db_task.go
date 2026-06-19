package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// upgradeDBPayload triggers a guarded major-version upgrade of a managed
// database (known engines only).
type upgradeDBPayload struct {
	DatabaseID    string `json:"database_id"`
	TargetVersion string `json:"target_version"`
}

// HandleUpgradeDBTask performs a guarded dump-and-reload major-version upgrade:
//
//  1. Take a logical dump of the current database (recorded as a backup row —
//     this is the rollback artifact, also uploaded to S3 when enabled).
//  2. Remove the old container and wipe its volume.
//  3. Provision a fresh container at the target version and restore the dump.
//
// If provisioning or restore fails, the database is rolled back to the prior
// version from the same dump, so the database is never left empty. Only logical-
// dump engines (postgres/mysql/mongo) are eligible; "other" and redis are not.
//
// The task does not auto-retry: a failed upgrade either rolls back cleanly or
// leaves the database failed with its dump intact for manual recovery.
func (h *TaskHandler) HandleUpgradeDBTask(ctx context.Context, t *asynq.Task) error {
	var payload upgradeDBPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("unmarshal upgrade_db payload: %w", err), asynq.SkipRetry)
	}

	slog.Info("handling upgrade_db task", "database_id", payload.DatabaseID, "target_version", payload.TargetVersion)

	dbID, err := parseUUID(payload.DatabaseID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid database_id (permanent): %w", err), asynq.SkipRetry)
	}

	db, err := h.Queries.GetDatabase(ctx, dbID)
	if err != nil {
		return errors.Join(fmt.Errorf("get database: %w", err), asynq.SkipRetry)
	}
	if dbBackupMethod(db) != "logical" {
		h.failDatabase(ctx, dbID, fmt.Sprintf("upgrade not supported for type %s", db.Type))
		return errors.Join(fmt.Errorf("upgrade not supported for type %s (permanent)", db.Type), asynq.SkipRetry)
	}

	creds, err := h.decryptDBCredentials(db)
	if err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("credentials: %v", err))
		return errors.Join(fmt.Errorf("credentials (permanent): %w", err), asynq.SkipRetry)
	}

	oldVersion := db.Version

	// 1. Pre-upgrade dump — the rollback artifact, recorded as a backup row.
	dumpPath, err := h.dumpForUpgrade(ctx, db, creds)
	if err != nil {
		// The database was not touched; return it to running.
		h.setDatabaseStatus(ctx, dbID, "running")
		return errors.Join(fmt.Errorf("pre-upgrade dump failed: %w", err), asynq.SkipRetry)
	}

	var hostPort int32
	if db.HostPort.Valid {
		hostPort = db.HostPort.Int32
	}

	// 2. Switch the row to the target version and rebuild the container on a
	// fresh volume. The dump (step 1) is the only data copy from here until the
	// restore succeeds, so a failure rolls back from it.
	dbNew, err := h.Queries.UpdateDatabaseVersion(ctx, generated.UpdateDatabaseVersionParams{ID: dbID, Version: payload.TargetVersion})
	if err != nil {
		h.setDatabaseStatus(ctx, dbID, "running")
		return errors.Join(fmt.Errorf("update version: %w", err), asynq.SkipRetry)
	}

	h.removeDBContainer(ctx, db.Slug)
	if err := h.Runtime.RemoveVolume(ctx, db.Slug+"-vol"); err != nil {
		slog.Warn("upgrade: remove old volume (may not exist)", "database_id", payload.DatabaseID, "error", err)
	}

	if err := h.provisionDBContainer(ctx, dbNew, creds, hostPort); err != nil {
		return h.rollbackUpgrade(ctx, dbNew, creds, oldVersion, hostPort, dumpPath,
			fmt.Sprintf("provision target version: %v", err))
	}

	// 3. Restore the dump into the new-version container.
	if err := h.applyRestoreArchive(ctx, dbNew, creds, "logical", dumpPath); err != nil {
		return h.rollbackUpgrade(ctx, dbNew, creds, oldVersion, hostPort, dumpPath,
			fmt.Sprintf("restore into target version: %v", err))
	}

	slog.Info("database upgraded", "database_id", payload.DatabaseID, "from", oldVersion, "to", payload.TargetVersion)
	return nil
}

// dumpForUpgrade produces a logical dump recorded as a succeeded backup row and
// returns the local path used as the upgrade's rollback artifact.
func (h *TaskHandler) dumpForUpgrade(ctx context.Context, db generated.Database, creds map[string]string) (string, error) {
	run, err := h.Queries.InsertDatabaseBackup(ctx, db.ID)
	if err != nil {
		return "", fmt.Errorf("insert backup run: %w", err)
	}

	if err := os.MkdirAll(h.Config.DatabaseBackupDir, 0o755); err != nil {
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("create backup dir: %v", err))
		return "", err
	}
	fileName := fmt.Sprintf("%s-preupgrade-%s.backup.gz", db.Slug, time.Now().UTC().Format("20060102T150405Z"))
	localPath := filepath.Join(h.Config.DatabaseBackupDir, fileName)

	f, err := os.Create(localPath)
	if err != nil {
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("create backup file: %v", err))
		return "", err
	}
	archiveErr := h.writeBackupArchive(ctx, db, creds, "logical", f)
	closeErr := f.Close()
	if archiveErr != nil {
		_ = os.Remove(localPath)
		h.failDatabaseBackup(ctx, run.ID, archiveErr.Error())
		return "", archiveErr
	}
	if closeErr != nil {
		_ = os.Remove(localPath)
		h.failDatabaseBackup(ctx, run.ID, closeErr.Error())
		return "", closeErr
	}

	var sizeBytes int64
	if info, statErr := os.Stat(localPath); statErr == nil {
		sizeBytes = info.Size()
	}
	remoteKey := pgtype.Text{}
	if h.BackupService != nil && h.BackupService.Enabled() {
		if key, upErr := h.BackupService.Upload(ctx, localPath); upErr != nil {
			slog.Warn("upgrade: pre-upgrade dump S3 upload failed; keeping local copy", "database_id", formatUUID(db.ID), "error", upErr)
		} else {
			remoteKey = pgtype.Text{String: key, Valid: true}
		}
	}
	h.finaliseDatabaseBackup(ctx, run.ID, generated.UpdateDatabaseBackupParams{
		ID:         run.ID,
		FinishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Status:     "succeeded",
		LocalPath:  pgtype.Text{String: localPath, Valid: true},
		RemoteKey:  remoteKey,
		SizeBytes:  sizeBytes,
	})
	return localPath, nil
}

// rollbackUpgrade restores the database to its prior version from the pre-upgrade
// dump after a failed upgrade. Returns nil when the rollback succeeds (the
// database is healthy at the old version, so there is nothing to retry); returns
// the error only if the rollback itself fails (database left failed).
func (h *TaskHandler) rollbackUpgrade(ctx context.Context, db generated.Database, creds map[string]string, oldVersion string, hostPort int32, dumpPath, reason string) error {
	slog.Warn("upgrade failed; rolling back to previous version", "database_id", formatUUID(db.ID), "reason", reason, "old_version", oldVersion)

	dbOld, err := h.Queries.UpdateDatabaseVersion(ctx, generated.UpdateDatabaseVersionParams{ID: db.ID, Version: oldVersion})
	if err != nil {
		h.failDatabase(ctx, db.ID, fmt.Sprintf("upgrade failed (%s); rollback could not reset version: %v", reason, err))
		return errors.Join(fmt.Errorf("rollback reset version: %w", err), asynq.SkipRetry)
	}

	h.removeDBContainer(ctx, dbOld.Slug)
	if err := h.Runtime.RemoveVolume(ctx, dbOld.Slug+"-vol"); err != nil {
		slog.Warn("rollback: remove volume (may not exist)", "database_id", formatUUID(db.ID), "error", err)
	}
	if err := h.provisionDBContainer(ctx, dbOld, creds, hostPort); err != nil {
		h.failDatabase(ctx, db.ID, fmt.Sprintf("upgrade failed (%s); rollback provision failed: %v", reason, err))
		return errors.Join(fmt.Errorf("rollback provision: %w", err), asynq.SkipRetry)
	}
	if err := h.applyRestoreArchive(ctx, dbOld, creds, "logical", dumpPath); err != nil {
		h.failDatabase(ctx, db.ID, fmt.Sprintf("upgrade failed (%s); rollback restore failed: %v", reason, err))
		return errors.Join(fmt.Errorf("rollback restore: %w", err), asynq.SkipRetry)
	}

	slog.Warn("upgrade rolled back; database restored to previous version", "database_id", formatUUID(db.ID), "old_version", oldVersion)
	return nil
}

func (h *TaskHandler) setDatabaseStatus(ctx context.Context, dbID pgtype.UUID, status string) {
	if _, err := h.Queries.UpdateDatabaseStatus(ctx, generated.UpdateDatabaseStatusParams{ID: dbID, Status: status}); err != nil {
		slog.Warn("failed to set database status", "database_id", formatUUID(dbID), "status", status, "error", err)
	}
}
