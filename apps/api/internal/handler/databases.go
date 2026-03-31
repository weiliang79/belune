package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
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
	Name        string                    `json:"name"`
	Slug        string                    `json:"slug"`
	Type        string                    `json:"type"`
	Version     string                    `json:"version"`
	Credentials *createDatabaseCredentials `json:"credentials"`
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

type databaseResponse struct {
	generated.Database
	Credentials      map[string]string `json:"credentials,omitempty"`
	ConnectionString string            `json:"connection_string,omitempty"`
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
	if _, ok := defaultVersions[req.Type]; !ok {
		writeError(w, http.StatusBadRequest, "type must be one of: postgres, mysql, redis, mongo")
		return
	}

	// Apply default version if empty
	if req.Version == "" {
		req.Version = defaultVersions[req.Type]
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

	// Build credentials based on type
	creds := make(map[string]string)
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

	credsJSON, err := json.Marshal(creds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal credentials")
		return
	}

	encrypted, err := crypto.Encrypt(credsJSON, h.cfg.EncryptionKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt credentials")
		return
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
			Status:               "creating",
			InternalHost:         pgtype.Text{},
			InternalPort:         pgtype.Int4{},
			CredentialsEncrypted: encrypted,
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
	payload, _ := json.Marshal(provisionDBPayload{DatabaseID: dbIDStr})
	task := asynq.NewTask("provision_db", payload)
	if _, err := h.asynq.Enqueue(task, asynq.Queue("critical")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue provision task")
		return
	}

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
		credsJSON, err := crypto.Decrypt(db.CredentialsEncrypted, h.cfg.EncryptionKey)
		if err == nil {
			var creds map[string]string
			if json.Unmarshal(credsJSON, &creds) == nil {
				resp.Credentials = creds
				resp.ConnectionString = buildConnectionString(db, creds)
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
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

	// Fetch database to get slug for container name
	db, err := h.queries.GetDatabase(r.Context(), dbUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	// Stop and remove container + volume
	if err := h.runtime.StopContainer(r.Context(), db.Slug); err != nil {
		slog.Warn("could not stop container during db deletion", "container", db.Slug, "error", err)
	}
	if err := h.runtime.RemoveContainer(r.Context(), db.Slug); err != nil {
		slog.Warn("could not remove container during db deletion", "container", db.Slug, "error", err)
	}
	if err := h.runtime.RemoveVolume(r.Context(), db.Slug+"-vol"); err != nil {
		slog.Warn("could not remove volume during db deletion", "volume", db.Slug+"-vol", "error", err)
	}

	if err := h.queries.DeleteDatabase(r.Context(), dbUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete database")
		return
	}

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
