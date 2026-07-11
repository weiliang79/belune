package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/runtime"
	"github.com/weiliang79/belune/internal/service/backup"
	"github.com/weiliang79/belune/internal/store/generated"
)

type DatabaseService struct {
	queries      *generated.Queries
	runtime      runtime.ContainerRuntime
	backups      *backup.Service           // optional; nil disables global remote backup cleanup
	destinations *BackupDestinationService // optional; routes config-backup remote cleanup
}

func NewDatabaseService(queries *generated.Queries, rt runtime.ContainerRuntime, backups *backup.Service, destinations *BackupDestinationService) *DatabaseService {
	return &DatabaseService{queries: queries, runtime: rt, backups: backups, destinations: destinations}
}

// deleteRemoteBackup removes a backup's remote object, routing to the config's
// project destination when the run came from a config, or the global target
// otherwise. Best-effort.
func (s *DatabaseService) deleteRemoteBackup(ctx context.Context, b generated.DatabaseBackup) {
	if !b.RemoteKey.Valid {
		return
	}
	if b.BackupConfigID.Valid && s.destinations != nil {
		client, err := s.destinations.ClientForConfig(ctx, b.BackupConfigID)
		if err != nil {
			slog.Warn("could not resolve destination for remote backup cleanup", "key", b.RemoteKey.String, "error", err)
			return
		}
		if err := client.DeleteFrom(ctx, []string{b.RemoteKey.String}); err != nil {
			slog.Warn("could not remove remote backup", "key", b.RemoteKey.String, "error", err)
		}
		return
	}
	if s.backups != nil && s.backups.Enabled() {
		if err := s.backups.Delete(ctx, []string{b.RemoteKey.String}); err != nil {
			slog.Warn("could not remove remote backup", "key", b.RemoteKey.String, "error", err)
		}
	}
}

// DeleteBackup removes a single backup belonging to dbID: its local file, S3
// object, and row. Returns an error if the backup isn't found or doesn't belong
// to the database.
func (s *DatabaseService) DeleteBackup(ctx context.Context, dbID, backupID pgtype.UUID) error {
	b, err := s.queries.GetDatabaseBackup(ctx, backupID)
	if err != nil {
		return err
	}
	if b.DatabaseID != dbID {
		return fmt.Errorf("backup does not belong to this database")
	}
	if b.LocalPath.Valid {
		if err := os.Remove(b.LocalPath.String); err != nil && !os.IsNotExist(err) {
			slog.Warn("could not remove backup file", "path", b.LocalPath.String, "error", err)
		}
	}
	s.deleteRemoteBackup(ctx, b)
	return s.queries.DeleteDatabaseBackup(ctx, backupID)
}

// cleanupBackups removes a database's backup archives (local files and, when
// configured, S3 objects). Best-effort: the DB-row delete cascades regardless.
func (s *DatabaseService) cleanupBackups(ctx context.Context, dbID pgtype.UUID) {
	rows, err := s.queries.ListDatabaseBackups(ctx, generated.ListDatabaseBackupsParams{DatabaseID: dbID, Limit: 1000})
	if err != nil {
		slog.Warn("could not list backups during db deletion", "error", err)
		return
	}
	for _, b := range rows {
		if b.LocalPath.Valid {
			if err := os.Remove(b.LocalPath.String); err != nil && !os.IsNotExist(err) {
				slog.Warn("could not remove backup file during db deletion", "path", b.LocalPath.String, "error", err)
			}
		}
		// Route remote cleanup per backup (config backups live in project
		// destinations; ad-hoc runs in the global target).
		s.deleteRemoteBackup(ctx, b)
	}
}

// Delete stops and removes the database container and its volume, then deletes the DB record.
func (s *DatabaseService) Delete(ctx context.Context, dbID pgtype.UUID) error {
	db, err := s.queries.GetDatabase(ctx, dbID)
	if err != nil {
		return err
	}

	if err := s.runtime.StopContainer(ctx, db.Slug); err != nil {
		slog.Warn("could not stop container during db deletion", "container", db.Slug, "error", err)
	}
	if err := s.runtime.RemoveContainer(ctx, db.Slug); err != nil {
		slog.Warn("could not remove container during db deletion", "container", db.Slug, "error", err)
	}
	if err := s.runtime.RemoveVolume(ctx, db.Slug+"-vol"); err != nil {
		slog.Warn("could not remove volume during db deletion", "volume", db.Slug+"-vol", "error", err)
	}

	s.cleanupBackups(ctx, dbID)

	return s.queries.DeleteDatabase(ctx, dbID)
}
