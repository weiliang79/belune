package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/naming"
	"github.com/weiling79/belune/internal/runtime"
	"github.com/weiling79/belune/internal/store/generated"
)

type ProjectService struct {
	queries *generated.Queries
	runtime runtime.ContainerRuntime
}

func NewProjectService(queries *generated.Queries, rt runtime.ContainerRuntime) *ProjectService {
	return &ProjectService{queries: queries, runtime: rt}
}

// Delete stops and removes all application containers for the project, then deletes the DB record.
// The DB delete cascades applications, deployments, env vars, and domains via FK ON DELETE CASCADE.
func (s *ProjectService) Delete(ctx context.Context, projectID pgtype.UUID) error {
	project, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		slog.Error("failed to fetch project for container cleanup", "project_id", fmt.Sprintf("%v", projectID), "error", err)
		// Still attempt DB delete even if we can't clean up containers
		return s.queries.DeleteProject(ctx, projectID)
	}

	applications, err := s.queries.ListApplicationsByProject(ctx, projectID)
	if err == nil {
		for _, app := range applications {
			appIDStr := uuidToString(app.ID)
			containerName := naming.ContainerName(project.Slug, app.Slug, appIDStr)
			intermediateContainerName := naming.IntermediateContainerName(project.Slug, appIDStr)
			oldContainerName := naming.OldContainerName(appIDStr)
			for _, name := range []string{containerName, intermediateContainerName, oldContainerName} {
				if err := s.runtime.StopContainer(ctx, name); err != nil {
					slog.Warn("could not stop container during project deletion", "container", name, "error", err)
				}
				if err := s.runtime.RemoveContainer(ctx, name); err != nil {
					slog.Warn("could not remove container during project deletion", "container", name, "error", err)
				}
			}
		}
	}

	return s.queries.DeleteProject(ctx, projectID)
}
