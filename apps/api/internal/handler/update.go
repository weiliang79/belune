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

	// ⚠️ Refuse a second concurrent run. Nothing else stops one: the helper is
	// created with an empty name so Docker generates a fresh one every time, and
	// every gate above passes identically on a second click — the cached target
	// has not changed and this container has not been replaced yet.
	//
	// It is easy to reach rather than a tight race. The card keeps offering
	// "Update now" after the 202 (updateAvailable is derived from the settings
	// cache, which nothing here changes), and update.sh takes minutes — it runs
	// a full pre-update backup between reading .env and rewriting it. Two runs
	// inside that window compute the SAME backup filenames from the same
	// CURRENT_VERSION (.env.backup-<v>, .infra-backup-<v>), so the second
	// clobbers the first — and those are the files update.sh tells the operator
	// are their way back.
	//
	// Same shape and status as "a backup is already in progress" (backups.go)
	// and "a deployment is already in progress" (applications.go).
	if running, err := updateHelperRunning(r.Context(), rt); err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach the Docker host")
		return
	} else if running {
		writeError(w, http.StatusConflict, "an update is already in progress")
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

// updateHelperRunning reports whether a self-update helper is still at work.
//
// Matches on LabelUpdateHelper rather than LabelHelper: backup, restore and
// snapshot helpers all carry the latter, and none of them should block an
// update. Status mirrors isRunningHelper's own definition in the orphan sweep
// (cleanup_task.go) — "created" counts, because a helper that has been created
// but not yet started is about to run, not finished.
//
// An exited helper never blocks: the update either finished (this container was
// replaced, so nothing here is running anyway) or failed, and a failed update
// must not lock the operator out of retrying from the dashboard.
func updateHelperRunning(ctx context.Context, rt runtime.ContainerRuntime) (bool, error) {
	all, err := rt.ListAllContainers(ctx)
	if err != nil {
		return false, err
	}
	for _, c := range all {
		if c.Labels[runtime.LabelUpdateHelper] != "true" {
			continue
		}
		if c.Status == "running" || c.Status == "created" {
			return true, nil
		}
	}
	return false, nil
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
