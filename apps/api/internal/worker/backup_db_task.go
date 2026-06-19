package worker

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// backupDBPayload triggers a logical-dump backup of a managed database.
type backupDBPayload struct {
	DatabaseID string `json:"database_id"`
}

// restoreDBPayload restores a managed database from a previously-recorded backup.
type restoreDBPayload struct {
	DatabaseID string `json:"database_id"`
	BackupID   string `json:"backup_id"`
}

// dumpSpec is the per-engine logical dump/restore command pair. Each command is
// run via `sh -c` inside the database's own container, using client tools that
// ship in the engine image (pg_dump/psql, mysqldump/mysql, mongodump/mongorestore).
// The dump streams to stdout; the restore reads from stdin. ok is false for
// engines without a logical-dump tool (redis — backed up via volume snapshot in
// a later version).
type dumpSpec struct {
	dump    []string
	restore []string
	ok      bool
}

// shArg single-quotes a value for safe inclusion in a `sh -c` command line.
func shArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func dbDumpSpec(dbType string, creds map[string]string) dumpSpec {
	switch dbType {
	case "postgres":
		user := shArg(creds["user"])
		db := shArg(creds["database"])
		pw := shArg(creds["password"])
		return dumpSpec{
			dump:    []string{"sh", "-c", fmt.Sprintf("PGPASSWORD=%s pg_dump -U %s -d %s --clean --if-exists --no-owner", pw, user, db)},
			restore: []string{"sh", "-c", fmt.Sprintf("PGPASSWORD=%s psql -U %s -d %s -v ON_ERROR_STOP=1", pw, user, db)},
			ok:      true,
		}
	case "mysql":
		user := shArg(creds["user"])
		db := shArg(creds["database"])
		pw := shArg(creds["password"])
		return dumpSpec{
			dump:    []string{"sh", "-c", fmt.Sprintf("MYSQL_PWD=%s mysqldump --single-transaction --routines --triggers -u %s %s", pw, user, db)},
			restore: []string{"sh", "-c", fmt.Sprintf("MYSQL_PWD=%s mysql -u %s %s", pw, user, db)},
			ok:      true,
		}
	case "mongo":
		user := shArg(creds["username"])
		pw := shArg(creds["password"])
		return dumpSpec{
			dump:    []string{"sh", "-c", fmt.Sprintf("mongodump --username %s --password %s --authenticationDatabase admin --archive", user, pw)},
			restore: []string{"sh", "-c", fmt.Sprintf("mongorestore --username %s --password %s --authenticationDatabase admin --archive --drop", user, pw)},
			ok:      true,
		}
	default:
		// redis (cache) and any unknown engine: no logical-dump tool.
		return dumpSpec{ok: false}
	}
}

