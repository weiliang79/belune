package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/pkg/crypto"
	"github.com/weiliang79/belune/internal/service/backup"
	"github.com/weiliang79/belune/internal/store/generated"
)

// BackupDestinationService manages project-scoped S3-compatible backup targets.
// Credentials are stored as a keyring-encrypted JSON blob; the transport client
// is built lazily via the scope-agnostic backup.NewDestinationClient.
type BackupDestinationService struct {
	queries *generated.Queries
	keyring *crypto.Keyring
}

func NewBackupDestinationService(queries *generated.Queries, keyring *crypto.Keyring) *BackupDestinationService {
	return &BackupDestinationService{queries: queries, keyring: keyring}
}

// DestinationCredentials is the secret material for an S3 destination.
type DestinationCredentials struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// SaveBackupDestinationParams holds the fields for creating/updating a
// destination. Credentials is optional on update: nil preserves the stored secret.
type SaveBackupDestinationParams struct {
	ProjectID   pgtype.UUID
	Name        string
	Provider    string
	Endpoint    string
	Region      string
	Bucket      string
	Prefix      string
	UseSSL      bool
	Credentials *DestinationCredentials
}

// Create inserts a new destination, encrypting its credentials. A local
// destination has nothing to encrypt — credentials_encrypted is NOT NULL, so
// it stores an empty (not nil) blob rather than relaxing the column.
func (s *BackupDestinationService) Create(ctx context.Context, p SaveBackupDestinationParams) (generated.BackupDestination, error) {
	var enc []byte
	if p.Provider == "local" {
		enc = []byte{}
	} else {
		if p.Credentials == nil {
			return generated.BackupDestination{}, fmt.Errorf("credentials are required")
		}
		var err error
		if enc, err = s.encryptCredentials(p.Credentials); err != nil {
			return generated.BackupDestination{}, err
		}
	}
	return s.queries.CreateBackupDestination(ctx, generated.CreateBackupDestinationParams{
		ProjectID:            p.ProjectID,
		Name:                 p.Name,
		Provider:             p.Provider,
		Endpoint:             p.Endpoint,
		Region:               p.Region,
		Bucket:               p.Bucket,
		Prefix:               p.Prefix,
		UseSsl:               p.UseSSL,
		CredentialsEncrypted: enc,
	})
}

// Update mutates an existing destination. When Credentials is nil the stored
// secret is preserved (COALESCE in the query handles the NULL).
func (s *BackupDestinationService) Update(ctx context.Context, id pgtype.UUID, p SaveBackupDestinationParams) (generated.BackupDestination, error) {
	var enc []byte
	if p.Credentials != nil {
		var err error
		if enc, err = s.encryptCredentials(p.Credentials); err != nil {
			return generated.BackupDestination{}, err
		}
	}
	return s.queries.UpdateBackupDestination(ctx, generated.UpdateBackupDestinationParams{
		ID:                   id,
		Name:                 p.Name,
		Provider:             p.Provider,
		Endpoint:             p.Endpoint,
		Region:               p.Region,
		Bucket:               p.Bucket,
		Prefix:               p.Prefix,
		UseSsl:               p.UseSSL,
		CredentialsEncrypted: enc, // nil preserves existing
	})
}

// ListByProject returns a project's destinations without their secrets.
func (s *BackupDestinationService) ListByProject(ctx context.Context, projectID pgtype.UUID) ([]generated.ListBackupDestinationsByProjectRow, error) {
	return s.queries.ListBackupDestinationsByProject(ctx, projectID)
}

// Get returns the raw destination row (including encrypted credentials).
func (s *BackupDestinationService) Get(ctx context.Context, id pgtype.UUID) (generated.BackupDestination, error) {
	return s.queries.GetBackupDestination(ctx, id)
}

// Delete removes a destination by id.
func (s *BackupDestinationService) Delete(ctx context.Context, id pgtype.UUID) error {
	return s.queries.DeleteBackupDestination(ctx, id)
}

// Resolve loads a destination and decrypts it into a transport-level
// backup.Destination ready for backup.NewDestinationClient. Used by the worker.
func (s *BackupDestinationService) Resolve(ctx context.Context, id pgtype.UUID) (backup.Destination, error) {
	row, err := s.queries.GetBackupDestination(ctx, id)
	if err != nil {
		return backup.Destination{}, err
	}
	return s.toDestination(row)
}

// ClientForConfig resolves the destination client for a backup config id. Used
// to delete a config-produced backup's remote object from the right bucket.
func (s *BackupDestinationService) ClientForConfig(ctx context.Context, configID pgtype.UUID) (*backup.DestinationClient, error) {
	cfg, err := s.queries.GetDatabaseBackupConfig(ctx, configID)
	if err != nil {
		return nil, err
	}
	dest, err := s.Resolve(ctx, cfg.DestinationID)
	if err != nil {
		return nil, err
	}
	return backup.NewDestinationClient(dest)
}

