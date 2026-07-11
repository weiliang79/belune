package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/weiliang79/belune/internal/proxy"
	"github.com/weiliang79/belune/internal/store/generated"
)

type settingResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// defaultInstanceName is used when the operator has not set one. It seeds both
// the GitHub App manifest name and the dashboard brand.
const defaultInstanceName = "Belune"

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

	// The dashboard domain is not just a stored string: it decides the hostname
	// Caddy obtains a certificate for, so it is validated before it is written and
	// applied to the proxy afterwards.
	dashboardDomain, changingDashboard := "", false
	for i, s := range req {
		if s.Key != proxy.SettingDashboardDomain {
			continue
		}
		host := strings.TrimSpace(s.Value)
		if host != "" && !hostnameRegex.MatchString(host) {
			writeError(w, http.StatusBadRequest, "invalid dashboard domain: must be a hostname such as belune.example.com")
			return
		}
		req[i].Value = host
		dashboardDomain, changingDashboard = host, true
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

	if changingDashboard {
		// Publish (or clear) the dashboard's own route. A failure here leaves the
		// setting saved but the proxy unchanged; the reconciler re-applies it on its
		// next pass, so report it rather than pretending nothing happened.
		if err := h.proxy.SetDashboardRoute(r.Context(), dashboardDomain); err != nil {
			slog.Error("failed to apply dashboard domain to proxy", "hostname", dashboardDomain, "error", err)
			writeError(w, http.StatusInternalServerError, "domain saved, but the proxy could not be updated — it will retry shortly")
			return
		}
	}

	h.audit(r, "update_settings", "settings", "", nil)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
