package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/robfig/cron/v3"
	"strings"

	"github.com/weiliang79/belune/internal/config"
	"github.com/weiliang79/belune/internal/proxy"
	"github.com/weiliang79/belune/internal/service"
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
		// The SMTP password is keyring-encrypted and managed via the dedicated
		// /api/settings/smtp endpoints — never expose it in the generic listing.
		if s.Key == service.SettingSMTPPassword {
			continue
		}
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

	// The dashboard settings are not just stored strings: they decide the hostname
	// Caddy obtains a certificate for, and how. Validate before writing, then
	// apply to the proxy — a bad value here takes HTTPS down on the panel itself.
	changingDashboard := false
	for i, s := range req {
		switch s.Key {
		case proxy.SettingDashboardDomain:
			host := strings.TrimSpace(s.Value)
			if host != "" && !hostnameRegex.MatchString(host) {
				writeError(w, http.StatusBadRequest, "invalid dashboard domain: must be a hostname such as belune.example.com")
				return
			}
			req[i].Value = host
			changingDashboard = true

		case proxy.SettingDashboardSSLMode:
			mode := strings.TrimSpace(s.Value)
			if !proxy.ValidSSLMode(mode) {
				writeError(w, http.StatusBadRequest, "invalid TLS mode: expected automatic, custom, or off")
				return
			}
			req[i].Value = mode
			changingDashboard = true

		case proxy.SettingDashboardCertificateID:
			id := strings.TrimSpace(s.Value)
			// A certificate that does not exist would leave the dashboard with
			// nothing to serve on :443 — refuse it here rather than discover it in
			// the reconciler, where the operator would never see the reason.
			if id != "" {
				var uid pgtype.UUID
				if err := uid.Scan(id); err != nil {
					writeError(w, http.StatusBadRequest, "invalid certificate id")
					return
				}
				if _, err := h.queries.GetCertificate(r.Context(), uid); err != nil {
					writeError(w, http.StatusBadRequest, "the selected certificate no longer exists")
					return
				}
			}
			req[i].Value = id
			changingDashboard = true

		case config.SettingPublicIP:
			// Blank clears the override (fall back to env/autodetect). A non-blank
			// value must parse as an IP — a garbage baseline is worse than none, as
			// it would mark every domain as pointing at "not this server".
			ip := strings.TrimSpace(s.Value)
			if ip != "" && net.ParseIP(ip) == nil {
				writeError(w, http.StatusBadRequest, "invalid server IP: must be an IPv4 or IPv6 address")
				return
			}
			req[i].Value = ip

		case config.SettingControlPlaneBackupSchedule:
			// Blank falls back to config.DefaultControlPlaneBackupSchedule (see the
			// worker sweep) — only validate when the operator sets one explicitly.
			sched := strings.TrimSpace(s.Value)
			if sched != "" {
				if _, err := cron.ParseStandard(sched); err != nil {
					writeError(w, http.StatusBadRequest, "invalid cron schedule")
					return
				}
			}
			req[i].Value = sched

		case config.SettingControlPlaneBackupRetainDays:
			v, errMsg := validateRetentionSetting(s.Value, 1, 3650)
			if errMsg != "" {
				writeError(w, http.StatusBadRequest, "invalid retention days: "+errMsg)
				return
			}
			req[i].Value = v

		case config.SettingControlPlaneBackupRetainCount:
			v, errMsg := validateRetentionSetting(s.Value, 1, 1000)
			if errMsg != "" {
				writeError(w, http.StatusBadRequest, "invalid retention count: "+errMsg)
				return
			}
			req[i].Value = v
		}
	}

	// The three dashboard settings only make sense together, and a request may
	// carry any subset of them — so resolve what the combination *will* be and
	// judge that, rather than each field in isolation.
	domain, mode, certID := h.effectiveDashboardTLS(r, req)
	if changingDashboard && domain != "" && mode == proxy.SSLModeCustom && certID == "" {
		writeError(w, http.StatusBadRequest, "choose a certificate to serve, or switch the TLS mode to automatic")
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

	if changingDashboard {
		// A full reconcile, not just the route: the mode also decides the auto-HTTPS
		// skip lists and which certificate is loaded, and those are the reconciler's
		// to write. Setting only the route would leave a window — up to a whole
		// reconcile interval — where the dashboard force-redirects to HTTPS while
		// still being skipped for certificates, which a browser shows as an error.
		//
		// Fall back to the route alone if there is no reconciler (tests), and treat
		// either failure as "saved, but not yet applied": the periodic pass fixes it.
		var err error
		if h.reconciler != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()
			err = h.reconciler.ReconcileNow(ctx)
		} else {
			err = h.proxy.SetDashboardRoute(r.Context(), domain, mode)
		}
		if err != nil {
			slog.Error("failed to apply dashboard settings to proxy", "hostname", domain, "ssl_mode", mode, "error", err)
			writeError(w, http.StatusInternalServerError, "settings saved, but the proxy could not be updated — it will retry shortly")
			return
		}
	}

	h.audit(r, "update_settings", "settings", "", nil)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// validateRetentionSetting trims and validates a retention-knob value. Blank
// falls back to the .env default (see config.SettingControlPlaneBackupRetain*),
// so only a non-blank value is range-checked.
func validateRetentionSetting(raw string, min, max int) (value, errMsg string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}
	n, err := strconv.Atoi(trimmed)
	if err != nil || n < min || n > max {
		return "", fmt.Sprintf("must be a whole number between %d and %d", min, max)
	}
	return trimmed, ""
}

// effectiveDashboardTLS resolves what the dashboard's TLS settings will be once
// this request is written: the stored values, with anything in the request
// overriding them. A PATCH-shaped API that only ever sees a subset of the keys
// cannot otherwise tell whether "mode = custom" is about to be left without a
// certificate.
func (h *Handler) effectiveDashboardTLS(r *http.Request, req []settingResponse) (domain, mode, certID string) {
	read := func(key string) string {
		if s, err := h.queries.GetSetting(r.Context(), key); err == nil {
			return strings.TrimSpace(s.Value)
		}
		return ""
	}
	domain = read(proxy.SettingDashboardDomain)
	mode = read(proxy.SettingDashboardSSLMode)
	certID = read(proxy.SettingDashboardCertificateID)

	for _, s := range req {
		switch s.Key {
		case proxy.SettingDashboardDomain:
			domain = s.Value
		case proxy.SettingDashboardSSLMode:
			mode = s.Value
		case proxy.SettingDashboardCertificateID:
			certID = s.Value
		}
	}

	// Absent means automatic: that is what every install did before the mode
	// existed, so an upgrade keeps serving the certificate it already has.
	if mode == "" {
		mode = proxy.SSLModeAutomatic
	}
	return domain, mode, certID
}
