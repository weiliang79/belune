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

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// paasBackupDir is the in-container scratch dir for "command" backup mode. The
// user's backup command writes here (exposed as $PAAS_BACKUP_DIR); the system
// then tars it out. Under /tmp so it is writable by non-root container users.
const paasBackupDir = "/tmp/paas-backup"

// backupDBPayload triggers a backup of a managed database.
type backupDBPayload struct {
	DatabaseID string `json:"database_id"`
}

// restoreDBPayload restores a managed database from a previously-recorded backup.
type restoreDBPayload struct {
	DatabaseID string `json:"database_id"`
	BackupID   string `json:"backup_id"`
}

// dumpSpec is the per-engine logical dump/restore command pair (known engines).
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
		return dumpSpec{ok: false}
	}
}

// dbBackupMethod selects how a database is backed up:
//   - "logical": known engine, online dump (pg_dump/mysqldump/mongodump)
//   - "volume_snapshot": "other" — cold tar of the data-dir volume
//   - "command": "other" — user-supplied backup/restore commands
//   - "none": unsupported (redis, or "other" with backup disabled)
func dbBackupMethod(db generated.Database) string {
	switch db.Type {
	case "postgres", "mysql", "mongo":
		return "logical"
	case "other":
		switch db.BackupMode {
		case "volume_snapshot", "command":
			return db.BackupMode
		}
	}
	return "none"
}

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

	method := dbBackupMethod(db)
	if method == "none" {
		return errors.Join(fmt.Errorf("backup is not supported for database type %s (permanent)", db.Type), asynq.SkipRetry)
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

	if err := os.MkdirAll(h.Config.DatabaseBackupDir, 0o755); err != nil {
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("create backup dir: %v", err))
		return fmt.Errorf("create backup dir: %w", err)
	}

	fileName := fmt.Sprintf("%s-%s.backup.gz", db.Slug, time.Now().UTC().Format("20060102T150405Z"))
	localPath := filepath.Join(h.Config.DatabaseBackupDir, fileName)

	f, err := os.Create(localPath)
	if err != nil {
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("create backup file: %v", err))
		return fmt.Errorf("create backup file: %w", err)
	}

	archiveErr := h.writeBackupArchive(ctx, db, creds, method, f)
	closeErr := f.Close()
	if archiveErr != nil {
		_ = os.Remove(localPath)
		h.failDatabaseBackup(ctx, run.ID, archiveErr.Error())
		return archiveErr
	}
	if closeErr != nil {
		_ = os.Remove(localPath)
		h.failDatabaseBackup(ctx, run.ID, fmt.Sprintf("finalise backup file: %v", closeErr))
		return fmt.Errorf("finalise backup file: %w", closeErr)
	}

	var sizeBytes int64
	if info, statErr := os.Stat(localPath); statErr == nil {
		sizeBytes = info.Size()
	}

	// Off-host copy is best-effort: a local archive is still a valid backup.
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

	slog.Info("database backed up", "database_id", payload.DatabaseID, "method", method, "size_bytes", sizeBytes, "remote", remoteKey.Valid)
	return nil
}

// writeBackupArchive produces the gzipped backup archive into f according to the
// chosen method. Permanent failures are wrapped with asynq.SkipRetry.
func (h *TaskHandler) writeBackupArchive(ctx context.Context, db generated.Database, creds map[string]string, method string, f *os.File) error {
	switch method {
	case "logical":
		spec := dbDumpSpec(db.Type, creds)
		gz := gzip.NewWriter(f)
		var stderr bytes.Buffer
		exit, execErr := h.Runtime.ContainerExec(ctx, db.Slug, spec.dump, nil, gz, &stderr)
		gzErr := gz.Close()
		if execErr != nil {
			return fmt.Errorf("exec dump: %w", execErr)
		}
		if exit != 0 {
			return errors.Join(fmt.Errorf("dump exited %d: %s", exit, strings.TrimSpace(stderr.String())), asynq.SkipRetry)
		}
		if gzErr != nil {
			return fmt.Errorf("flush dump gzip: %w", gzErr)
		}
		return nil
	case "volume_snapshot":
		return h.snapshotVolume(ctx, db, f)
	case "command":
		return h.commandBackup(ctx, db, f)
	}
	return errors.Join(fmt.Errorf("unknown backup method %q", method), asynq.SkipRetry)
}

