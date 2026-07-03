package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type ApplicationService struct {
	db      *pgxpool.Pool
	queries *generated.Queries
	runtime runtime.ContainerRuntime
	keyring *crypto.Keyring
}

func NewApplicationService(db *pgxpool.Pool, queries *generated.Queries, rt runtime.ContainerRuntime, keyring *crypto.Keyring) *ApplicationService {
	return &ApplicationService{db: db, queries: queries, runtime: rt, keyring: keyring}
}

// CreateApplicationParams holds the parameters for creating an application.
type CreateApplicationParams struct {
	ProjectID        pgtype.UUID
	ProjectSlug      string
	Name             string
	BaseSlug         string
	Type             string
	SourceRepo       string
	SourceImage      string
	DockerfilePath   string
	BuildType        string
	CPULimit         float64
	MemoryLimit      int64
	GitToken         string      // plaintext PAT; encrypted before storage
	HealthCheckPath  string      // HTTP path to poll after deploy (e.g. /healthz)
	GitIntegrationID pgtype.UUID // optional FK to a connected provider account
}

// Create inserts the application record and sets its final slug atomically.
// A webhook secret is auto-generated. If GitToken is set, it is encrypted before storage.
func (s *ApplicationService) Create(ctx context.Context, p CreateApplicationParams) (generated.Application, error) {
	webhookSecret, err := crypto.GenerateWebhookSecret()
	if err != nil {
		return generated.Application{}, fmt.Errorf("generate webhook secret: %w", err)
	}

	var gitCreds []byte
	if p.GitToken != "" {
		encrypted, err := s.keyring.Encrypt([]byte(p.GitToken))
		if err != nil {
			return generated.Application{}, fmt.Errorf("encrypt git token: %w", err)
		}
		gitCreds = encrypted
	}

	var app generated.Application
	err = store.WithTx(ctx, s.db, func(q *generated.Queries) error {
		var err error
		app, err = q.CreateApplication(ctx, generated.CreateApplicationParams{
			ProjectID:               p.ProjectID,
			Name:                    p.Name,
			Slug:                    p.BaseSlug,
			Type:                    p.Type,
			SourceRepo:              pgtype.Text{String: p.SourceRepo, Valid: p.SourceRepo != ""},
			SourceImage:             pgtype.Text{String: p.SourceImage, Valid: p.SourceImage != ""},
			DockerfilePath:          pgtype.Text{String: p.DockerfilePath, Valid: p.DockerfilePath != ""},
			BuildType:               p.BuildType,
			CpuLimit:                p.CPULimit,
			MemoryLimit:             p.MemoryLimit,
			WebhookSecret:           pgtype.Text{String: webhookSecret, Valid: true},
			GitCredentialsEncrypted: gitCreds,
			HealthCheckPath:         pgtype.Text{String: p.HealthCheckPath, Valid: p.HealthCheckPath != ""},
			GitIntegrationID:        p.GitIntegrationID,
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

// UpdateApplicationParams holds parameters for updating an application.
type UpdateApplicationParams struct {
	Name              string
	SourceRepo        string
	SourceImage       string
	DockerfilePath    string
	BuildTypeOverride string
	BuilderImage      string
	CPULimit          float64
	MemoryLimit       int64
	GitToken          string      // plaintext PAT; encrypted before storage; empty = no change
	HealthCheckPath   string      // empty = clear existing
	GitIntegrationID  pgtype.UUID // optional FK to a connected provider account; zero = clear
}

// Update applies field changes to an application.
// If GitToken is non-empty, it replaces the stored credentials; if empty, existing credentials are preserved.
func (s *ApplicationService) Update(
	ctx context.Context,
	appID pgtype.UUID,
	current generated.Application,
	p UpdateApplicationParams,
) (generated.Application, error) {
	name := p.Name
	if name == "" {
		name = current.Name
	}

	gitCreds := current.GitCredentialsEncrypted
	if p.GitToken != "" {
		encrypted, err := s.keyring.Encrypt([]byte(p.GitToken))
		if err != nil {
			return generated.Application{}, fmt.Errorf("encrypt git token: %w", err)
		}
		gitCreds = encrypted
	}

	return s.queries.UpdateApplication(ctx, generated.UpdateApplicationParams{
		ID:                      appID,
		Name:                    name,
		SourceRepo:              pgtype.Text{String: p.SourceRepo, Valid: p.SourceRepo != ""},
		SourceImage:             pgtype.Text{String: p.SourceImage, Valid: p.SourceImage != ""},
		DockerfilePath:          pgtype.Text{String: p.DockerfilePath, Valid: p.DockerfilePath != ""},
		BuildTypeOverride:       pgtype.Text{String: p.BuildTypeOverride, Valid: p.BuildTypeOverride != ""},
		BuilderImage:            pgtype.Text{String: p.BuilderImage, Valid: p.BuilderImage != ""},
		CustomBuildpacks:        current.CustomBuildpacks,
		Status:                  current.Status,
		CpuLimit:                p.CPULimit,
		MemoryLimit:             p.MemoryLimit,
		GitCredentialsEncrypted: gitCreds,
		HealthCheckPath:         pgtype.Text{String: p.HealthCheckPath, Valid: p.HealthCheckPath != ""},
		GitIntegrationID:        p.GitIntegrationID,
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

	// Drop the per-app CNB cache volumes too. They are tagged paas-cache=true
	// so PruneVolumes would not have reclaimed them; leaving them behind after
	// delete would leak disk space permanently.
	for _, vol := range []string{
		naming.CNBCacheVolumeName(appIDStr),
		naming.CNBLaunchCacheVolumeName(appIDStr),
	} {
		if err := s.runtime.RemoveVolume(ctx, vol); err != nil {
			slog.Debug("could not remove cache volume during app deletion (may not exist)", "volume", vol, "error", err)
		}
	}

	// Drop persistent data volumes. They are tagged paas-data=true so
	// PruneVolumes never reclaims them; app deletion is the only point they can
	// be cleaned up. List them BEFORE DeleteApplication, whose ON DELETE CASCADE
	// removes the application_volumes rows.
	dataVols, err := s.queries.ListApplicationVolumes(ctx, appID)
	if err != nil {
		slog.Warn("could not list data volumes during app deletion", "application_id", appIDStr, "error", err)
	}
	for _, v := range dataVols {
		volName := naming.AppVolumeName(appIDStr, v.Name)
		if err := s.runtime.RemoveVolume(ctx, volName); err != nil {
			slog.Debug("could not remove data volume during app deletion (may not exist)", "volume", volName, "error", err)
		}
	}

	return s.queries.DeleteApplication(ctx, appID)
}

// FindOrCreatePreview returns the preview child for (parent, branch), creating
// it when missing. Children inherit their parent's source config, build type,
// resource limits, and git credential linkage, so the existing deploy worker
// handles them without special casing. The child's slug follows the same
// "{projectSlug}-{baseSlug}-{appID[:8]}" shape as any other application, where
// baseSlug = "{parentBaseSlug}-{branchSlug}".
//
// parentBaseSlug is the pre-finalization base (before the project prefix was
// baked in by Create); callers derive it from the parent row's metadata.
func (s *ApplicationService) FindOrCreatePreview(
	ctx context.Context,
	parent generated.Application,
	projectSlug, parentBaseSlug, branch, branchSlug string,
) (generated.Application, bool, error) {
	existing, err := s.queries.GetPreviewByParentBranch(ctx, generated.GetPreviewByParentBranchParams{
		ParentApplicationID: pgtype.UUID{Bytes: parent.ID.Bytes, Valid: true},
		Branch:              pgtype.Text{String: branch, Valid: true},
	})
	if err == nil {
		return existing, false, nil
	}

	previewBaseSlug := fmt.Sprintf("%s-%s", parentBaseSlug, branchSlug)

	var created generated.Application
	err = store.WithTx(ctx, s.db, func(q *generated.Queries) error {
		var txErr error
		created, txErr = q.CreatePreviewApplication(ctx, generated.CreatePreviewApplicationParams{
			ProjectID:               parent.ProjectID,
			Name:                    fmt.Sprintf("%s (%s)", parent.Name, branch),
			Slug:                    previewBaseSlug,
			Type:                    parent.Type,
			SourceRepo:              parent.SourceRepo,
			SourceImage:             parent.SourceImage,
			DockerfilePath:          parent.DockerfilePath,
			BuildType:               parent.BuildType,
			CpuLimit:                parent.CpuLimit,
			MemoryLimit:             parent.MemoryLimit,
			GitCredentialsEncrypted: parent.GitCredentialsEncrypted,
			HealthCheckPath:         parent.HealthCheckPath,
			GitIntegrationID:        parent.GitIntegrationID,
			ParentApplicationID:     pgtype.UUID{Bytes: parent.ID.Bytes, Valid: true},
			Branch:                  pgtype.Text{String: branch, Valid: true},
		})
		if txErr != nil {
			return txErr
		}
		appIDStr := uuidToString(created.ID)
		finalSlug := fmt.Sprintf("%s-%s-%s", projectSlug, previewBaseSlug, appIDStr[:8])
		if txErr := q.UpdateApplicationSlug(ctx, generated.UpdateApplicationSlugParams{
			ID:   created.ID,
			Slug: finalSlug,
		}); txErr != nil {
			return txErr
		}
		created.Slug = finalSlug
		return nil
	})
	if err != nil {
		return generated.Application{}, false, err
	}
	return created, true, nil
}
