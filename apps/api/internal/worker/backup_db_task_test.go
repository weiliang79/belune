package worker_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/testutil"
)

// seedDatabase inserts a user, project, and a running database row with
// encrypted credentials. opts can override the create params (type, backup
// mode, data dir, etc.) before insert.
func seedDatabase(t *testing.T, opts ...func(*generated.CreateDatabaseParams)) generated.Database {
	t.Helper()
	ctx := context.Background()
	suffix := randomSuffix(t)

	user, err := testQueries.CreateUser(ctx, generated.CreateUserParams{
		Email:        "db-" + suffix + "@test.com",
		PasswordHash: "x",
		Role:         "admin",
	})
	require.NoError(t, err)
	project, err := testQueries.CreateProject(ctx, generated.CreateProjectParams{
		Name:   "DB Project",
		Slug:   "dbproj-" + suffix,
		UserID: user.ID,
	})
	require.NoError(t, err)

	keyring, err := crypto.ParseKeyringEnv("", testutil.TestEncryptionKey, "")
	require.NoError(t, err)
	credsJSON, err := json.Marshal(map[string]string{
		"user": "u", "password": "p", "database": "d", "username": "u",
	})
	require.NoError(t, err)
	enc, err := keyring.Encrypt(credsJSON)
	require.NoError(t, err)

	params := generated.CreateDatabaseParams{
		ProjectID:            project.ID,
		Type:                 "postgres",
		Name:                 "db-" + suffix,
		Slug:                 project.Slug + "-db",
		Version:              "16",
		Status:               status.DatabaseRunning,
		InternalHost:         pgtype.Text{String: project.Slug + "-db", Valid: true},
		InternalPort:         pgtype.Int4{Int32: 5432, Valid: true},
		CredentialsEncrypted: enc,
		BackupMode:           "none",
	}
	for _, fn := range opts {
		fn(&params)
	}
	db, err := testQueries.CreateDatabase(ctx, params)
	require.NoError(t, err)
	return db
}

func dbIDStr(db generated.Database) string { return uuidString(db.ID) }

