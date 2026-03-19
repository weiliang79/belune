package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type envVarResponse struct {
	ID       pgtype.UUID `json:"id"`
	Key      string      `json:"key"`
	Value    string      `json:"value,omitempty"`
	IsSecret bool        `json:"is_secret"`
}

func (h *Handler) ListEnvVars(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	envVars, err := h.queries.ListEnvVarsByApplication(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list env vars")
		return
	}

	result := make([]envVarResponse, 0, len(envVars))
	for _, ev := range envVars {
		resp := envVarResponse{
			ID:       ev.ID,
			Key:      ev.Key,
			IsSecret: ev.IsSecret,
		}

		// Decrypt and show non-secret values; mask secrets
		if !ev.IsSecret {
			decrypted, err := crypto.Decrypt(ev.ValueEncrypted, h.cfg.EncryptionKey)
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

type updateEnvVarsRequest struct {
	Vars []envVarInput `json:"vars"`
}

type envVarInput struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

func (h *Handler) UpdateEnvVars(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	var req updateEnvVarsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, v := range req.Vars {
		if v.Key == "" {
			continue
		}

		encrypted, err := crypto.Encrypt([]byte(v.Value), h.cfg.EncryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt value")
			return
		}

		_, err = h.queries.UpsertEnvVar(r.Context(), generated.UpsertEnvVarParams{
			ApplicationID: applicationUUID,
			Key:            v.Key,
			ValueEncrypted: encrypted,
			IsSecret:       v.IsSecret,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save env var")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
