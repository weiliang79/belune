package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type provisionDBPayload struct {
	DatabaseID string `json:"database_id"`
}

// reconfigureDBPayload toggles SSH-tunnel external access for a database by
// recreating its container with (enable) or without (disable) a loopback host
// port binding.
type reconfigureDBPayload struct {
	DatabaseID string `json:"database_id"`
	Enable     bool   `json:"enable"`
}

func (h *TaskHandler) HandleProvisionDBTask(ctx context.Context, t *asynq.Task) error {
	var payload provisionDBPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal provision_db payload: %w", err)
	}

	slog.Info("handling provision_db task", "database_id", payload.DatabaseID)

	dbID, err := parseUUID(payload.DatabaseID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid database_id (permanent): %w", err), asynq.SkipRetry)
	}

	db, err := h.Queries.GetDatabase(ctx, dbID)
	if err != nil {
		return fmt.Errorf("get database: %w", err)
	}

	creds, err := h.decryptDBCredentials(db)
	if err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("credentials: %v", err))
		return fmt.Errorf("credentials: %w", err)
	}

	// New databases start internal-only (no host port); external access is an
	// explicit opt-in handled by the reconfigure task.
	if err := h.provisionDBContainer(ctx, db, creds, 0); err != nil {
		h.failDatabase(ctx, dbID, err.Error())
		return err
	}

	slog.Info("database provisioned", "database_id", payload.DatabaseID, "container", db.Slug, "type", db.Type)
	return nil
}

// HandleReconfigureDBTask recreates a database container to add or remove the
// loopback host-port binding used for SSH-tunnel external access. The named
// volume is reattached, so data survives the recreate (brief downtime).
func (h *TaskHandler) HandleReconfigureDBTask(ctx context.Context, t *asynq.Task) error {
	var payload reconfigureDBPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal reconfigure_db payload: %w", err)
	}

	slog.Info("handling reconfigure_db task", "database_id", payload.DatabaseID, "enable", payload.Enable)

	dbID, err := parseUUID(payload.DatabaseID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid database_id (permanent): %w", err), asynq.SkipRetry)
	}

	db, err := h.Queries.GetDatabase(ctx, dbID)
	if err != nil {
		return fmt.Errorf("get database: %w", err)
	}

	creds, err := h.decryptDBCredentials(db)
	if err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("credentials: %v", err))
		return fmt.Errorf("credentials: %w", err)
	}

	// Prior host port, for rollback if the new configuration fails to come up.
	// db was read before any change, so this is the pre-toggle value.
	var oldHostPort int32
	if db.HostPort.Valid {
		oldHostPort = db.HostPort.Int32
	}

	// Allocate the loopback port up front so a failure leaves the old container
	// untouched. A fresh free port is taken each time external access is enabled.
	var hostPort int32
	if payload.Enable {
		p, perr := freeLoopbackPort()
		if perr != nil {
			h.failDatabase(ctx, dbID, fmt.Sprintf("allocate host port: %v", perr))
			return fmt.Errorf("allocate host port: %w", perr)
		}
		hostPort = p
	}

	// Remove the existing container (best-effort; the volume persists).
	h.removeDBContainer(ctx, db.Slug)

	if err := h.provisionDBContainer(ctx, db, creds, hostPort); err != nil {
		// A failed external-access toggle must not take the database offline.
		// Roll back to the prior configuration so the database comes back up;
		// the toggle simply doesn't take effect. Only if rollback also fails is
		// the database genuinely broken (and its volume data is still intact).
		slog.Warn("reconfigure failed; rolling back to previous state",
			"database_id", payload.DatabaseID, "error", err)
		h.removeDBContainer(ctx, db.Slug)
		if rbErr := h.provisionDBContainer(ctx, db, creds, oldHostPort); rbErr != nil {
			h.failDatabase(ctx, dbID, fmt.Sprintf("reconfigure failed and rollback failed: %v", rbErr))
			return fmt.Errorf("reconfigure rollback failed: %w", rbErr)
		}
		slog.Warn("reconfigure rolled back; database restored to previous state",
			"database_id", payload.DatabaseID)
		// Returning nil: the database is healthy in its prior state, so there is
		// nothing to retry. The unchanged external_access state signals to the
		// UI that the toggle did not take.
		return nil
	}

	slog.Info("database reconfigured", "database_id", payload.DatabaseID, "external_access", payload.Enable, "host_port", hostPort)
	return nil
}

