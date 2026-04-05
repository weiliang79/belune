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
	db            *pgxpool.Pool
	queries       *generated.Queries
	runtime       runtime.ContainerRuntime
	encryptionKey string
}

func NewApplicationService(db *pgxpool.Pool, queries *generated.Queries, rt runtime.ContainerRuntime, encryptionKey string) *ApplicationService {
	return &ApplicationService{db: db, queries: queries, runtime: rt, encryptionKey: encryptionKey}
}

// CreateApplicationParams holds the parameters for creating an application.
type CreateApplicationParams struct {
	ProjectID       pgtype.UUID
	ProjectSlug     string
	Name            string
	BaseSlug        string
	Type            string
	SourceRepo      string
	SourceImage     string
	DockerfilePath  string
	BuildType       string
	CPULimit        float64
	MemoryLimit     int64
	GitToken        string      // plaintext PAT; encrypted before storage
	HealthCheckPath string      // HTTP path to poll after deploy (e.g. /healthz)
	Port            int         // container port the app listens on (default 8080)
	GitCredentialID pgtype.UUID // optional FK to centralized git_credentials
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
		encrypted, err := crypto.Encrypt([]byte(p.GitToken), s.encryptionKey)
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
			Port:                    portOrDefault(p.Port),
			GitCredentialID:         p.GitCredentialID,
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
	Port              int         // container port; 0 = preserve existing
	GitCredentialID   pgtype.UUID // optional FK to centralized git_credentials; zero = no change
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

	port := current.Port
	if p.Port > 0 {
		port = int32(p.Port)
	}

	gitCreds := current.GitCredentialsEncrypted
	if p.GitToken != "" {
		encrypted, err := crypto.Encrypt([]byte(p.GitToken), s.encryptionKey)
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
		Port:                    port,
		GitCredentialID:         p.GitCredentialID,
	})
}

// portOrDefault returns p if > 0, else 8080.
func portOrDefault(p int) int32 {
	if p > 0 {
		return int32(p)
	}
	return 8080
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
