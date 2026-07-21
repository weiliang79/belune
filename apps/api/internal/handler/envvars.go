package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/store"
	"github.com/weiliang79/belune/internal/store/generated"
)

var envKeyRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// preparedEnvVar is one validated, already-encrypted variable ready to be
// written.
type preparedEnvVar struct {
	key       string
	encrypted []byte
	isSecret  bool
}

// prepareEnvVars validates every submitted key and encrypts every value before
// anything is written, returning a client error message on the first problem.
//
// Doing all of this up front is the point: these endpoints replace the whole
// set, so a failure discovered halfway through would otherwise leave the stored
// variables partially rewritten — or, in the project endpoint's original form,
// already deleted. Nothing here touches the database.
//
// Blank keys are skipped rather than rejected: the editor submits an empty row
// for the "add variable" affordance.
func prepareEnvVars(vars []envVarInput, encrypt func([]byte) ([]byte, error)) ([]preparedEnvVar, string) {
	out := make([]preparedEnvVar, 0, len(vars))
	seen := make(map[string]struct{}, len(vars))
	for _, v := range vars {
		if v.Key == "" {
			continue
		}
		if !envKeyRegex.MatchString(v.Key) {
			return nil, fmt.Sprintf("invalid env var key: %q", v.Key)
		}
		// A duplicate key would make the upsert order decide the winner, so
		// reject it instead of silently keeping the last one.
		if _, dup := seen[v.Key]; dup {
			return nil, fmt.Sprintf("duplicate env var key: %q", v.Key)
		}
		seen[v.Key] = struct{}{}

		encrypted, err := encrypt([]byte(v.Value))
		if err != nil {
			return nil, "failed to encrypt value"
		}
		out = append(out, preparedEnvVar{key: v.Key, encrypted: encrypted, isSecret: v.IsSecret})
	}
	return out, ""
}

// keysOf returns the keys to retain, for the delete-what-was-not-submitted step.
func keysOf(prepared []preparedEnvVar) []string {
	keys := make([]string, 0, len(prepared))
	for _, p := range prepared {
		keys = append(keys, p.key)
	}
	return keys
}

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

	if !h.canAccessApplication(r, applicationUUID) {
		writeError(w, http.StatusForbidden, "access denied")
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

	if !h.canAccessApplication(r, applicationUUID) {
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

	// The request carries the complete set, so anything missing from it was
	// deleted in the editor. Upserting alone left those rows behind, and the
	// deploy kept injecting them into the container — removing a variable in the
	// UI appeared to work but changed nothing.
	//
	// Upsert and delete run in one transaction so the stored set is never
	// observed half-applied: a failure partway through leaves the previous set
	// intact rather than a mixture of old and new.
	if err := store.WithTx(r.Context(), h.db, func(q *generated.Queries) error {
		for _, p := range prepared {
			if _, err := q.UpsertEnvVar(r.Context(), generated.UpsertEnvVarParams{
				ApplicationID:  applicationUUID,
				Key:            p.key,
				ValueEncrypted: p.encrypted,
				IsSecret:       p.isSecret,
			}); err != nil {
				return err
			}
		}
		return q.DeleteEnvVarsNotIn(r.Context(), generated.DeleteEnvVarsNotInParams{
			ApplicationID: applicationUUID,
			Keys:          keysOf(prepared),
		})
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save env vars")
		return
	}

	h.markConfigChanged(r.Context(), applicationUUID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
