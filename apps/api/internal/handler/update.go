package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/weiliang79/belune/internal/runtime"
	"github.com/weiliang79/belune/internal/store/generated"
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

	helperID, err := rt.SpawnUpdateHelper(r.Context(), runtime.UpdateHelperConfig{
		Image:      image,
		WorkingDir: workingDir,
		Version:    target,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to start the update: %v", err))
		return
	}

	// Record the attempt so GetSelfUpdateStatus can report on it afterwards.
	// Without this a helper that dies on its first line is invisible: this
	// endpoint answers 202 the moment the container is CREATED, never waiting to
	// see whether the script survived, so the dashboard would say "started" and
	// then show nothing at all, forever.
	//
	// Deliberately absent from updatableSettings, like the update_latest_* cache
	// it sits beside — a token must not be able to forge or erase the record of
	// an update attempt.
	h.setSetting(r.Context(), settingUpdateHelperID, helperID)
	h.setSetting(r.Context(), settingUpdateHelperTarget, target)
	h.setSetting(r.Context(), settingUpdateHelperStartedAt, time.Now().UTC().Format(time.RFC3339))

	h.audit(r, "trigger_update", "platform", "", map[string]any{
		"from": version.Version,
		"to":   target,
	})
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "started",
		"target": target,
	})
}

// The last update attempt, written by TriggerSelfUpdate and read by
// GetSelfUpdateStatus. Cache keys owned by this package, never writable through
// PUT /api/settings — see updatableSettings.
const (
	settingUpdateHelperID        = "update_helper_id"
	settingUpdateHelperTarget    = "update_helper_target"
	settingUpdateHelperStartedAt = "update_helper_started_at"
)

// GetSelfUpdateStatus reports what became of the last update this dashboard
// started, so a helper that failed is visible somewhere other than
// `docker logs` on a container the operator does not know exists.
//
// ⚠️ A SUCCESSFUL update also leaves an exited helper behind, so "exited" alone
// cannot mean failure. The version decides: once this build reports the version
// the attempt targeted, the update landed — and since a successful update
// replaces this container, the process answering here IS the new build. Only an
// exited helper with the version still unchanged is a failure.
//
// GET /api/maintenance/update/status (admin, session-only)
//
//apidoc:tag platform/maintenance
//apidoc:title Get Update Status
func (h *Handler) GetSelfUpdateStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := map[string]any{"state": "idle"}

	helperID := h.settingValue(ctx, settingUpdateHelperID)
	if helperID == "" {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	target := h.settingValue(ctx, settingUpdateHelperTarget)
	resp["target"] = target
	if startedAt := h.settingValue(ctx, settingUpdateHelperStartedAt); startedAt != "" {
		resp["started_at"] = startedAt
	}

	// Landed: this build is at or past what the attempt targeted. Clear the
	// record so a later failure cannot be confused with this success.
	current := "v" + strings.TrimPrefix(version.Version, "v")
	if target != "" && semver.IsValid(current) && semver.Compare("v"+target, current) <= 0 {
		h.clearUpdateAttempt(ctx)
		writeJSON(w, http.StatusOK, map[string]any{"state": "idle"})
		return
	}

	rt, err := h.runtimes.Local(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach the Docker host")
		return
	}
	all, err := rt.ListAllContainers(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach the Docker host")
		return
	}

	var helper *runtime.ContainerInfo
	for i := range all {
		if all[i].ID == helperID {
			helper = &all[i]
			break
		}
	}
	if helper == nil {
		// Reaped, pruned, or removed by hand — and the version never moved, so
		// whatever it did, it did not finish the job.
		resp["state"] = "failed"
		resp["reason"] = "The update helper is gone and this install is still on " + version.Version + "."
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if helper.Status == "running" || helper.Status == "created" {
		resp["state"] = "running"
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp["state"] = "failed"
	resp["reason"] = updateHelperFailureReason(ctx, rt, helperID)
	writeJSON(w, http.StatusOK, resp)
}

// updateHelperFailureReason pulls the tail of the helper's own output, which is
// the only place the actual cause appears — a missing script, an unreadable
// .env, a release that could not be fetched. Best-effort: a reason we cannot
// read must not turn the status call itself into an error, since "it failed" is
// already the useful half.
func updateHelperFailureReason(ctx context.Context, rt runtime.ContainerRuntime, helperID string) string {
	const fallback = "The update did not complete. Check the update helper's container logs on the host."
	rc, err := rt.ContainerLogsTail(ctx, helperID, 20)
	if err != nil {
		return fallback
	}
	defer rc.Close()

	out, err := io.ReadAll(io.LimitReader(rc, 8<<10))
	if err != nil {
		return fallback
	}
	lines := make([]string, 0, 3)
	for l := range strings.SplitSeq(string(out), "\n") {
		// Docker multiplexes logs with an 8-byte stream header per frame; strip
		// any leading non-printables so the message reads cleanly in a toast.
		l = strings.TrimSpace(strings.Map(func(r rune) rune {
			if r < 32 && r != '\t' {
				return -1
			}
			return r
		}, l))
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return fallback
	}
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.Join(lines, " · ")
}

func (h *Handler) clearUpdateAttempt(ctx context.Context) {
	for _, k := range []string{settingUpdateHelperID, settingUpdateHelperTarget, settingUpdateHelperStartedAt} {
		h.setSetting(ctx, k, "")
	}
}

// setSetting writes one settings row, logging rather than failing: every caller
// here is recording or clearing the update-attempt record, and losing that must
// not fail the request that was otherwise successful.
func (h *Handler) setSetting(ctx context.Context, key, value string) {
	if _, err := h.queries.UpsertSetting(ctx, generated.UpsertSettingParams{Key: key, Value: value}); err != nil {
		slog.Warn("update: failed to write setting", "key", key, "error", err)
	}
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
