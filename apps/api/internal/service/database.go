package service

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type DatabaseService struct {
	queries *generated.Queries
	runtime runtime.ContainerRuntime
}

func NewDatabaseService(queries *generated.Queries, rt runtime.ContainerRuntime) *DatabaseService {
	return &DatabaseService{queries: queries, runtime: rt}
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

	return s.queries.DeleteDatabase(ctx, dbID)
}
