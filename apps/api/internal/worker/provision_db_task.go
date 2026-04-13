package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type provisionDBPayload struct {
	DatabaseID string `json:"database_id"`
}

func (h *TaskHandler) HandleProvisionDBTask(ctx context.Context, t *asynq.Task) error {
	var payload provisionDBPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal provision_db payload: %w", err)
	}

	slog.Info("handling provision_db task", "database_id", payload.DatabaseID)

	dbID, err := parseUUID(payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("invalid database_id (permanent): %w: %w", err, asynq.SkipRetry)
	}

	db, err := h.Queries.GetDatabase(ctx, dbID)
	if err != nil {
		return fmt.Errorf("get database: %w", err)
	}

	// Decrypt credentials
	credsJSON, err := crypto.Decrypt(db.CredentialsEncrypted, h.EncryptionKey)
	if err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("decrypt credentials: %v", err))
		return fmt.Errorf("decrypt credentials: %w", err)
	}

	var creds map[string]string
	if err := json.Unmarshal(credsJSON, &creds); err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("unmarshal credentials: %v", err))
		return fmt.Errorf("unmarshal credentials: %w", err)
	}

	containerName := db.Slug
	volumeName := fmt.Sprintf("%s-vol", db.Slug)

	// Determine image, port, data dir, env vars, and cmd based on db type
	var (
		image   string
		port    int32
		dataDir string
		env     map[string]string
		cmd     []string
	)

	switch db.Type {
	case "postgres":
		image = fmt.Sprintf("postgres:%s", db.Version)
		port = 5432
		dataDir = "/var/lib/postgresql/data"
		env = map[string]string{
			"POSTGRES_USER":     creds["user"],
			"POSTGRES_PASSWORD": creds["password"],
			"POSTGRES_DB":       creds["database"],
		}
	case "mysql":
		image = fmt.Sprintf("mysql:%s", db.Version)
		port = 3306
		dataDir = "/var/lib/mysql"
		env = map[string]string{
			"MYSQL_ROOT_PASSWORD": creds["root_password"],
			"MYSQL_DATABASE":      creds["database"],
			"MYSQL_USER":          creds["user"],
			"MYSQL_PASSWORD":      creds["password"],
		}
	case "redis":
		image = fmt.Sprintf("redis:%s", db.Version)
		port = 6379
		dataDir = "/data"
		env = map[string]string{}
		cmd = []string{"redis-server", "--requirepass", creds["password"]}
	case "mongo":
		image = fmt.Sprintf("mongo:%s", db.Version)
		port = 27017
		dataDir = "/data/db"
		env = map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": creds["username"],
			"MONGO_INITDB_ROOT_PASSWORD": creds["password"],
		}
	default:
		h.failDatabase(ctx, dbID, fmt.Sprintf("unsupported database type: %s", db.Type))
		return fmt.Errorf("unsupported database type: %s", db.Type)
	}

	// Ensure project-scoped network exists for DB–app communication (idempotent)
	project, err := h.Queries.GetProject(ctx, db.ProjectID)
	if err != nil {
		slog.Warn("failed to get project for network setup, using paas-infra fallback", "error", err)
	}
	projectNetwork := "paas-infra"
	if project.Slug != "" {
		projectNetwork = naming.ProjectNetworkName(project.Slug)
		if netErr := h.Runtime.CreateNetwork(ctx, projectNetwork); netErr != nil {
			slog.Debug("could not create project network (may already exist)", "network", projectNetwork, "error", netErr)
		}
	}

	// Pull the image
	slog.Info("pulling database image", "image", image)
	if err := h.Runtime.PullImage(ctx, image); err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("pull image: %v", err))
		return fmt.Errorf("pull image: %w", err)
	}

	// Create the volume
	if err := h.Runtime.CreateVolume(ctx, volumeName); err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("create volume: %v", err))
		return fmt.Errorf("create volume: %w", err)
	}

	// Create the container on the project network so apps can connect
	containerID, err := h.Runtime.CreateContainer(ctx, runtime.ContainerConfig{
		Name:    containerName,
		Image:   image,
		Env:     env,
		Cmd:     cmd,
		Ports:   map[string]string{},
		Volumes: map[string]string{volumeName: dataDir},
		Network: projectNetwork,
	})
	if err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("create container: %v", err))
		return fmt.Errorf("create container: %w", err)
	}

	// Start the container
	if err := h.Runtime.StartContainer(ctx, containerID); err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("start container: %v", err))
		return fmt.Errorf("start container: %w", err)
	}

	// Update database record with running status and connection info
	h.Queries.UpdateDatabaseAfterProvision(ctx, generated.UpdateDatabaseAfterProvisionParams{
		ID:           dbID,
		Status:       status.DatabaseRunning,
		InternalHost: pgtype.Text{String: containerName, Valid: true},
		InternalPort: pgtype.Int4{Int32: port, Valid: true},
	})

	slog.Info("database provisioned",
		"database_id", payload.DatabaseID,
		"container", containerName,
		"type", db.Type,
		"port", port,
	)
	return nil
}

func (h *TaskHandler) failDatabase(ctx context.Context, dbID pgtype.UUID, errMsg string) {
	slog.Error("database provisioning failed", "database_id", fmt.Sprintf("%v", dbID), "error", errMsg)
	h.Queries.UpdateDatabaseStatus(ctx, generated.UpdateDatabaseStatusParams{
		ID:     dbID,
		Status: status.DatabaseFailed,
	})
}