// ClientForVolumeBackupConfig resolves the destination client for an application
// volume backup config id. Used by the volume restore worker (download) and by
// config deletion (remove the config's remote objects).
func (s *BackupDestinationService) ClientForVolumeBackupConfig(ctx context.Context, configID pgtype.UUID) (*backup.DestinationClient, error) {
	cfg, err := s.queries.GetApplicationVolumeBackupConfig(ctx, configID)
	if err != nil {
		return nil, err
	}
	dest, err := s.Resolve(ctx, cfg.DestinationID)
	if err != nil {
		return nil, err
	}
	return backup.NewDestinationClient(dest)
}

// Test builds a client for the stored destination and verifies bucket access.
// A local destination has nothing to reach — it always succeeds.
func (s *BackupDestinationService) Test(ctx context.Context, id pgtype.UUID) error {
	dest, err := s.Resolve(ctx, id)
	if err != nil {
		return err
	}
	if dest.IsLocal() {
		return nil
	}
	client, err := backup.NewDestinationClient(dest)
	if err != nil {
		return err
	}
	return client.Test(ctx)
}

// TestConnection verifies ad-hoc connection params (used by the create/edit form
// before the destination is saved). When credentials are omitted and fallbackID
// is set, the stored destination's credentials are used — so editing without
// re-entering the secret still tests against the form's other (possibly changed)
// fields.
func (s *BackupDestinationService) TestConnection(ctx context.Context, p SaveBackupDestinationParams, fallbackID pgtype.UUID) error {
	if p.Provider == "local" {
		return nil
	}
	accessKey, secretKey := "", ""
	if p.Credentials != nil {
		accessKey, secretKey = p.Credentials.AccessKey, p.Credentials.SecretKey
	}
	if (accessKey == "" || secretKey == "") && fallbackID.Valid {
		if row, err := s.queries.GetBackupDestination(ctx, fallbackID); err == nil {
			if creds, derr := s.decryptCredentials(row.CredentialsEncrypted); derr == nil {
				if accessKey == "" {
					accessKey = creds.AccessKey
				}
				if secretKey == "" {
					secretKey = creds.SecretKey
				}
			}
		}
	}
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("credentials are required to test the connection")
	}

	client, err := backup.NewDestinationClient(backup.Destination{
		Endpoint:  p.Endpoint,
		Region:    p.Region,
		Bucket:    p.Bucket,
		Prefix:    p.Prefix,
		AccessKey: accessKey,
		SecretKey: secretKey,
		UseSSL:    p.UseSSL,
	})
	if err != nil {
		return err
	}
	return client.Test(ctx)
}

func (s *BackupDestinationService) toDestination(row generated.BackupDestination) (backup.Destination, error) {
	// A local destination has no credentials to decrypt (credentials_encrypted
	// is an empty, not NULL, bytea — see Create/Update) and no client is ever
	// built for it, so skip decryption entirely rather than fail on empty input.
	if row.Provider == "local" {
		return backup.Destination{Provider: row.Provider}, nil
	}
	creds, err := s.decryptCredentials(row.CredentialsEncrypted)
	if err != nil {
		return backup.Destination{}, err
	}
	return backup.Destination{
		Provider:  row.Provider,
		Endpoint:  row.Endpoint,
		Region:    row.Region,
		Bucket:    row.Bucket,
		Prefix:    row.Prefix,
		AccessKey: creds.AccessKey,
		SecretKey: creds.SecretKey,
		UseSSL:    row.UseSsl,
	}, nil
}

func (s *BackupDestinationService) encryptCredentials(c *DestinationCredentials) ([]byte, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal destination credentials: %w", err)
	}
	enc, err := s.keyring.Encrypt(raw)
	if err != nil {
		return nil, fmt.Errorf("encrypt destination credentials: %w", err)
	}
	return enc, nil
}

func (s *BackupDestinationService) decryptCredentials(encrypted []byte) (DestinationCredentials, error) {
	if len(encrypted) == 0 {
		return DestinationCredentials{}, fmt.Errorf("destination has no stored credentials")
	}
	raw, err := s.keyring.Decrypt(encrypted)
	if err != nil {
		return DestinationCredentials{}, fmt.Errorf("decrypt destination credentials: %w", err)
	}
	var creds DestinationCredentials
	if err := json.Unmarshal(raw, &creds); err != nil {
		return DestinationCredentials{}, fmt.Errorf("unmarshal destination credentials: %w", err)
	}
	return creds, nil
}
