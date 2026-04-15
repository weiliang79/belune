package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

func (h *Handler) ListProjectEnvVars(w http.ResponseWriter, r *http.Request) {
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

	envVars, err := h.queries.ListProjectEnvVars(r.Context(), projectUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list project env vars")
		return
	}

	result := make([]envVarResponse, 0, len(envVars))
	for _, ev := range envVars {
		resp := envVarResponse{
			ID:       ev.ID,
			Key:      ev.Key,
			IsSecret: ev.IsSecret,
		}

		if !ev.IsSecret {
			decrypted, err := h.cfg.Keyring.Decrypt(ev.ValueEncrypted)
			if err == nil {
				resp.Value = string(decrypted)
			}
		} else {
			resp.Value = "••••••••"
		}

		result = append(result, resp)
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) UpdateProjectEnvVars(w http.ResponseWriter, r *http.Request) {
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

	var req updateEnvVarsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Delete all existing project env vars, then upsert the new set
	if err := h.queries.DeleteProjectEnvVarsByProject(r.Context(), projectUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear project env vars")
		return
	}

	for _, v := range req.Vars {
		if v.Key == "" {
			continue
		}
		if !envKeyRegex.MatchString(v.Key) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid env var key: %q", v.Key))
			return
		}

		encrypted, err := h.cfg.Keyring.Encrypt([]byte(v.Value))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt value")
			return
		}

		_, err = h.queries.UpsertProjectEnvVar(r.Context(), generated.UpsertProjectEnvVarParams{
			ProjectID:      projectUUID,
			Key:            v.Key,
			ValueEncrypted: encrypted,
			IsSecret:       v.IsSecret,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save project env var")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
