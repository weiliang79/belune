package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type settingResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
