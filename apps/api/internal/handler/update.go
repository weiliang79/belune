package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/weiliang79/belune/internal/runtime"
	"github.com/weiliang79/belune/internal/version"
)

// TriggerSelfUpdate applies the update the Server-page card is currently
// showing, by spawning a detached helper container that runs
// scripts/update.sh against the host install directory. The helper survives
// this container being replaced — that IS the point: `docker compose up -d`
// inside it is what stops this process partway through handling whatever
// request comes after this one's response.
//
// Triple-gated like the host shell: admin + session (route group), step-up
// re-auth (password + a second-factor code for anyone who has one enrolled),
// and a manifest check that refuses outright when the target release needs a
// host-side action the container cannot perform on its own.
// POST /api/maintenance/update (admin, session-only)
//
//apidoc:tag platform/maintenance
func (h *Handler) TriggerSelfUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		Method   string `json:"method"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
		writeError(w, http.StatusBadRequest, "password required")
		return
	}
	if _, ok := h.stepUpReauth(w, r, req.Password, req.Method, req.Code); !ok {
		return
	}

	// Resolved from the update-check worker's cache (worker/update_check_task.go),
	// never re-resolved here — the operator updates to exactly the version the
	// card showed them, not whatever happens to be latest by the time they click.
	target := strings.TrimSpace(h.settingValue(r.Context(), "update_latest_version"))
	if target == "" {
		writeError(w, http.StatusBadRequest, "no update available")
		return
	}

	current := "v" + strings.TrimPrefix(version.Version, "v")
	if !semver.IsValid(current) {
		writeError(w, http.StatusBadRequest, "this build has no version to update from")
		return
	}
	if semver.Compare("v"+target, current) <= 0 {
		writeError(w, http.StatusBadRequest, "already up to date")
		return
	}

	if h.settingValue(r.Context(), "update_latest_requires_host_update") == "true" {
		writeError(w, http.StatusBadRequest,
			"this release changes host-level configuration and cannot be applied from the dashboard — run scripts/update.sh on the host instead")
		return
	}

	rt, err := h.runtimes.Local(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach the Docker host")
		return
	}
	image := h.selfImage(r.Context())
	if image == "" {
		writeError(w, http.StatusInternalServerError, "could not determine this container's own image")
		return
	}
	workingDir, err := h.selfWorkingDir(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := rt.SpawnUpdateHelper(r.Context(), runtime.UpdateHelperConfig{
		Image:      image,
		WorkingDir: workingDir,
		Version:    target,
	}); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to start the update: %v", err))
		return
	}

	h.audit(r, "trigger_update", "platform", "", map[string]any{
		"from": version.Version,
		"to":   target,
	})
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "started",
		"target": target,
	})
}

// settingValue reads one setting's trimmed value, or "" if it is unset or the
// read fails — every caller here treats both the same way (fall through to the
// next gate), so there is no reason to distinguish them.
func (h *Handler) settingValue(ctx context.Context, key string) string {
	s, err := h.queries.GetSetting(ctx, key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(s.Value)
}