func TestHandleBackupDBTask_Logical(t *testing.T) {
	ctx := context.Background()
	rt := &testutil.MockContainerRuntime{
		ExecFunc: func(_ context.Context, _ string, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
			_, _ = stdout.Write([]byte("-- SQL DUMP --"))
			return 0, nil
		},
	}
	h := newTestHandler(rt, nil)
	h.Config.DatabaseBackupDir = t.TempDir()

	db := seedDatabase(t)
	payload, _ := json.Marshal(map[string]string{"database_id": dbIDStr(db)})
	require.NoError(t, h.HandleBackupDBTask(ctx, asynq.NewTask("backup_db", payload)))

	backups, err := testQueries.ListDatabaseBackups(ctx, generated.ListDatabaseBackupsParams{DatabaseID: db.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, "succeeded", backups[0].Status)
	assert.Greater(t, backups[0].SizeBytes, int64(0))
	assert.True(t, backups[0].LocalPath.Valid)
	_, statErr := os.Stat(backups[0].LocalPath.String)
	assert.NoError(t, statErr)
}

// orderRuntime records the relative order of stop/start/tar so the cold-snapshot
// bracket (stop → tar → start) can be asserted.
type orderRuntime struct {
	*testutil.MockContainerRuntime
	order *[]string
}

func (r *orderRuntime) StopContainer(ctx context.Context, id string) error {
	*r.order = append(*r.order, "stop")
	return r.MockContainerRuntime.StopContainer(ctx, id)
}

func (r *orderRuntime) StartContainer(ctx context.Context, id string) error {
	*r.order = append(*r.order, "start")
	return r.MockContainerRuntime.StartContainer(ctx, id)
}

func TestHandleBackupDBTask_VolumeSnapshotOrdering(t *testing.T) {
	ctx := context.Background()
	var order []string
	mock := &testutil.MockContainerRuntime{
		RunHelperFunc: func(_ context.Context, _ runtime.ContainerConfig, _ io.Reader, stdout, _ io.Writer) (int, error) {
			order = append(order, "tar")
			_, _ = stdout.Write([]byte("TAR ARCHIVE"))
			return 0, nil
		},
	}
	rt := &orderRuntime{MockContainerRuntime: mock, order: &order}
	h := newTestHandler(rt, nil)
	h.Config.DatabaseBackupDir = t.TempDir()
	h.Config.DatabaseBackupHelperImage = "alpine:3.20"

	db := seedDatabase(t, func(p *generated.CreateDatabaseParams) {
		p.Type = "other"
		p.BackupMode = "volume_snapshot"
		p.Image = pgtype.Text{String: "clickhouse/clickhouse-server:24.3", Valid: true}
		p.ContainerPort = pgtype.Int4{Int32: 9000, Valid: true}
		p.DataDir = pgtype.Text{String: "/var/lib/clickhouse", Valid: true}
	})
	payload, _ := json.Marshal(map[string]string{"database_id": dbIDStr(db)})
	require.NoError(t, h.HandleBackupDBTask(ctx, asynq.NewTask("backup_db", payload)))

	// Cold snapshot must stop the DB, tar the volume, then restart it.
	assert.Equal(t, []string{"stop", "tar", "start"}, order)

	backups, err := testQueries.ListDatabaseBackups(ctx, generated.ListDatabaseBackupsParams{DatabaseID: db.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, "succeeded", backups[0].Status)
}

func TestHandleBackupDBTask_Command(t *testing.T) {
	ctx := context.Background()
	var cmds []string
	rt := &testutil.MockContainerRuntime{
		ExecFunc: func(_ context.Context, _ string, cmd []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
			cmds = append(cmds, strings.Join(cmd, " "))
			if stdout != nil {
				_, _ = stdout.Write([]byte("ARCHIVE"))
			}
			return 0, nil
		},
	}
	h := newTestHandler(rt, nil)
	h.Config.DatabaseBackupDir = t.TempDir()

	db := seedDatabase(t, func(p *generated.CreateDatabaseParams) {
		p.Type = "other"
		p.BackupMode = "command"
		p.Image = pgtype.Text{String: "myimage:1", Valid: true}
		p.ContainerPort = pgtype.Int4{Int32: 1234, Valid: true}
		p.DataDir = pgtype.Text{String: "/data", Valid: true}
		p.BackupCommand = pgtype.Text{String: "mydump", Valid: true}
		p.RestoreCommand = pgtype.Text{String: "myrestore", Valid: true}
	})
	payload, _ := json.Marshal(map[string]string{"database_id": dbIDStr(db)})
	require.NoError(t, h.HandleBackupDBTask(ctx, asynq.NewTask("backup_db", payload)))

	// Expect: prep (runs user command), tar the dir, cleanup.
	require.Len(t, cmds, 3)
	assert.Contains(t, cmds[0], "mydump")
	assert.Contains(t, cmds[0], "PAAS_BACKUP_DIR")
	assert.Contains(t, cmds[1], "tar czf -")
	assert.Contains(t, cmds[2], "rm -rf")
}

func TestHandleUpgradeDBTask_HappyPath(t *testing.T) {
	ctx := context.Background()
	// A realistic dump body so the gzipped archive clears the pre-upgrade
	// dump-validity floor (the destructive wipe is gated on a non-empty dump).
	dumpBody := []byte(strings.Repeat(
		"CREATE TABLE t (id int, name text, created timestamptz);\n"+
			"INSERT INTO t VALUES (1, 'alpha', now()), (2, 'beta', now());\n", 40))
	rt := &testutil.MockContainerRuntime{
		ExecFunc: func(_ context.Context, _ string, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
			if stdout != nil {
				_, _ = stdout.Write(dumpBody)
			}
			return 0, nil
		},
	}
	h := newTestHandler(rt, nil)
	h.Config.DatabaseBackupDir = t.TempDir()

	db := seedDatabase(t)
	payload, _ := json.Marshal(map[string]string{"database_id": dbIDStr(db), "target_version": "17"})
	require.NoError(t, h.HandleUpgradeDBTask(ctx, asynq.NewTask("upgrade_db", payload)))

	updated, err := testQueries.GetDatabase(ctx, db.ID)
	require.NoError(t, err)
	assert.Equal(t, "17", updated.Version)
	assert.Equal(t, status.DatabaseRunning, updated.Status)

	// The pre-upgrade dump is recorded as a succeeded backup (rollback artifact).
	backups, err := testQueries.ListDatabaseBackups(ctx, generated.ListDatabaseBackupsParams{DatabaseID: db.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, "succeeded", backups[0].Status)
}
