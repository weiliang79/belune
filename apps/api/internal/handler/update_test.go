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