func (h *TaskHandler) decryptDBCredentials(db generated.Database) (map[string]string, error) {
	credsJSON, err := h.Keyring.Decrypt(db.CredentialsEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	var creds map[string]string
	if err := json.Unmarshal(credsJSON, &creds); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return creds, nil
}

// dbContainerSpec returns the per-type image, data dir, container port, env, and
// command for a managed database.
func dbContainerSpec(dbType, version string, creds map[string]string) (image, dataDir string, port int32, env map[string]string, cmd []string, err error) {
	switch dbType {
	case "postgres":
		return fmt.Sprintf("postgres:%s", version), "/var/lib/postgresql/data", 5432, map[string]string{
			"POSTGRES_USER":     creds["user"],
			"POSTGRES_PASSWORD": creds["password"],
			"POSTGRES_DB":       creds["database"],
		}, nil, nil
	case "mysql":
		return fmt.Sprintf("mysql:%s", version), "/var/lib/mysql", 3306, map[string]string{
			"MYSQL_ROOT_PASSWORD": creds["root_password"],
			"MYSQL_DATABASE":      creds["database"],
			"MYSQL_USER":          creds["user"],
			"MYSQL_PASSWORD":      creds["password"],
		}, nil, nil
	case "redis":
		return fmt.Sprintf("redis:%s", version), "/data", 6379, map[string]string{},
			[]string{"redis-server", "--requirepass", creds["password"]}, nil
	case "mongo":
		return fmt.Sprintf("mongo:%s", version), "/data/db", 27017, map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": creds["username"],
			"MONGO_INITDB_ROOT_PASSWORD": creds["password"],
		}, nil, nil
	default:
		return "", "", 0, nil, nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// provisionDBContainer (re)creates and starts a database's container, optionally
// binding the database port to 127.0.0.1:hostPort (hostPort > 0). It is
// idempotent on the network/volume/image so it can be used for both first-time
// provisioning and reconfigure recreates. On success the database row is stamped
// running with its connection info and host port.
func (h *TaskHandler) provisionDBContainer(ctx context.Context, db generated.Database, creds map[string]string, hostPort int32) error {
	containerName := db.Slug
	volumeName := fmt.Sprintf("%s-vol", db.Slug)

	image, dataDir, port, env, cmd, err := dbContainerSpec(db.Type, db.Version, creds)
	if err != nil {
		return err
	}

	project, err := h.Queries.GetProject(ctx, db.ProjectID)
	if err != nil {
		return fmt.Errorf("get project for network setup: %w", err)
	}
	projectNetwork := naming.ProjectNetworkName(project.Slug)
	if netErr := h.Runtime.CreateNetwork(ctx, projectNetwork); netErr != nil {
		slog.Debug("could not create project network (may already exist)", "network", projectNetwork, "error", netErr)
	}

	if err := h.Runtime.PullImage(ctx, image); err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	if err := h.Runtime.CreateVolume(ctx, volumeName); err != nil {
		return fmt.Errorf("create volume: %w", err)
	}

	ports := map[string]string{}
	hostBindIP := ""
	hostPortCol := pgtype.Int4{}
	if hostPort > 0 {
		// Loopback only — reachable from the host (e.g. an SSH tunnel), never
		// the public internet.
		ports[fmt.Sprintf("%d", hostPort)] = fmt.Sprintf("%d", port)
		hostBindIP = "127.0.0.1"
		hostPortCol = pgtype.Int4{Int32: hostPort, Valid: true}
	}

	containerID, err := h.Runtime.CreateContainer(ctx, runtime.ContainerConfig{
		Name:        containerName,
		Image:       image,
		Env:         env,
		Cmd:         cmd,
		Ports:       ports,
		HostBindIP:  hostBindIP,
		Volumes:     map[string]string{volumeName: dataDir},
		Network:     projectNetwork,
		CPULimit:    db.CpuLimit,
		MemoryLimit: db.MemoryLimit,
	})
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	if err := h.Runtime.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	h.Queries.UpdateDatabaseAfterProvision(ctx, generated.UpdateDatabaseAfterProvisionParams{
		ID:           db.ID,
		Status:       status.DatabaseRunning,
		InternalHost: pgtype.Text{String: containerName, Valid: true},
		InternalPort: pgtype.Int4{Int32: port, Valid: true},
		HostPort:     hostPortCol,
	})
	return nil
}

// removeDBContainer stops and removes a database container by name. Both steps
// are best-effort: a missing/stopped container is not an error for the caller.
func (h *TaskHandler) removeDBContainer(ctx context.Context, name string) {
	if err := h.Runtime.StopContainer(ctx, name); err != nil {
		slog.Debug("reconfigure: stop container (may already be stopped)", "container", name, "error", err)
	}
	if err := h.Runtime.RemoveContainer(ctx, name); err != nil {
		slog.Debug("reconfigure: remove container (may already be gone)", "container", name, "error", err)
	}
}

// freeLoopbackPort asks the OS for a free TCP port on the loopback interface.
// The listener is closed immediately; the small window before Docker binds it is
// acceptable for a single-host deployment (a collision just fails the recreate,
// which can be retried).
func freeLoopbackPort() (int32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return int32(l.Addr().(*net.TCPAddr).Port), nil
}

func (h *TaskHandler) failDatabase(ctx context.Context, dbID pgtype.UUID, errMsg string) {
	slog.Error("database task failed", "database_id", fmt.Sprintf("%v", dbID), "error", errMsg)
	h.Queries.UpdateDatabaseStatus(ctx, generated.UpdateDatabaseStatusParams{
		ID:     dbID,
		Status: status.DatabaseFailed,
	})
}
