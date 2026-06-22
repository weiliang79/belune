package service

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/service/backup"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type DatabaseService struct {
	queries *generated.Queries
	runtime runtime.ContainerRuntime
	backups *backup.Service // optional; nil disables remote backup cleanup
}

func NewDatabaseService(queries *generated.Queries, rt runtime.ContainerRuntime, backups *backup.Service) *DatabaseService {
	return &DatabaseService{queries: queries, runtime: rt, backups: backups}
}

// cleanupBackups removes a database's backup archives (local files and, when
// configured, S3 objects). Best-effort: the DB-row delete cascades regardless.
func (s *DatabaseService) cleanupBackups(ctx context.Context, dbID pgtype.UUID) {
	rows, err := s.queries.ListDatabaseBackups(ctx, generated.ListDatabaseBackupsParams{DatabaseID: dbID, Limit: 1000})
	if err != nil {
		slog.Warn("could not list backups during db deletion", "error", err)
		return
	}
	var remoteKeys []string
	for _, b := range rows {
		if b.LocalPath.Valid {
			if err := os.Remove(b.LocalPath.String); err != nil && !os.IsNotExist(err) {
				slog.Warn("could not remove backup file during db deletion", "path", b.LocalPath.String, "error", err)
			}
		}
		if b.RemoteKey.Valid {
			remoteKeys = append(remoteKeys, b.RemoteKey.String)
		}
	}
	if len(remoteKeys) > 0 && s.backups != nil && s.backups.Enabled() {
		if err := s.backups.Delete(ctx, remoteKeys); err != nil {
			slog.Warn("could not remove remote backups during db deletion", "count", len(remoteKeys), "error", err)
		}
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
