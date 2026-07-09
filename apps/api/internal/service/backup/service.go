// Package backup provides S3-compatible object storage for off-host backups.
// The service wraps github.com/minio/minio-go/v7 so a single client covers
// AWS S3, Cloudflare R2, Backblaze B2, Wasabi, and self-hosted MinIO.
package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"

	"github.com/weiling79/belune/internal/config"
)

// BackupObject is a single remote backup entry returned by List.
type BackupObject struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// Service uploads and manages backup archives in an S3-compatible bucket.
// It is safe for concurrent use. The minio client is constructed lazily on
// first use so a misconfigured (or disabled) service never dials on startup.
type Service struct {
	cfg    *config.Config
	client *minio.Client // nil until init()
}

// New creates a backup service. The client is not dialled until the first
// operation; NewService never returns an error.
func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// Enabled reports whether remote backup is configured.
func (s *Service) Enabled() bool {
	return s.cfg.BackupRemoteEnabled && s.cfg.BackupS3Bucket != ""
}

// Upload uploads localPath to the configured bucket and returns the remote key.
// The remote key is derived from the filename: <prefix><basename>.
func (s *Service) Upload(ctx context.Context, localPath string) (string, error) {
	if err := s.init(); err != nil {
		return "", err
	}

	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open backup file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat backup file: %w", err)
	}

	key := s.cfg.BackupS3Prefix + filepath.Base(localPath)
	contentType := "application/gzip"
	if strings.HasSuffix(localPath, ".age") {
		contentType = "application/octet-stream"
	}

	_, err = s.client.PutObject(ctx, s.cfg.BackupS3Bucket, key, f, info.Size(),
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("upload to s3: %w", err)
	}
	return key, nil
}

// Download fetches the object at key into localPath, creating parent dirs as
// needed. Used to restore a managed-database backup from S3.
func (s *Service) Download(ctx context.Context, key, localPath string) error {
	if err := s.init(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create download dir: %w", err)
	}
	if err := s.client.FGetObject(ctx, s.cfg.BackupS3Bucket, key, localPath,
		minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("download from s3: %w", err)
	}
	return nil
}

// List returns all backup objects under the configured prefix, sorted oldest-first.
func (s *Service) List(ctx context.Context) ([]BackupObject, error) {
	if err := s.init(); err != nil {
		return nil, err
	}

	var objects []BackupObject
	for obj := range s.client.ListObjects(ctx, s.cfg.BackupS3Bucket,
		minio.ListObjectsOptions{Prefix: s.cfg.BackupS3Prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("list objects: %w", obj.Err)
		}
		objects = append(objects, BackupObject{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
		})
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.Before(objects[j].LastModified)
	})
	return objects, nil
}

// Delete removes the given keys from the bucket. Missing keys are silently ignored.
func (s *Service) Delete(ctx context.Context, keys []string) error {
	if err := s.init(); err != nil {
		return err
	}

	objectsCh := make(chan minio.ObjectInfo, len(keys))
	for _, key := range keys {
		objectsCh <- minio.ObjectInfo{Key: key}
	}
	close(objectsCh)

	var errs []string
	for result := range s.client.RemoveObjects(ctx, s.cfg.BackupS3Bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", result.ObjectName, result.Err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("delete objects: %s", strings.Join(errs, "; "))
	}
	return nil
}

// LatestKey returns the key and upload timestamp of the most-recent backup,
// or empty string and zero time when no backups exist.
func (s *Service) LatestKey(ctx context.Context) (string, time.Time, error) {
	objects, err := s.List(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	if len(objects) == 0 {
		return "", time.Time{}, nil
	}
	latest := objects[len(objects)-1]
	return latest.Key, latest.LastModified, nil
}

// EnsureBucket creates the configured bucket if it does not already exist.
// This is used in tests and can be called during initial provisioning.
// Check verifies the remote storage is reachable and the configured bucket
// exists, without mutating anything. Used by the admin "Test connection"
// action. Returns an error describing the failure (unreachable, bad
// credentials, or missing bucket).
func (s *Service) Check(ctx context.Context) error {
	if !s.Enabled() {
		return fmt.Errorf("remote backup storage is not configured")
	}
	if err := s.init(); err != nil {
		return err
	}
	exists, err := s.client.BucketExists(ctx, s.cfg.BackupS3Bucket)
	if err != nil {
		return fmt.Errorf("reach bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket %q does not exist", s.cfg.BackupS3Bucket)
	}
	return nil
}

func (s *Service) EnsureBucket(ctx context.Context) error {
	if err := s.init(); err != nil {
		return err
	}
	exists, err := s.client.BucketExists(ctx, s.cfg.BackupS3Bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.cfg.BackupS3Bucket,
		minio.MakeBucketOptions{Region: s.cfg.BackupS3Region}); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	return nil
}

// SelectForDeletion returns the keys that should be deleted under the given
// retention policy. objects must be sorted oldest-first (as returned by List).
// The retainCount newest objects are always kept regardless of age. Beyond
// that, objects whose LastModified is older than retainDays days are deleted.
func SelectForDeletion(objects []BackupObject, now time.Time, retainDays, retainCount int) []string {
	if len(objects) == 0 {
		return nil
	}

	// keepFrom is the index of the first object in the "always keep" tail.
	keepFrom := len(objects) - retainCount
	if keepFrom < 0 {
		keepFrom = 0
	}

	cutoff := now.AddDate(0, 0, -retainDays)

	var toDelete []string
	for i, obj := range objects {
		if i >= keepFrom {
			break
		}
		if obj.LastModified.Before(cutoff) {
			toDelete = append(toDelete, obj.Key)
		}
	}
	return toDelete
}

// Rotate applies the retention policy: deletes remote objects that are beyond
// BackupRetainCount AND older than BackupRetainDays. Returns the deleted keys.
func (s *Service) Rotate(ctx context.Context) ([]string, error) {
	objects, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	keys := SelectForDeletion(objects, time.Now(), s.cfg.BackupRetainDays, s.cfg.BackupRetainCount)
	if len(keys) == 0 {
		return nil, nil
	}

	if err := s.Delete(ctx, keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// init lazily constructs the minio client on first use.
func (s *Service) init() error {
	if s.client != nil {
		return nil
	}
	if !s.Enabled() {
		return fmt.Errorf("remote backup is not enabled (BACKUP_REMOTE_ENABLED=false or BACKUP_S3_BUCKET empty)")
	}

	client, err := newMinioClient(s.cfg.BackupS3Endpoint, s.cfg.BackupS3Region,
		s.cfg.BackupS3AccessKey, s.cfg.BackupS3SecretKey, s.cfg.BackupS3UseSSL)
	if err != nil {
		return fmt.Errorf("create s3 client: %w", err)
	}
	s.client = client
	return nil
}
