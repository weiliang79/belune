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

// deleteRemoteBackup removes a backup's remote object. It routes to the
// destination the backup was recorded as written to, falling back to the
// config's current destination (backups predating 000061) and then the global
// target. Best-effort — deleting from the wrong bucket would leave the real
// object behind, which is why the recorded location wins.
func (s *DatabaseService) deleteRemoteBackup(ctx context.Context, b generated.DatabaseBackup) {
	if !b.RemoteKey.Valid {
		return
	}
	if s.destinations != nil {
		locs, err := s.queries.ListLocationsForDatabaseBackup(ctx, b.ID)
		if err != nil {
			slog.Warn("could not list backup locations for remote cleanup", "key", b.RemoteKey.String, "error", err)
		}
		for _, loc := range locs {
			if !loc.RemoteKey.Valid {
				continue
			}
			client, cerr := s.destinations.ClientForDestination(ctx, loc.DestinationID)
			if cerr != nil {
				slog.Warn("could not resolve recorded destination for remote backup cleanup", "key", loc.RemoteKey.String, "error", cerr)
				continue
			}
			if client == nil {
				continue // local destination: the file is handled by the caller
			}
			if derr := client.DeleteFrom(ctx, []string{loc.RemoteKey.String}); derr != nil {
				slog.Warn("could not remove remote backup", "key", loc.RemoteKey.String, "error", derr)
			}
		}
		if len(locs) > 0 {
			return
		}
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

// cleanupBackupsPageSize bounds one listing pass; cleanupBackups keeps going
// until a short page. The old single capped call was silently partial: the
// delete-confirmation dialog counts every backup, so a database with more than
// one page promised destruction that never happened and left the surplus S3
// objects with no row pointing at them — unreachable, unprunable, still billed.
const cleanupBackupsPageSize = 1000

// cleanupBackups removes a database's backup archives (local files and, when
// configured, S3 objects). Best-effort: the DB-row delete cascades regardless.
func (s *DatabaseService) cleanupBackups(ctx context.Context, dbID pgtype.UUID) {
	for {
		rows, err := s.queries.ListDatabaseBackups(ctx, generated.ListDatabaseBackupsParams{
			DatabaseID: dbID, Limit: cleanupBackupsPageSize,
		})
		if err != nil {
			slog.Warn("could not list backups during db deletion", "error", err)
			return
		}
		if len(rows) == 0 {
			return
		}
		// Bail when a page removes nothing: the listing has no offset, so a page
		// that cannot delete its rows would be handed back identically forever.
		if s.cleanupBackupPage(ctx, rows) == 0 {
			slog.Warn("stopping backup cleanup: no rows could be removed",
				"remaining", len(rows))
			return
		}
		if len(rows) < cleanupBackupsPageSize {
			return
		}
	}
}

// cleanupBackupPage erases one page of backups: local file, remote object, and
// row. The row delete is what lets the caller page — the listing has no offset,
// so each pass must shrink the set it is walking. Returns how many rows went.
func (s *DatabaseService) cleanupBackupPage(ctx context.Context, rows []generated.DatabaseBackup) int {
	removed := 0
	for _, b := range rows {
		if b.LocalPath.Valid {
			if err := os.Remove(b.LocalPath.String); err != nil && !os.IsNotExist(err) {
				slog.Warn("could not remove backup file during db deletion", "path", b.LocalPath.String, "error", err)
			}
		}
		// Route remote cleanup per backup (config backups live in project
		// destinations; ad-hoc runs in the global target).
		s.deleteRemoteBackup(ctx, b)
		if err := s.queries.DeleteDatabaseBackup(ctx, b.ID); err != nil {
			slog.Warn("could not delete backup row during db deletion",
				"backup_id", b.ID, "error", err)
			continue
		}
		removed++
	}
	return removed
}

// DeletionImpact is what deleting a database takes with it beyond the database
// itself. database_backups.database_id is ON DELETE CASCADE and cleanupBackups
// erases the remote objects too, so this is destruction the operator has to be
// shown before they consent to it — not a warning after the fact.
type DeletionImpact struct {
	BackupCount  int64
	Destinations []string
}

// DeletionImpact counts the backups a delete would destroy and names the
// destinations holding copies. Destination names come from recorded locations,
// so backups written before 000061 are counted but their bucket is not named.
func (s *DatabaseService) DeletionImpact(ctx context.Context, dbID pgtype.UUID) (DeletionImpact, error) {
	count, err := s.queries.CountDatabaseBackupsWithArtifacts(ctx, dbID)
	if err != nil {
		return DeletionImpact{}, fmt.Errorf("count database backups: %w", err)
	}
	names, err := s.queries.ListDestinationNamesForDatabaseBackups(ctx, dbID)
	if err != nil {
		return DeletionImpact{}, fmt.Errorf("list backup destinations: %w", err)
	}
	return DeletionImpact{BackupCount: count, Destinations: names}, nil
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