// snapshotVolume cold-tars an "other" database's data-dir volume into f. The
// database container is stopped for a consistent snapshot and always restarted.
func (h *TaskHandler) snapshotVolume(ctx context.Context, db generated.Database, f *os.File) error {
	dataDir := db.DataDir.String
	if dataDir == "" {
		return errors.Join(errors.New("data_dir is not set (permanent)"), asynq.SkipRetry)
	}
	helperImage := h.Config.DatabaseBackupHelperImage
	if err := h.Runtime.PullImage(ctx, helperImage); err != nil {
		return fmt.Errorf("pull helper image: %w", err)
	}

	// Cold snapshot: stop the database so files are not mutated mid-tar.
	if err := h.Runtime.StopContainer(ctx, db.Slug); err != nil {
		slog.Warn("snapshot: stop database (may already be stopped)", "database_id", formatUUID(db.ID), "error", err)
	}
	defer func() {
		if err := h.Runtime.StartContainer(ctx, db.Slug); err != nil {
			slog.Error("snapshot: failed to restart database after backup", "database_id", formatUUID(db.ID), "error", err)
		}
	}()

	var stderr bytes.Buffer
	exit, err := h.Runtime.RunHelper(ctx, runtime.ContainerConfig{
		Image:   helperImage,
		Cmd:     []string{"tar", "czf", "-", "-C", dataDir, "."},
		Volumes: map[string]string{db.Slug + "-vol": dataDir},
	}, nil, f, &stderr)
	if err != nil {
		return fmt.Errorf("snapshot helper: %w", err)
	}
	if exit != 0 {
		return errors.Join(fmt.Errorf("snapshot tar exited %d: %s", exit, strings.TrimSpace(stderr.String())), asynq.SkipRetry)
	}
	return nil
}

// commandBackup runs the user's backup command in the running container (writing
// into $PAAS_BACKUP_DIR), then tars that directory into f.
func (h *TaskHandler) commandBackup(ctx context.Context, db generated.Database, f *os.File) error {
	if !db.BackupCommand.Valid || db.BackupCommand.String == "" {
		return errors.Join(errors.New("backup_command is not set (permanent)"), asynq.SkipRetry)
	}

	prep := fmt.Sprintf("rm -rf %s && mkdir -p %s && export PAAS_BACKUP_DIR=%s && ( %s )",
		paasBackupDir, paasBackupDir, paasBackupDir, db.BackupCommand.String)
	var stderr bytes.Buffer
	exit, err := h.Runtime.ContainerExec(ctx, db.Slug, []string{"sh", "-c", prep}, nil, nil, &stderr)
	if err != nil {
		return fmt.Errorf("exec backup command: %w", err)
	}
	if exit != 0 {
		return errors.Join(fmt.Errorf("backup command exited %d: %s", exit, strings.TrimSpace(stderr.String())), asynq.SkipRetry)
	}

	var terr bytes.Buffer
	exit, err = h.Runtime.ContainerExec(ctx, db.Slug,
		[]string{"sh", "-c", fmt.Sprintf("tar czf - -C %s .", paasBackupDir)}, nil, f, &terr)
	if err != nil {
		return fmt.Errorf("exec tar: %w", err)
	}
	if exit != 0 {
		return errors.Join(fmt.Errorf("tar exited %d: %s", exit, strings.TrimSpace(terr.String())), asynq.SkipRetry)
	}

	// Best-effort cleanup.
	_, _ = h.Runtime.ContainerExec(ctx, db.Slug, []string{"sh", "-c", "rm -rf " + paasBackupDir}, nil, nil, nil)
	return nil
}

// HandleRestoreDBTask restores a managed database from a recorded backup using
// the method that produced it. Local file is preferred; otherwise downloaded
// from S3.
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

	method := dbBackupMethod(db)
	if method == "none" {
		return errors.Join(fmt.Errorf("restore not supported for database type %s (permanent)", db.Type), asynq.SkipRetry)
	}

	creds, err := h.decryptDBCredentials(db)
	if err != nil {
		return errors.Join(fmt.Errorf("credentials (permanent): %w", err), asynq.SkipRetry)
	}

	dumpPath, cleanup, err := h.resolveBackupFile(ctx, backup)
	if err != nil {
		return fmt.Errorf("resolve backup file: %w", err)
	}
	defer cleanup()

	if err := h.applyRestoreArchive(ctx, db, creds, method, dumpPath); err != nil {
		return err
	}

	slog.Info("database restored", "database_id", payload.DatabaseID, "backup_id", payload.BackupID, "method", method)
	return nil
}

