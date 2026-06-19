package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

var defaultVersions = map[string]string{
	"postgres": "16",
	"mysql":    "8",
	"redis":    "7",
	"mongo":    "7",
}

type createDatabaseCredentials struct {
	User         string `json:"user"`
	Password     string `json:"password"`
	DatabaseName string `json:"database_name"`
	RootPassword string `json:"root_password"`
}

type createDatabaseRequest struct {
	Name        string                     `json:"name"`
	Slug        string                     `json:"slug"`
	Type        string                     `json:"type"`
	Version     string                     `json:"version"`
	Credentials *createDatabaseCredentials `json:"credentials"`

	// "other" type only: an arbitrary container image run as a managed database.
	Image          string            `json:"image"`
	ContainerPort  int32             `json:"container_port"`
	DataDir        string            `json:"data_dir"`
	Env            map[string]string `json:"env"`             // passed verbatim as container env
	BackupMode     string            `json:"backup_mode"`     // volume_snapshot | command
	BackupCommand  string            `json:"backup_command"`  // command mode: dump into $PAAS_BACKUP_DIR
	RestoreCommand string            `json:"restore_command"` // command mode: restore from $PAAS_BACKUP_DIR
}

var defaultUsers = map[string]string{
	"postgres": "postgres",
	"mysql":    "root",
	"redis":    "default",
	"mongo":    "admin",
}

type provisionDBPayload struct {
	DatabaseID string `json:"database_id"`
}

type volumeInfo struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
}

// externalAccessInfo describes the SSH-tunnel loopback exposure. Enabled means a
// host port is bound on 127.0.0.1; the frontend builds the ssh -L command and a
// localhost connection string from these fields.
type externalAccessInfo struct {
	Enabled  bool   `json:"enabled"`
	HostPort int32  `json:"host_port,omitempty"`
	SSHHost  string `json:"ssh_host,omitempty"`
	SSHUser  string `json:"ssh_user,omitempty"`
}

type databaseResponse struct {
	generated.Database
	Credentials      map[string]string   `json:"credentials,omitempty"`
	ConnectionString string              `json:"connection_string,omitempty"`
	Volume           *volumeInfo         `json:"volume,omitempty"`
	ExternalAccess   *externalAccessInfo `json:"external_access,omitempty"`
}

