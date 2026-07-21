package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/store"
	"github.com/weiliang79/belune/internal/store/generated"
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

	prepared, errMsg := prepareEnvVars(req.Vars, h.cfg.Keyring.Encrypt)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// This used to delete every variable first and validate afterwards, so a
	// single bad key — or a failure anywhere in the loop — returned an error
	// having already wiped the project's variables, with no transaction to undo
	// it. Validation and encryption now happen before any write, and the
	// replacement runs in one transaction.
	if err := store.WithTx(r.Context(), h.db, func(q *generated.Queries) error {
		for _, p := range prepared {
			if _, err := q.UpsertProjectEnvVar(r.Context(), generated.UpsertProjectEnvVarParams{
				ProjectID:      projectUUID,
				Key:            p.key,
				ValueEncrypted: p.encrypted,
				IsSecret:       p.isSecret,
			}); err != nil {
				return err
			}
		}
		return q.DeleteProjectEnvVarsNotIn(r.Context(), generated.DeleteProjectEnvVarsNotInParams{
			ProjectID: projectUUID,
			Keys:      keysOf(prepared),
		})
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save project env vars")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
