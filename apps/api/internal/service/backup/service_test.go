package backup_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/service/backup"
)

// startMinio spins up a MinIO testcontainer and returns a configured Service.
func startMinio(t *testing.T) (*backup.Service, func()) {
	t.Helper()
	ctx := context.Background()

	const accessKey = "minioadmin"
	const secretKey = "minioadmin"
	const bucket = "test-backups"

	container, err := tcminio.Run(ctx, "minio/minio:latest",
		tcminio.WithUsername(accessKey),
		tcminio.WithPassword(secretKey),
	)
	require.NoError(t, err, "start minio container")

	endpoint, err := container.ConnectionString(ctx)
	require.NoError(t, err)
	// endpoint includes "http://" prefix — strip it for minio-go
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	cfg := &config.Config{
		BackupRemoteEnabled: true,
		BackupS3Endpoint:    endpoint,
		BackupS3Region:      "us-east-1",
		BackupS3Bucket:      bucket,
		BackupS3AccessKey:   accessKey,
		BackupS3SecretKey:   secretKey,
		BackupS3Prefix:      "paas/",
		BackupS3UseSSL:      false,
		BackupRetainDays:    30,
		BackupRetainCount:   14,
	}

	svc := backup.New(cfg)

	// Create the bucket — MinIO starts empty.
	require.NoError(t, svc.EnsureBucket(ctx))

	teardown := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("minio container terminate: %v", err)
		}
	}
	return svc, teardown
}

func tempBackupFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestBackupService_UploadListDelete(t *testing.T) {
	svc, teardown := startMinio(t)
	defer teardown()
	ctx := context.Background()

	path := tempBackupFile(t, "paas-backup-20260501T120000Z.tar.gz", "fake archive content")

	// Upload
	key, err := svc.Upload(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, "paas/paas-backup-20260501T120000Z.tar.gz", key)

	// List — should contain the one object
	objects, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, objects, 1)
	assert.Equal(t, key, objects[0].Key)
	assert.Equal(t, int64(len("fake archive content")), objects[0].Size)

	// Delete
	require.NoError(t, svc.Delete(ctx, []string{key}))

	// List again — should be empty
	objects, err = svc.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, objects)
}

func TestBackupService_LatestKey(t *testing.T) {
	svc, teardown := startMinio(t)
	defer teardown()
	ctx := context.Background()

	// No objects yet
	key, ts, err := svc.LatestKey(ctx)
	require.NoError(t, err)
	assert.Empty(t, key)
	assert.True(t, ts.IsZero())

	// Upload two files
	for i, name := range []string{
		"paas-backup-20260501T100000Z.tar.gz",
		"paas-backup-20260501T120000Z.tar.gz",
	} {
		path := tempBackupFile(t, name, fmt.Sprintf("content %d", i))
		_, err := svc.Upload(ctx, path)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // ensure distinct LastModified
	}

	key, ts, err = svc.LatestKey(ctx)
	require.NoError(t, err)
	assert.Contains(t, key, "20260501T120000Z")
	assert.False(t, ts.IsZero())
}

func TestBackupService_AgeFileContentType(t *testing.T) {
	svc, teardown := startMinio(t)
	defer teardown()
	ctx := context.Background()

	path := tempBackupFile(t, "paas-backup-20260501T120000Z.tar.gz.age", "encrypted")
	key, err := svc.Upload(ctx, path)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(key, ".age"))
}

func TestBackupService_Disabled(t *testing.T) {
	cfg := &config.Config{BackupRemoteEnabled: false}
	svc := backup.New(cfg)

	_, err := svc.Upload(context.Background(), "/tmp/fake.tar.gz")
	assert.ErrorContains(t, err, "not enabled")

	assert.False(t, svc.Enabled())
}

func TestBackupService_DeleteMissingKeyIsNoError(t *testing.T) {
	svc, teardown := startMinio(t)
	defer teardown()
	ctx := context.Background()

	// Deleting a key that doesn't exist should not error (S3 semantics)
	err := svc.Delete(ctx, []string{"paas/nonexistent.tar.gz"})
	assert.NoError(t, err)
}

func TestSelectForDeletion(t *testing.T) {
	now := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)

	// Objects sorted oldest-first (as List returns them).
	objects := []backup.BackupObject{
		{Key: "backup-1", LastModified: now.AddDate(0, 0, -60)}, // 60 days old
		{Key: "backup-2", LastModified: now.AddDate(0, 0, -45)}, // 45 days old
		{Key: "backup-3", LastModified: now.AddDate(0, 0, -20)}, // 20 days old, within retainDays
		{Key: "backup-4", LastModified: now.AddDate(0, 0, -10)}, // always-keep (count)
		{Key: "backup-5", LastModified: now.AddDate(0, 0, -1)},  // always-keep (count)
	}

	t.Run("deletes old beyond count", func(t *testing.T) {
		// retainDays=30, retainCount=2 → always keep backup-4, backup-5
		// backup-1 (60d) and backup-2 (45d) are older than 30d → delete
		// backup-3 (20d) is within 30d → keep
		keys := backup.SelectForDeletion(objects, now, 30, 2)
		assert.Equal(t, []string{"backup-1", "backup-2"}, keys)
	})

	t.Run("retainCount protects recent objects regardless of age", func(t *testing.T) {
		// retainCount=5 → all 5 objects always kept
		keys := backup.SelectForDeletion(objects, now, 1, 5)
		assert.Empty(t, keys)
	})

	t.Run("retainDays=0 equivalent to immediate deletion", func(t *testing.T) {
		// retainDays=0 → all objects older than now deleted (all 3 candidates)
		keys := backup.SelectForDeletion(objects, now, 0, 2)
		assert.Equal(t, []string{"backup-1", "backup-2", "backup-3"}, keys)
	})

	t.Run("empty list returns nil", func(t *testing.T) {
		keys := backup.SelectForDeletion(nil, now, 30, 3)
		assert.Nil(t, keys)
	})

	t.Run("retainCount larger than list keeps everything", func(t *testing.T) {
		keys := backup.SelectForDeletion(objects[:2], now, 1, 10)
		assert.Empty(t, keys)
	})
}
