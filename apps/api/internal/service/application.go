package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type ApplicationService struct {
	db      *pgxpool.Pool
	queries *generated.Queries
	runtime runtime.ContainerRuntime
}

func NewApplicationService(db *pgxpool.Pool, queries *generated.Queries, rt runtime.ContainerRuntime) *ApplicationService {
	return &ApplicationService{db: db, queries: queries, runtime: rt}
}

// CreateApplicationParams holds the parameters for creating an application.
type CreateApplicationParams struct {
	ProjectID      pgtype.UUID
	ProjectSlug    string
	Name           string
	BaseSlug       string
	Type           string
	SourceRepo     string
	SourceImage    string
	DockerfilePath string
	BuildType      string
}

// Create inserts the application record and sets its final slug atomically.
func (s *ApplicationService) Create(ctx context.Context, p CreateApplicationParams) (generated.Application, error) {
	var app generated.Application
	err := store.WithTx(ctx, s.db, func(q *generated.Queries) error {
		var err error
		app, err = q.CreateApplication(ctx, generated.CreateApplicationParams{
			ProjectID:      p.ProjectID,
			Name:           p.Name,
			Slug:           p.BaseSlug,
			Type:           p.Type,
			SourceRepo:     pgtype.Text{String: p.SourceRepo, Valid: p.SourceRepo != ""},
			SourceImage:    pgtype.Text{String: p.SourceImage, Valid: p.SourceImage != ""},
			DockerfilePath: pgtype.Text{String: p.DockerfilePath, Valid: p.DockerfilePath != ""},
			BuildType:      p.BuildType,
		})
		if err != nil {
			return err
		}
		appIDStr := uuidToString(app.ID)
		finalSlug := fmt.Sprintf("%s-%s-%s", p.ProjectSlug, p.BaseSlug, appIDStr[:8])
		if err := q.UpdateApplicationSlug(ctx, generated.UpdateApplicationSlugParams{
			ID:   app.ID,
			Slug: finalSlug,
		}); err != nil {
			return err
		}
		app.Slug = finalSlug
		return nil
	})
	return app, err
}

// Update applies field changes to an application.
func (s *ApplicationService) Update(
	ctx context.Context,
	appID pgtype.UUID,
	current generated.Application,
	name, sourceRepo, sourceImage, dockerfilePath, buildTypeOverride, builderImage string,
) (generated.Application, error) {
	if name == "" {
		name = current.Name
	}
	return s.queries.UpdateApplication(ctx, generated.UpdateApplicationParams{
		ID:                appID,
		Name:              name,
		SourceRepo:        pgtype.Text{String: sourceRepo, Valid: sourceRepo != ""},
		SourceImage:       pgtype.Text{String: sourceImage, Valid: sourceImage != ""},
		DockerfilePath:    pgtype.Text{String: dockerfilePath, Valid: dockerfilePath != ""},
		BuildTypeOverride: pgtype.Text{String: buildTypeOverride, Valid: buildTypeOverride != ""},
		BuilderImage:      pgtype.Text{String: builderImage, Valid: builderImage != ""},
		CustomBuildpacks:  current.CustomBuildpacks,
		Status:            current.Status,
	})
}

// Delete stops and removes the application container, then deletes the DB record.
// The DB delete cascades deployments, env vars, and domains via FK ON DELETE CASCADE.
func (s *ApplicationService) Delete(ctx context.Context, appID pgtype.UUID, projectSlug, appSlug string) error {
	appIDStr := uuidToString(appID)
	containerName := naming.ContainerName(projectSlug, appSlug, appIDStr)
	intermediateContainerName := naming.IntermediateContainerName(projectSlug, appIDStr)
	oldContainerName := naming.OldContainerName(appIDStr)
	for _, name := range []string{containerName, intermediateContainerName, oldContainerName} {
		if err := s.runtime.StopContainer(ctx, name); err != nil {
			slog.Warn("could not stop container during app deletion", "container", name, "error", err)
		}
		if err := s.runtime.RemoveContainer(ctx, name); err != nil {
			slog.Warn("could not remove container during app deletion", "container", name, "error", err)
		}
	}
	return s.queries.DeleteApplication(ctx, appID)
}
