package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/weiling79/belune/internal/store/generated"
)

type settingResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// defaultInstanceName is used when the operator has not set one. It seeds both
// the GitHub App manifest name and the dashboard brand.
const defaultInstanceName = "Self-Hosted PaaS"

// instanceName returns the configured "instance_name" setting, or the default
// when unset/blank.
func (h *Handler) instanceName(ctx context.Context) string {
	s, err := h.queries.GetSetting(ctx, "instance_name")
	if err != nil || strings.TrimSpace(s.Value) == "" {
		return defaultInstanceName
	}
	return strings.TrimSpace(s.Value)
}

func (h *Handler) ListSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.queries.ListSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list settings")
		return
	}

	result := make([]settingResponse, 0, len(settings))
	for _, s := range settings {
		result = append(result, settingResponse{
			Key:   s.Key,
			Value: s.Value,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req []settingResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, s := range req {
		if s.Key == "" {
			continue
		}
		if _, err := h.queries.UpsertSetting(r.Context(), generated.UpsertSettingParams{
			Key:   s.Key,
			Value: s.Value,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update setting")
			return
		}
	}

	h.audit(r, "update_settings", "settings", "", nil)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