func (h *Handler) CreateDatabase(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	var projectUUID pgtype.UUID
	if err := projectUUID.Scan(projectID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if !h.canAccessProject(r, projectUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req createDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}

	// Validate type
	isOther := req.Type == "other"
	if !isOther {
		if _, ok := defaultVersions[req.Type]; !ok {
			writeError(w, http.StatusBadRequest, "type must be one of: postgres, mysql, redis, mongo, other")
			return
		}
		// Apply default version if empty.
		if req.Version == "" {
			req.Version = defaultVersions[req.Type]
		}
	} else {
		// "other": the user supplies the image, port, data dir, and backup mode.
		if req.Image == "" || req.ContainerPort <= 0 {
			writeError(w, http.StatusBadRequest, "image and container_port are required for 'other' databases")
			return
		}
		if req.DataDir == "" {
			req.DataDir = "/data"
		}
		if req.BackupMode == "" {
			req.BackupMode = "volume_snapshot"
		}
		if req.BackupMode != "volume_snapshot" && req.BackupMode != "command" {
			writeError(w, http.StatusBadRequest, "backup_mode must be volume_snapshot or command")
			return
		}
		if req.BackupMode == "command" && (req.BackupCommand == "" || req.RestoreCommand == "") {
			writeError(w, http.StatusBadRequest, "backup_command and restore_command are required for command backup mode")
			return
		}
	}

	// Fetch project for slug
	project, err := h.queries.GetProject(r.Context(), projectUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	baseSlug := naming.Slugify(req.Name)
	if req.Slug != "" {
		baseSlug = naming.Slugify(req.Slug)
	}

	// Build credentials. For known engines these are well-known keys consumed by
	// the per-engine env in provisioning; for "other" the map is passed verbatim
	// as container env (so it doubles as the env/credentials the image needs).
	creds := make(map[string]string)
	if isOther {
		for k, v := range req.Env {
			creds[k] = v
		}
	} else {
		// Generate random password
		passwordBytes := make([]byte, 16)
		if _, err := rand.Read(passwordBytes); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate credentials")
			return
		}
		password := hex.EncodeToString(passwordBytes)

		// Determine effective credential values
		user := defaultUsers[req.Type]
		dbName := req.Name
		if req.Credentials != nil {
			if req.Credentials.User != "" {
				user = req.Credentials.User
			}
			if req.Credentials.Password != "" {
				password = req.Credentials.Password
			}
			if req.Credentials.DatabaseName != "" {
				dbName = req.Credentials.DatabaseName
			}
		}

		switch req.Type {
		case "postgres":
			creds["user"] = user
			creds["password"] = password
			creds["database"] = dbName
		case "mysql":
			rootPassword := ""
			if req.Credentials != nil && req.Credentials.RootPassword != "" {
				rootPassword = req.Credentials.RootPassword
			} else {
				rootPasswordBytes := make([]byte, 16)
				if _, err := rand.Read(rootPasswordBytes); err != nil {
					writeError(w, http.StatusInternalServerError, "failed to generate root password")
					return
				}
				rootPassword = hex.EncodeToString(rootPasswordBytes)
			}
			creds["root_password"] = rootPassword
			creds["user"] = user
			creds["password"] = password
			creds["database"] = dbName
		case "redis":
			creds["password"] = password
		case "mongo":
			creds["username"] = user
			creds["password"] = password
		}
	}

	credsJSON, err := json.Marshal(creds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal credentials")
		return
	}

	encrypted, err := h.cfg.Keyring.Encrypt(credsJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt credentials")
		return
	}

	// "other"-type provisioning columns (NULL/none for known engines).
	imageCol := pgtype.Text{}
	portCol := pgtype.Int4{}
	dataDirCol := pgtype.Text{}
	backupCmdCol := pgtype.Text{}
	restoreCmdCol := pgtype.Text{}
	backupMode := "none"
	if isOther {
		imageCol = pgtype.Text{String: req.Image, Valid: true}
		portCol = pgtype.Int4{Int32: req.ContainerPort, Valid: true}
		dataDirCol = pgtype.Text{String: req.DataDir, Valid: true}
		backupMode = req.BackupMode
		if req.BackupMode == "command" {
			backupCmdCol = pgtype.Text{String: req.BackupCommand, Valid: true}
			restoreCmdCol = pgtype.Text{String: req.RestoreCommand, Valid: true}
		}
	}

	var db generated.Database
	if err := store.WithTx(r.Context(), h.db, func(q *generated.Queries) error {
		var err error
		db, err = q.CreateDatabase(r.Context(), generated.CreateDatabaseParams{
			ProjectID:            projectUUID,
			Type:                 req.Type,
			Name:                 req.Name,
			Slug:                 baseSlug,
			Version:              req.Version,
			Status:               status.DatabaseCreating,
			InternalHost:         pgtype.Text{},
			InternalPort:         pgtype.Int4{},
			CredentialsEncrypted: encrypted,
			Image:                imageCol,
			ContainerPort:        portCol,
			DataDir:              dataDirCol,
			BackupMode:           backupMode,
			BackupCommand:        backupCmdCol,
			RestoreCommand:       restoreCmdCol,
		})
		if err != nil {
			return err
		}
		// Construct final slug: {projectSlug}-{baseSlug}-{shortId}
		dbIDStr := fmt.Sprintf("%x-%x-%x-%x-%x",
			db.ID.Bytes[0:4], db.ID.Bytes[4:6], db.ID.Bytes[6:8], db.ID.Bytes[8:10], db.ID.Bytes[10:16])
		finalSlug := fmt.Sprintf("%s-%s-%s", project.Slug, baseSlug, dbIDStr[:8])
		if err := q.UpdateDatabaseSlug(r.Context(), generated.UpdateDatabaseSlugParams{
			ID:   db.ID,
			Slug: finalSlug,
		}); err != nil {
			return err
		}
		db.Slug = finalSlug
		return nil
	}); err != nil {
		slog.Error("failed to create database", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create database")
		return
	}

	// Enqueue provision task
	dbIDStr := fmt.Sprintf("%x-%x-%x-%x-%x",
		db.ID.Bytes[0:4], db.ID.Bytes[4:6], db.ID.Bytes[6:8], db.ID.Bytes[8:10], db.ID.Bytes[10:16])
	payload, err := json.Marshal(provisionDBPayload{DatabaseID: dbIDStr})
	if err != nil {
		slog.Error("failed to marshal provision_db payload", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create provision task")
		return
	}
	task := asynq.NewTask("provision_db", payload)
	if _, err := h.asynq.Enqueue(task, asynq.Queue("critical")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue provision task")
		return
	}

	h.audit(r, "create_database", "database", uuidToString(db.ID), map[string]any{"name": req.Name, "type": req.Type})

	writeJSON(w, http.StatusAccepted, db)
}

func (h *Handler) ListDatabases(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	var projectUUID pgtype.UUID
	if err := projectUUID.Scan(projectID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if !h.canAccessProject(r, projectUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	databases, err := h.queries.ListDatabasesByProject(r.Context(), projectUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list databases")
		return
	}

	writeJSON(w, http.StatusOK, databases)
}

func (h *Handler) GetDatabase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "databaseId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid database id")
		return
	}

	if !h.canAccessDatabase(r, uuid) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	db, err := h.queries.GetDatabase(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	resp := databaseResponse{Database: db}

	// Decrypt credentials
	if len(db.CredentialsEncrypted) > 0 {
		credsJSON, err := h.cfg.Keyring.Decrypt(db.CredentialsEncrypted)
		if err == nil {
			var creds map[string]string
			if json.Unmarshal(credsJSON, &creds) == nil {
				resp.Credentials = creds
				resp.ConnectionString = buildConnectionString(db, creds)
			}
		}
	}

	// Read-only managed-volume info (best-effort). Name matches the provision
	// convention; size comes from the runtime's disk-usage view.
	volName := db.Slug + "-vol"
	if size, err := h.runtime.VolumeSize(r.Context(), volName); err == nil {
		resp.Volume = &volumeInfo{Name: volName, SizeBytes: size}
	}

	// External-access (SSH tunnel) state — enabled when a loopback host port is
	// bound. SSH host/user come from config and are presentation-only hints.
	resp.ExternalAccess = &externalAccessInfo{
		Enabled:  db.HostPort.Valid,
		HostPort: db.HostPort.Int32,
		SSHHost:  h.cfg.ServerSSHHost,
		SSHUser:  h.cfg.ServerSSHUser,
	}

	writeJSON(w, http.StatusOK, resp)
}

type externalAccessRequest struct {
	Enabled bool `json:"enabled"`
}

// SetDatabaseExternalAccess enables or disables SSH-tunnel external access by
// recreating the database container with/without a loopback host-port binding.
// The recreate runs as an async task; the database is marked transitional
// (creating) so the UI reflects the brief downtime.
func (h *Handler) SetDatabaseExternalAccess(w http.ResponseWriter, r *http.Request) {
	databaseID := chi.URLParam(r, "databaseId")
	var dbUUID pgtype.UUID
	if err := dbUUID.Scan(databaseID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid database id")
		return
	}

	if !h.canAccessDatabase(r, dbUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req externalAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	db, err := h.queries.GetDatabase(r.Context(), dbUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if db.Status != status.DatabaseRunning {
		writeError(w, http.StatusConflict, "database must be running to change external access")
		return
	}
	// Already in the requested state — nothing to recreate.
	if db.HostPort.Valid == req.Enabled {
		writeJSON(w, http.StatusOK, map[string]string{"status": db.Status})
		return
	}

	// Mark transitional before the async recreate so the UI shows progress.
	if _, err := h.queries.UpdateDatabaseStatus(r.Context(), generated.UpdateDatabaseStatusParams{
		ID:     dbUUID,
		Status: status.DatabaseCreating,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update database")
		return
	}

	payload, err := json.Marshal(map[string]any{"database_id": databaseID, "enable": req.Enabled})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create reconfigure task")
		return
	}
	if _, err := h.asynq.Enqueue(asynq.NewTask("reconfigure_db", payload), asynq.Queue("critical")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue reconfigure task")
		return
	}

	h.audit(r, "update_database", "database", databaseID, map[string]any{
		"external_access": req.Enabled,
	})

	writeJSON(w, http.StatusAccepted, map[string]string{"status": status.DatabaseCreating})
}

type updateDatabaseRequest struct {
	CPULimit    float64 `json:"cpu_limit"`
	MemoryLimit int64   `json:"memory_limit"`
}

// UpdateDatabase edits a managed database's resource limits (CPU cores / memory
// bytes) and applies them live to the running container. The limit is persisted
// regardless, so it also takes effect if the container is later recreated.
func (h *Handler) UpdateDatabase(w http.ResponseWriter, r *http.Request) {
	databaseID := chi.URLParam(r, "databaseId")
	var dbUUID pgtype.UUID
	if err := dbUUID.Scan(databaseID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid database id")
		return
	}

	if !h.canAccessDatabase(r, dbUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req updateDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CPULimit < 0 || req.MemoryLimit < 0 {
		writeError(w, http.StatusBadRequest, "cpu_limit and memory_limit must be non-negative")
		return
	}

	db, err := h.queries.UpdateDatabaseResources(r.Context(), generated.UpdateDatabaseResourcesParams{
		ID:          dbUUID,
		CpuLimit:    req.CPULimit,
		MemoryLimit: req.MemoryLimit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update database")
		return
	}

	// Apply live; a failure here is non-fatal because the persisted limit will
	// be honoured the next time the container is (re)created.
	if db.Status == status.DatabaseRunning && db.InternalHost.Valid {
		if err := h.runtime.UpdateContainerResources(r.Context(), db.InternalHost.String, req.CPULimit, req.MemoryLimit); err != nil {
			slog.Warn("failed to apply database resource limits live", "database_id", databaseID, "error", err)
		}
	}

	h.audit(r, "update_database", "database", databaseID, map[string]any{
		"cpu_limit":    req.CPULimit,
		"memory_limit": req.MemoryLimit,
	})

	writeJSON(w, http.StatusOK, db)
}

// databaseBackupEnabled reports whether a database can be backed up: known
// engines have a logical-dump tool; "other" supports backup when a backup_mode
// (volume_snapshot or command) was configured. redis (cache) is not supported.
func databaseBackupEnabled(db generated.Database) bool {
	switch db.Type {
	case "postgres", "mysql", "mongo":
		return true
	case "other":
		return db.BackupMode == "volume_snapshot" || db.BackupMode == "command"
	}
	return false
}

type databaseBackupResponse struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	SizeBytes  int64      `json:"size_bytes"`
	HasRemote  bool       `json:"has_remote"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
}

func toBackupResponse(b generated.DatabaseBackup) databaseBackupResponse {
	resp := databaseBackupResponse{
		ID:        uuidToString(b.ID),
		Status:    b.Status,
		SizeBytes: b.SizeBytes,
		HasRemote: b.RemoteKey.Valid,
		StartedAt: b.StartedAt.Time,
	}
	if b.FinishedAt.Valid {
		t := b.FinishedAt.Time
		resp.FinishedAt = &t
	}
	if b.Error.Valid {
		resp.Error = b.Error.String
	}
	return resp
}

// ListDatabaseBackups returns the recent backup runs for a database (newest first).
func (h *Handler) ListDatabaseBackups(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "databaseId")
	var dbUUID pgtype.UUID
	if err := dbUUID.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid database id")
		return
	}
	if !h.canAccessDatabase(r, dbUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	backups, err := h.queries.ListDatabaseBackups(r.Context(), generated.ListDatabaseBackupsParams{
		DatabaseID: dbUUID,
		Limit:      50,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list backups")
		return
	}

	resp := make([]databaseBackupResponse, 0, len(backups))
	for _, b := range backups {
		resp = append(resp, toBackupResponse(b))
	}
	writeJSON(w, http.StatusOK, resp)
}

// BackupDatabase enqueues an online logical-dump backup of a running database.
func (h *Handler) BackupDatabase(w http.ResponseWriter, r *http.Request) {
	databaseID := chi.URLParam(r, "databaseId")
	var dbUUID pgtype.UUID
	if err := dbUUID.Scan(databaseID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid database id")
		return
	}
	if !h.canAccessDatabase(r, dbUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	db, err := h.queries.GetDatabase(r.Context(), dbUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if !databaseBackupEnabled(db) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("logical backup is not supported for %s", db.Type))
		return
	}
	if db.Status != status.DatabaseRunning {
		writeError(w, http.StatusConflict, "database must be running to back up")
		return
	}

	payload, err := json.Marshal(map[string]any{"database_id": databaseID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backup task")
		return
	}
	if _, err := h.asynq.Enqueue(asynq.NewTask("backup_db", payload), asynq.Queue("default")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue backup task")
		return
	}

	h.audit(r, "backup_database", "database", databaseID, nil)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "running"})
}

type restoreDatabaseRequest struct {
	BackupID string `json:"backup_id"`
}

// RestoreDatabase enqueues a restore of a database from one of its recorded backups.
func (h *Handler) RestoreDatabase(w http.ResponseWriter, r *http.Request) {
	databaseID := chi.URLParam(r, "databaseId")
	var dbUUID pgtype.UUID
	if err := dbUUID.Scan(databaseID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid database id")
		return
	}
	if !h.canAccessDatabase(r, dbUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req restoreDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var backupUUID pgtype.UUID
	if err := backupUUID.Scan(req.BackupID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup id")
		return
	}

	db, err := h.queries.GetDatabase(r.Context(), dbUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if !databaseBackupEnabled(db) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("restore is not supported for %s", db.Type))
		return
	}
	if db.Status != status.DatabaseRunning {
		writeError(w, http.StatusConflict, "database must be running to restore")
		return
	}

	backup, err := h.queries.GetDatabaseBackup(r.Context(), backupUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	if backup.DatabaseID != db.ID {
		writeError(w, http.StatusBadRequest, "backup does not belong to this database")
		return
	}
	if backup.Status != "succeeded" {
		writeError(w, http.StatusConflict, "only a succeeded backup can be restored")
		return
	}

	payload, err := json.Marshal(map[string]any{"database_id": databaseID, "backup_id": req.BackupID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create restore task")
		return
	}
	if _, err := h.asynq.Enqueue(asynq.NewTask("restore_db", payload), asynq.Queue("critical")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue restore task")
		return
	}

	h.audit(r, "restore_database", "database", databaseID, map[string]any{"backup_id": req.BackupID})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "restoring"})
}

func (h *Handler) DeleteDatabase(w http.ResponseWriter, r *http.Request) {
	databaseID := chi.URLParam(r, "databaseId")
	var dbUUID pgtype.UUID
	if err := dbUUID.Scan(databaseID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid database id")
		return
	}

	if !h.canAccessDatabase(r, dbUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	if err := h.dbService.Delete(r.Context(), dbUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete database")
		return
	}

	h.audit(r, "delete_database", "database", databaseID, nil)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func buildConnectionString(db generated.Database, creds map[string]string) string {
	host := db.InternalHost.String
	port := db.InternalPort.Int32

	switch db.Type {
	case "postgres":
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s",
			creds["user"], creds["password"], host, port, creds["database"])
	case "mysql":
		return fmt.Sprintf("mysql://%s:%s@%s:%d/%s",
			creds["user"], creds["password"], host, port, creds["database"])
	case "redis":
		return fmt.Sprintf("redis://:%s@%s:%d",
			creds["password"], host, port)
	case "mongo":
		return fmt.Sprintf("mongodb://%s:%s@%s:%d",
			creds["username"], creds["password"], host, port)
	default:
		return ""
	}
}
