package handler_test

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/runtime"
	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/testutil"
	"github.com/weiliang79/belune/internal/version"
)

// setUpdateSetting seeds one settings row directly (bypassing PUT /api/settings
// — update_latest_* are deliberately absent from its allowlist) and schedules
// its removal. resetDB(t)'s TruncateAll does not clear the settings table, so
// without this a value seeded here would leak into whatever test runs next.
func setUpdateSetting(t *testing.T, key, value string) {
	t.Helper()
	ctx := context.Background()
	_, err := env.Queries.UpsertSetting(ctx, generated.UpsertSettingParams{Key: key, Value: value})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Queries.UpsertSetting(context.Background(), generated.UpsertSettingParams{Key: key, Value: ""})
	})
}

// clearUpdateAttemptSettings removes the update-attempt record TriggerSelfUpdate
// writes whenever it actually spawns a helper.
//
// ⚠️ Any test that reaches the spawn path needs this. resetDB's TruncateAll does
// not clear the settings table (see setUpdateSetting), so the stored helper id
// survives into the next test, which then sees a stale attempt and reports
// "failed" where it expected "idle". Two tests reach that path and neither is
// obviously about settings — the conflict test's own "does NOT block" subtests
// succeed on purpose.
func clearUpdateAttemptSettings(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, k := range []string{"update_helper_id", "update_helper_target", "update_helper_started_at"} {
			_, _ = env.Queries.UpsertSetting(context.Background(),
				generated.UpsertSettingParams{Key: k, Value: ""})
		}
	})
}

// withVersion overrides the running build's reported version for the duration
// of the test — go test never sets the ldflags-stamped version.Version, so it
// is "dev" (an invalid semver) by default, and the "already up to date" /
// "spawns the helper" cases need a real one to compare against.
func withVersion(t *testing.T, v string) {
	t.Helper()
	old := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = old })
}

// selfContainerIDForTest mirrors handler.selfContainerID()'s unexported logic
// (platform.go) so the fixture container this test seeds into the mock runtime
// carries the SAME id the handler will independently look up at request time —
// whether that's a real container id (running in the devcontainer, where this
// test suite normally runs) or the hostname fallback (a bare `go test`).
var selfContainerIDRe = regexp.MustCompile(`/containers/([0-9a-f]{64})`)

func selfContainerIDForTest(t *testing.T) string {
	t.Helper()
	if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		if m := selfContainerIDRe.FindSubmatch(data); m != nil {
			return string(m[1])
		}
	}
	host, err := os.Hostname()
	require.NoError(t, err)
	return host
}

func TestTriggerSelfUpdate_RequiresPassword(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/maintenance/update", map[string]string{},
		testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// TestTriggerSelfUpdate_RequiresSecondFactorWhenEnrolled mirrors
// TestHostShell_RequiresTheSecondFactorWhenEnrolled: this ends with the
// control-plane container replacing itself, at least as privileged as the host
// shell, so the same step-up shape (password alone is not enough once a second
// factor is enrolled) applies.
func TestTriggerSelfUpdate_RequiresSecondFactorWhenEnrolled(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	secret, _, token := enableTOTP(t, token)

	resp := env.DoRequest(t, "POST", "/api/maintenance/update", map[string]string{
		"password": "password123",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"password-only step-up must not trigger an update for a 2FA user")
	assert.Contains(t, testutil.ReadJSON(t, resp)["error"], "verification code")

	// With the code the gate passes — it then fails further along (no update
	// cached), which is exactly how far this test is meant to reach.
	resp = env.DoRequest(t, "POST", "/api/maintenance/update", map[string]string{
		"password": "password123", "code": loginCode(t, secret),
	}, testutil.AuthHeader(token))
	assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
		"a correct code must get past the step-up gate")
	resp.Body.Close()
}

func TestTriggerSelfUpdate_RefusesWhenNoUpdateCached(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/maintenance/update", map[string]string{
		"password": "password123",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, testutil.ReadJSON(t, resp)["error"], "no update available")
}