func (h *TaskHandler) applyRestoreArchive(ctx context.Context, db generated.Database, creds map[string]string, method, dumpPath string) error {
	switch method {
	case "logical":
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

		spec := dbDumpSpec(db.Type, creds)
		var stderr bytes.Buffer
		exit, execErr := h.Runtime.ContainerExec(ctx, db.Slug, spec.restore, gz, nil, &stderr)
		if execErr != nil {
			return fmt.Errorf("exec restore: %w", execErr)
		}
		if exit != 0 {
			return errors.Join(fmt.Errorf("restore exited %d: %s", exit, strings.TrimSpace(stderr.String())), asynq.SkipRetry)
		}
		return nil
	case "volume_snapshot":
		return h.restoreVolume(ctx, db, dumpPath)
	case "command":
		return h.commandRestore(ctx, db, dumpPath)
	}
	return errors.Join(fmt.Errorf("unknown restore method %q", method), asynq.SkipRetry)
}

// restoreVolume wipes the data-dir volume and untars the snapshot into it. The
// database is stopped for the swap and always restarted.
func (h *TaskHandler) restoreVolume(ctx context.Context, db generated.Database, dumpPath string) error {
	dataDir := db.DataDir.String
	if dataDir == "" {
		return errors.Join(errors.New("data_dir is not set (permanent)"), asynq.SkipRetry)
	}
	helperImage := h.Config.DatabaseBackupHelperImage
	if err := h.Runtime.PullImage(ctx, helperImage); err != nil {
		return fmt.Errorf("pull helper image: %w", err)
	}

	f, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("open snapshot file: %w", err)
	}
	defer f.Close()

	if err := h.Runtime.StopContainer(ctx, db.Slug); err != nil {
		slog.Warn("restore: stop database (may already be stopped)", "database_id", formatUUID(db.ID), "error", err)
	}
	defer func() {
		if err := h.Runtime.StartContainer(ctx, db.Slug); err != nil {
			slog.Error("restore: failed to restart database after restore", "database_id", formatUUID(db.ID), "error", err)
		}
	}()

	script := fmt.Sprintf("find %s -mindepth 1 -delete && tar xzf - -C %s", dataDir, dataDir)
	var stderr bytes.Buffer
	exit, err := h.Runtime.RunHelper(ctx, runtime.ContainerConfig{
		Image:   helperImage,
		Cmd:     []string{"sh", "-c", script},
		Volumes: map[string]string{db.Slug + "-vol": dataDir},
	}, f, nil, &stderr)
	if err != nil {
		return fmt.Errorf("restore helper: %w", err)
	}
	if exit != 0 {
		return errors.Join(fmt.Errorf("restore tar exited %d: %s", exit, strings.TrimSpace(stderr.String())), asynq.SkipRetry)
	}
	return nil
}

// commandRestore untars the archive into $PAAS_BACKUP_DIR in the running
// container, then runs the user's restore command.
func (h *TaskHandler) commandRestore(ctx context.Context, db generated.Database, dumpPath string) error {
	if !db.RestoreCommand.Valid || db.RestoreCommand.String == "" {
		return errors.Join(errors.New("restore_command is not set (permanent)"), asynq.SkipRetry)
	}

	f, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("open backup file: %w", err)
	}
	defer f.Close()

	unpack := fmt.Sprintf("rm -rf %s && mkdir -p %s && tar xzf - -C %s", paasBackupDir, paasBackupDir, paasBackupDir)
	var uerr bytes.Buffer
	exit, err := h.Runtime.ContainerExec(ctx, db.Slug, []string{"sh", "-c", unpack}, f, nil, &uerr)
	if err != nil {
		return fmt.Errorf("exec unpack: %w", err)
	}
	if exit != 0 {
		return errors.Join(fmt.Errorf("unpack exited %d: %s", exit, strings.TrimSpace(uerr.String())), asynq.SkipRetry)
	}

	run := fmt.Sprintf("export PAAS_BACKUP_DIR=%s && ( %s )", paasBackupDir, db.RestoreCommand.String)
	var rerr bytes.Buffer
	exit, err = h.Runtime.ContainerExec(ctx, db.Slug, []string{"sh", "-c", run}, nil, nil, &rerr)
	if err != nil {
		return fmt.Errorf("exec restore command: %w", err)
	}
	if exit != 0 {
		return errors.Join(fmt.Errorf("restore command exited %d: %s", exit, strings.TrimSpace(rerr.String())), asynq.SkipRetry)
	}

	_, _ = h.Runtime.ContainerExec(ctx, db.Slug, []string{"sh", "-c", "rm -rf " + paasBackupDir}, nil, nil, nil)
	return nil
}

// resolveBackupFile returns a readable path to the backup archive. If the local
// copy is missing it downloads the remote object to a temp file; the returned
// cleanup removes any temp file (no-op for the local copy).
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
