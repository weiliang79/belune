package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Destination is the decrypted, transport-level description of a backup
// target. It is deliberately scope-agnostic: a project-owned destination and
// any future admin-owned target build the exact same client, so the upload/
// list/delete plumbing is shared.
//
// Provider == "local" is the one exception to "transport-level": it isn't a
// transport at all — Endpoint/Bucket/AccessKey/SecretKey are meaningless and
// NewDestinationClient must never be called for it. Callers check
// Provider == "local" BEFORE building a client and skip straight to keeping
// the staged archive on-host. Kept on this struct (rather than a separate
// lookup) so a single Resolve() call gives callers everything they need to
// branch correctly.
type Destination struct {
	Provider  string // "s3", "r2", "b2", "wasabi", "minio", "other", or "local"
	Endpoint  string // empty = AWS regional endpoint derived from Region
	Region    string
	Bucket    string
	Prefix    string // base key prefix prepended to every object key
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// IsLocal reports whether this destination means "keep the archive on-host,
// don't upload anywhere" — the local equivalent of an S3-compatible target.
func (d Destination) IsLocal() bool { return d.Provider == "local" }

// DestinationClient is a minio client bound to a single destination's bucket.
type DestinationClient struct {
	client *minio.Client
	bucket string
}

// newMinioClient builds an S3-compatible client. An empty endpoint resolves to
// the AWS regional endpoint. Shared by the global env-var Service and per-
// destination clients.
func newMinioClient(endpoint, region, accessKey, secretKey string, useSSL bool) (*minio.Client, error) {
	if endpoint == "" {
		endpoint = fmt.Sprintf("s3.%s.amazonaws.com", region)
	}
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
}

// NewDestinationClient dials an S3-compatible client for one destination.
func NewDestinationClient(d Destination) (*DestinationClient, error) {
	client, err := newMinioClient(d.Endpoint, d.Region, d.AccessKey, d.SecretKey, d.UseSSL)
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}
	return &DestinationClient{client: client, bucket: d.Bucket}, nil
}

// BuildKey joins a user-supplied prefix and a filename into an S3 object key.
// Leading/trailing slashes on the prefix are trimmed (S3 keys are not absolute);
// an empty prefix yields just the name.
func BuildKey(prefix, name string) string {
	p := strings.Trim(prefix, "/")
	if p == "" {
		return name
	}
	return p + "/" + name
}

// Test verifies connectivity and that the configured bucket is reachable.
func (c *DestinationClient) Test(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket %q does not exist or is not accessible", c.bucket)
	}
	return nil
}

// UploadTo uploads localPath to the destination bucket under key and returns key.
func (c *DestinationClient) UploadTo(ctx context.Context, localPath, key string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open backup file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat backup file: %w", err)
	}

	contentType := "application/gzip"
	if strings.HasSuffix(localPath, ".age") {
		contentType = "application/octet-stream"
	}

	if _, err := c.client.PutObject(ctx, c.bucket, key, f, info.Size(),
		minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return "", fmt.Errorf("upload to s3: %w", err)
	}
	return key, nil
}

// Download fetches key into localPath, creating parent dirs as needed.
func (c *DestinationClient) Download(ctx context.Context, key, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create download dir: %w", err)
	}
	if err := c.client.FGetObject(ctx, c.bucket, key, localPath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("download from s3: %w", err)
	}
	return nil
}

// DeleteFrom removes keys from the destination bucket. Missing keys are ignored.
func (c *DestinationClient) DeleteFrom(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	objectsCh := make(chan minio.ObjectInfo, len(keys))
	for _, key := range keys {
		objectsCh <- minio.ObjectInfo{Key: key}
	}
	close(objectsCh)

	var errs []string
	for result := range c.client.RemoveObjects(ctx, c.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", result.ObjectName, result.Err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("delete objects: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ListUnder returns all objects under prefix, sorted oldest-first.
func (c *DestinationClient) ListUnder(ctx context.Context, prefix string) ([]BackupObject, error) {
	var objects []BackupObject
	for obj := range c.client.ListObjects(ctx, c.bucket,
		minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
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