// TestTriggerSelfUpdate_RefusesADevBuild guards the trap a naive semver
// compare would fall into: "dev" (what every `go build` without the release
// ldflags reports) is not a valid semver, and comparing it anyway would either
// panic or silently treat every release as "not newer".
func TestTriggerSelfUpdate_RefusesADevBuild(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	setUpdateSetting(t, "update_latest_version", "9.9.9")

	resp := env.DoRequest(t, "POST", "/api/maintenance/update", map[string]string{
		"password": "password123",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, testutil.ReadJSON(t, resp)["error"], "no version to update from")
}

func TestTriggerSelfUpdate_RefusesWhenAlreadyUpToDate(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	withVersion(t, "v1.2.3")
	setUpdateSetting(t, "update_latest_version", "1.2.3")

	resp := env.DoRequest(t, "POST", "/api/maintenance/update", map[string]string{
		"password": "password123",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, testutil.ReadJSON(t, resp)["error"], "already up to date")
}

func TestTriggerSelfUpdate_RefusesWhenRequiresHostUpdate(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	withVersion(t, "v1.2.3")
	setUpdateSetting(t, "update_latest_version", "1.3.0")
	setUpdateSetting(t, "update_latest_requires_host_update", "true")

	resp := env.DoRequest(t, "POST", "/api/maintenance/update", map[string]string{
		"password": "password123",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, testutil.ReadJSON(t, resp)["error"], "run scripts/update.sh on the host")
}

// TestTriggerSelfUpdate_SpawnsHelperWithResolvedTarget is the happy path: past
// every gate, it resolves this container's own image and compose working
// directory and hands scripts/update.sh exactly the cached target version —
// never re-resolving "latest" itself.
func TestTriggerSelfUpdate_SpawnsHelperWithResolvedTarget(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	withVersion(t, "v1.2.3")
	setUpdateSetting(t, "update_latest_version", "1.3.0")
	setUpdateSetting(t, "update_latest_requires_host_update", "false")
	clearUpdateAttemptSettings(t)

	env.Runtime.ListAllContainers_ = []runtime.ContainerInfo{
		{
			ID:    selfContainerIDForTest(t),
			Image: "ghcr.io/weiliang79/belune:v1.2.3",
			Labels: map[string]string{
				"com.docker.compose.project.working_dir": "/opt/belune",
			},
		},
	}
	t.Cleanup(func() { env.Runtime.ListAllContainers_ = nil })

	resp := env.DoRequest(t, "POST", "/api/maintenance/update", map[string]string{
		"password": "password123",
	}, testutil.AuthHeader(token))
	body := testutil.ReadJSON(t, resp)
	require.Equal(t, http.StatusAccepted, resp.StatusCode, "%v", body)
	assert.Equal(t, "1.3.0", body["target"])

	require.Len(t, env.Runtime.SpawnUpdateHelperCalls, 1)
	call := env.Runtime.SpawnUpdateHelperCalls[0]
	assert.Equal(t, "1.3.0", call.Version)
	assert.Equal(t, "/opt/belune", call.WorkingDir)
	assert.Equal(t, "ghcr.io/weiliang79/belune:v1.2.3", call.Image)

	// The attempt is recorded, which is what lets GetSelfUpdateStatus report on
	// a helper that dies immediately instead of the dashboard claiming forever
	// that an update is under way.
	//
	// ⚠️ Cleaned up here because resetDB's TruncateAll does not clear settings
	// (see setUpdateSetting): without this the stored helper id leaks into the
	// next test, which then sees a stale attempt and reports "failed" where it
	// expected "idle". Found by running the suite, not this test alone.
	ctx := context.Background()
	for _, k := range []string{"update_helper_id", "update_helper_target", "update_helper_started_at"} {
		got, err := env.Queries.GetSetting(ctx, k)
		require.NoError(t, err, "%s must be recorded", k)
		assert.NotEmpty(t, got.Value, "%s must be recorded", k)
	}
}

// TestTriggerSelfUpdate_RefusesWhenAnUpdateIsAlreadyRunning covers the one gate
// that is not about the target version: a helper already at work.
//
// Nothing else stops a second run. The helper is created with an empty name so
// Docker never reports a conflict, and every other gate passes identically on a
// second click — the cached target has not moved and this container has not been
// replaced yet. Two update.sh processes inside the same window compute the same
// CURRENT_VERSION and so the same .env.backup-<v>/.infra-backup-<v> filenames,
// and the second clobbers the first: the files update.sh tells the operator are
// their way back.
//
// The subtests are the whole point. Matching on LabelHelper would have been the
// obvious implementation and is wrong — it would let a running volume restore
// block an update — and an exited helper must not lock the operator out of
// retrying after a failed update.
func TestTriggerSelfUpdate_RefusesWhenAnUpdateIsAlreadyRunning(t *testing.T) {
	self := runtime.ContainerInfo{
		ID:     selfContainerIDForTest(t),
		Image:  "ghcr.io/weiliang79/belune:v1.2.3",
		Labels: map[string]string{"com.docker.compose.project.working_dir": "/opt/belune"},
	}
	helper := func(status string, labels map[string]string) runtime.ContainerInfo {
		return runtime.ContainerInfo{ID: "helper123", Status: status, Labels: labels}
	}
	updateLabels := map[string]string{runtime.LabelHelper: "true", runtime.LabelUpdateHelper: "true"}
	otherLabels := map[string]string{runtime.LabelHelper: "true"}

	for _, tc := range []struct {
		name    string
		extra   runtime.ContainerInfo
		want    int
		spawned int
	}{
		{"a running update helper blocks", helper("running", updateLabels), http.StatusConflict, 0},
		{"a created update helper blocks, it is about to run", helper("created", updateLabels), http.StatusConflict, 0},
		{"an exited update helper does NOT block — a failed update must stay retryable", helper("exited", updateLabels), http.StatusAccepted, 1},
		{"a running backup/restore helper does NOT block — different job entirely", helper("running", otherLabels), http.StatusAccepted, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)
			token := env.SetupAdmin(t, "admin@test.com", "password123")
			withVersion(t, "v1.2.3")
			setUpdateSetting(t, "update_latest_version", "1.3.0")
			setUpdateSetting(t, "update_latest_requires_host_update", "false")
			clearUpdateAttemptSettings(t)

			env.Runtime.ListAllContainers_ = []runtime.ContainerInfo{self, tc.extra}
			env.Runtime.SpawnUpdateHelperCalls = nil
			t.Cleanup(func() {
				env.Runtime.ListAllContainers_ = nil
				env.Runtime.SpawnUpdateHelperCalls = nil
			})

			resp := env.DoRequest(t, "POST", "/api/maintenance/update", map[string]string{
				"password": "password123",
			}, testutil.AuthHeader(token))
			body := testutil.ReadJSON(t, resp)
			require.Equal(t, tc.want, resp.StatusCode, "%v", body)

			// The status alone is not the guarantee — no helper may be spawned.
			assert.Len(t, env.Runtime.SpawnUpdateHelperCalls, tc.spawned)
		})
	}
}

// TestGetSelfUpdateStatus covers the gap that made a failed update invisible:
// TriggerSelfUpdate answers 202 the moment the helper container is CREATED and
// never waits to see whether the script survived, so a helper dying on its
// first line left the dashboard saying "started" and then showing nothing.
//
// ⚠️ The "landed" case is the one that makes this non-trivial. A SUCCESSFUL
// update also leaves an exited helper behind, so "exited" cannot mean failure
// on its own — the version has to decide. A successful update replaces this
// container, so the process answering is the new build, and its version being
// at or past the target is what proves the update worked.
func TestGetSelfUpdateStatus(t *testing.T) {
	const path = "/api/maintenance/update/status"

	t.Run("idle when nothing has been attempted", func(t *testing.T) {
		resetDB(t)
		token := env.SetupAdmin(t, "admin@test.com", "password123")
		resp := env.DoRequest(t, "GET", path, nil, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "idle", testutil.ReadJSON(t, resp)["state"])
	})

	t.Run("running while the helper is alive", func(t *testing.T) {
		resetDB(t)
		token := env.SetupAdmin(t, "admin@test.com", "password123")
		withVersion(t, "v1.2.3")
		setUpdateSetting(t, "update_helper_id", "helper123")
		setUpdateSetting(t, "update_helper_target", "1.3.0")

		env.Runtime.ListAllContainers_ = []runtime.ContainerInfo{
			{ID: "helper123", Status: "running"},
		}
		t.Cleanup(func() { env.Runtime.ListAllContainers_ = nil })

		body := testutil.ReadJSON(t, env.DoRequest(t, "GET", path, nil, testutil.AuthHeader(token)))
		assert.Equal(t, "running", body["state"])
		assert.Equal(t, "1.3.0", body["target"])
	})

	t.Run("failed when the helper exited and the version never moved", func(t *testing.T) {
		resetDB(t)
		token := env.SetupAdmin(t, "admin@test.com", "password123")
		withVersion(t, "v1.2.3") // still the OLD version — the update did not land
		setUpdateSetting(t, "update_helper_id", "helper123")
		setUpdateSetting(t, "update_helper_target", "1.3.0")

		env.Runtime.ListAllContainers_ = []runtime.ContainerInfo{
			{ID: "helper123", Status: "exited"},
		}
		env.Runtime.ContainerLogsTail_ = "bash: scripts/update.sh: No such file or directory\n"
		t.Cleanup(func() {
			env.Runtime.ListAllContainers_ = nil
			env.Runtime.ContainerLogsTail_ = ""
		})

		body := testutil.ReadJSON(t, env.DoRequest(t, "GET", path, nil, testutil.AuthHeader(token)))
		assert.Equal(t, "failed", body["state"])
		// The helper's own output is the only place the cause appears — quoting
		// it is the whole point, so a generic "it failed" is not enough here.
		assert.Contains(t, body["reason"], "No such file or directory")
	})

	t.Run("⚠️ NOT failed when the update landed, though the helper also exited", func(t *testing.T) {
		resetDB(t)
		token := env.SetupAdmin(t, "admin@test.com", "password123")
		withVersion(t, "v1.3.0") // the new build IS the target — it worked
		setUpdateSetting(t, "update_helper_id", "helper123")
		setUpdateSetting(t, "update_helper_target", "1.3.0")

		env.Runtime.ListAllContainers_ = []runtime.ContainerInfo{
			{ID: "helper123", Status: "exited"},
		}
		t.Cleanup(func() { env.Runtime.ListAllContainers_ = nil })

		body := testutil.ReadJSON(t, env.DoRequest(t, "GET", path, nil, testutil.AuthHeader(token)))
		assert.Equal(t, "idle", body["state"],
			"an exited helper after a SUCCESSFUL update must not be reported as a failure")

		// The record is cleared, so a later unrelated failure cannot inherit it.
		body = testutil.ReadJSON(t, env.DoRequest(t, "GET", path, nil, testutil.AuthHeader(token)))
		assert.Equal(t, "idle", body["state"])
	})

	t.Run("failed when the helper is gone entirely", func(t *testing.T) {
		resetDB(t)
		token := env.SetupAdmin(t, "admin@test.com", "password123")
		withVersion(t, "v1.2.3")
		setUpdateSetting(t, "update_helper_id", "reaped-long-ago")
		setUpdateSetting(t, "update_helper_target", "1.3.0")

		env.Runtime.ListAllContainers_ = []runtime.ContainerInfo{}
		t.Cleanup(func() { env.Runtime.ListAllContainers_ = nil })

		body := testutil.ReadJSON(t, env.DoRequest(t, "GET", path, nil, testutil.AuthHeader(token)))
		assert.Equal(t, "failed", body["state"])
	})
}