// HandleBackupDBTask runs an online logical dump of a managed database, gzips it
// to the local backup directory, and (when remote backup is enabled) uploads it
// to S3. The result is recorded in database_backups.
func (h *TaskHandler) HandleBackupDBTask(ctx context.Context, t *asynq.Task) error {
	var payload backupDBPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal backup_db payload: %w", err)
	}

	slog.Info("handling backup_db task", "database_id", payload.DatabaseID)

	dbID, err := parseUUID(payload.DatabaseID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid database_id (permanent): %w", err), asynq.SkipRetry)
	}

	db, err := h.Queries.GetDatabase(ctx, dbID)
	if err != nil {
		return fmt.Errorf("get database: %w", err)
	}

	// Record the run up front so the UI shows a "running" backup immediately.
	run, err := h.Queries.InsertDatabaseBackup(ctx, dbID)
	if err != nil {
		return fmt.Errorf("insert backup run: %w", err)
	}

	creds, err := h.decryptDBCredentials(db)
	if err != nil {
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("credentials: %v", err))
		return errors.Join(fmt.Errorf("credentials (permanent): %w", err), asynq.SkipRetry)
	}

	spec := dbDumpSpec(db.Type, creds)
	if !spec.ok {
		msg := fmt.Sprintf("logical backup is not supported for %s", db.Type)
		h.failDatabaseBackup(ctx, run.ID, msg)
		return errors.Join(errors.New(msg), asynq.SkipRetry)
	}

	if err := os.MkdirAll(h.Config.DatabaseBackupDir, 0o755); err != nil {
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("create backup dir: %v", err))
		return fmt.Errorf("create backup dir: %w", err)
	}

	fileName := fmt.Sprintf("%s-%s.dump.gz", db.Slug, time.Now().UTC().Format("20060102T150405Z"))
	localPath := filepath.Join(h.Config.DatabaseBackupDir, fileName)

	f, err := os.Create(localPath)
	if err != nil {
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("create dump file: %v", err))
		return fmt.Errorf("create dump file: %w", err)
	}
	gz := gzip.NewWriter(f)

	var stderr bytes.Buffer
	exit, execErr := h.Runtime.ContainerExec(ctx, db.Slug, spec.dump, nil, gz, &stderr)
	// Close the gzip + file before any error handling so the bytes are flushed.
	gzErr := gz.Close()
	closeErr := f.Close()

	if execErr != nil {
		_ = os.Remove(localPath)
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("exec dump: %v", execErr))
		return fmt.Errorf("exec dump: %w", execErr)
	}
	if exit != 0 {
		_ = os.Remove(localPath)
		msg := fmt.Sprintf("dump exited %d: %s", exit, strings.TrimSpace(stderr.String()))
		h.failDatabaseBackup(ctx, run.ID, msg)
		return errors.Join(errors.New(msg), asynq.SkipRetry)
	}
	if gzErr != nil || closeErr != nil {
		_ = os.Remove(localPath)
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("finalise dump file: %v / %v", gzErr, closeErr))
		return fmt.Errorf("finalise dump file: %v / %v", gzErr, closeErr)
	}

	var sizeBytes int64
	if info, statErr := os.Stat(localPath); statErr == nil {
		sizeBytes = info.Size()
	}

	// Off-host copy is best-effort: a local dump is still a valid backup.
	remoteKey := pgtype.Text{}
	if h.BackupService != nil && h.BackupService.Enabled() {
		if key, upErr := h.BackupService.Upload(ctx, localPath); upErr != nil {
			slog.Warn("backup_db: S3 upload failed; keeping local copy", "database_id", payload.DatabaseID, "error", upErr)
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

	slog.Info("database backed up", "database_id", payload.DatabaseID, "size_bytes", sizeBytes, "remote", remoteKey.Valid)
	return nil
}

// HandleRestoreDBTask restores a managed database from a recorded backup. It
// prefers the local dump file and falls back to downloading from S3. The dump
// is streamed (gunzipped) into the engine's restore client. The database stays
// online throughout (logical restore).
func (h *TaskHandler) HandleRestoreDBTask(ctx context.Context, t *asynq.Task) error {
	var payload restoreDBPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal restore_db payload: %w", err)
	}

	slog.Info("handling restore_db task", "database_id", payload.DatabaseID, "backup_id", payload.BackupID)

	dbID, err := parseUUID(payload.DatabaseID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid database_id (permanent): %w", err), asynq.SkipRetry)
	}
	backupID, err := parseUUID(payload.BackupID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid backup_id (permanent): %w", err), asynq.SkipRetry)
	}

	db, err := h.Queries.GetDatabase(ctx, dbID)
	if err != nil {
		return fmt.Errorf("get database: %w", err)
	}
	backup, err := h.Queries.GetDatabaseBackup(ctx, backupID)
	if err != nil {
		return fmt.Errorf("get backup: %w", err)
	}
	if backup.DatabaseID != db.ID {
		return errors.Join(errors.New("backup does not belong to database (permanent)"), asynq.SkipRetry)
	}

	creds, err := h.decryptDBCredentials(db)
	if err != nil {
		return errors.Join(fmt.Errorf("credentials (permanent): %w", err), asynq.SkipRetry)
	}
	spec := dbDumpSpec(db.Type, creds)
	if !spec.ok {
		return errors.Join(fmt.Errorf("restore not supported for %s (permanent)", db.Type), asynq.SkipRetry)
	}

	// Resolve the dump file: local first, then S3.
	dumpPath, cleanup, err := h.resolveBackupFile(ctx, backup)
	if err != nil {
		return fmt.Errorf("resolve backup file: %w", err)
	}
	defer cleanup()

	f, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("open dump file: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip reader: %w", err)
	}
	defer gz.Close()

	var stderr bytes.Buffer
	exit, execErr := h.Runtime.ContainerExec(ctx, db.Slug, spec.restore, gz, nil, &stderr)
	if execErr != nil {
		return fmt.Errorf("exec restore: %w", execErr)
	}
	if exit != 0 {
		return errors.Join(fmt.Errorf("restore exited %d: %s", exit, strings.TrimSpace(stderr.String())), asynq.SkipRetry)
	}

	slog.Info("database restored", "database_id", payload.DatabaseID, "backup_id", payload.BackupID)
	return nil
}

// resolveBackupFile returns a readable path to the backup's gzipped dump. If the
// local copy is missing it downloads the remote object to a temp file; the
// returned cleanup removes any temp file (no-op for the local copy).
func (h *TaskHandler) resolveBackupFile(ctx context.Context, backup generated.DatabaseBackup) (string, func(), error) {
	noop := func() {}
	if backup.LocalPath.Valid {
		if _, err := os.Stat(backup.LocalPath.String); err == nil {
			return backup.LocalPath.String, noop, nil
		}
	}
	if !backup.RemoteKey.Valid {
		return "", noop, errors.New("backup has no local file and no remote copy")
	}
	if h.BackupService == nil || !h.BackupService.Enabled() {
		return "", noop, errors.New("backup is remote-only but S3 is not configured")
	}
	tmp := filepath.Join(os.TempDir(), "paas-restore-"+filepath.Base(backup.RemoteKey.String))
	if err := h.BackupService.Download(ctx, backup.RemoteKey.String, tmp); err != nil {
		return "", noop, err
	}
	return tmp, func() { _ = os.Remove(tmp) }, nil
}

func (h *TaskHandler) failDatabaseBackup(ctx context.Context, id pgtype.UUID, errMsg string) {
	slog.Error("database backup failed", "backup_id", formatUUID(id), "error", errMsg)
	h.finaliseDatabaseBackup(ctx, id, generated.UpdateDatabaseBackupParams{
		ID:         id,
		FinishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Status:     "failed",
		Error:      pgtype.Text{String: errMsg, Valid: true},
	})
}

func (h *TaskHandler) finaliseDatabaseBackup(ctx context.Context, id pgtype.UUID, params generated.UpdateDatabaseBackupParams) {
	if !id.Valid {
		return
	}
	if err := h.Queries.UpdateDatabaseBackup(ctx, params); err != nil {
		slog.Warn("backup_db: failed to update run record", "backup_id", formatUUID(id), "error", err)
	}
}
