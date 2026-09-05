package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/service"
	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/testutil"
)

// mintScoped creates a token via the real POST /api/tokens endpoint (session
// auth) with exactly the given scopes, and returns its plaintext.
func mintScoped(t *testing.T, sessionToken string, scopes []string) string {
	t.Helper()
	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "scope-test",
		"scopes": scopes,
	}, testutil.AuthHeader(sessionToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	return testutil.ReadJSON(t, resp)["token"].(string)
}

// createPinnedAPIToken inserts a token directly, pinned to projectID — there
// is no create-endpoint field for this yet (project narrowing has no UI),
// but the enforcement side must still honor a pin however the row got it.
func createPinnedAPIToken(t *testing.T, userID, projectID string, scopes []string) (plain string) {
	t.Helper()
	var uid, pid pgtype.UUID
	require.NoError(t, uid.Scan(userID))
	require.NoError(t, pid.Scan(projectID))

	plainTok, hash, err := service.GenerateToken()
	require.NoError(t, err)

	_, err = env.Queries.CreateAPIToken(context.Background(), generated.CreateAPITokenParams{
		UserID:      uid,
		Name:        "pinned",
		TokenHash:   hash,
		Scopes:      scopes,
		ProjectID:   pid,
		RoleAtIssue: "admin",
	})
	require.NoError(t, err)
	return plainTok
}

func minimalApp(t *testing.T, token, projectID string) map[string]any {
	t.Helper()
	return env.CreateApplication(t, token, projectID, map[string]any{
		"name":         "Scope Test App",
		"type":         "image",
		"build_type":   "image",
		"source_image": "nginx:latest",
	})
}

// TestScope_ReadTokenCannotWrite pins the core lattice rule: a read-only
// token may GET but is refused on a write endpoint.
func TestScope_ReadTokenCannotWrite(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])

	readPlain := mintScoped(t, adminToken, []string{"read"})

	getResp := env.DoRequest(t, "GET", "/api/projects/"+projectID, nil, testutil.AuthHeader(readPlain))
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "read scope must satisfy a GET")
	getResp.Body.Close()

	putResp := env.DoRequest(t, "PUT", "/api/projects/"+projectID, map[string]any{
		"name": "Renamed",
	}, testutil.AuthHeader(readPlain))
	assert.Equal(t, http.StatusForbidden, putResp.StatusCode, "read scope must not satisfy a write")
	putResp.Body.Close()
}

// TestScope_WriteTokenCanAlsoRead pins the asymmetric half of the lattice:
// write is the umbrella scope, so a write-scoped token is not also required
// to carry "read" separately.
func TestScope_WriteTokenCanAlsoRead(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])

	writePlain := mintScoped(t, adminToken, []string{"write"})

	getResp := env.DoRequest(t, "GET", "/api/projects/"+projectID, nil, testutil.AuthHeader(writePlain))
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "write must satisfy a read requirement too")
	getResp.Body.Close()

	putResp := env.DoRequest(t, "PUT", "/api/projects/"+projectID, map[string]any{
		"name": "Renamed",
	}, testutil.AuthHeader(writePlain))
	assert.Equal(t, http.StatusOK, putResp.StatusCode)
	putResp.Body.Close()
}

// TestScope_DeployTokenCannotDoGeneralWrite pins that "deploy" is narrower
// than "write" — a CI deploy token must not also be able to rewrite the
// application's stored configuration.
func TestScope_DeployTokenCannotDoGeneralWrite(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])
	app := minimalApp(t, adminToken, projectID)
	appID := extractID(app["id"])

	deployPlain := mintScoped(t, adminToken, []string{"deploy"})

	deployResp := env.DoRequest(t, "POST",
		fmt.Sprintf("/api/projects/%s/applications/%s/deploy", projectID, appID),
		nil, testutil.AuthHeader(deployPlain))
	assert.NotEqual(t, http.StatusForbidden, deployResp.StatusCode, "deploy scope must satisfy a deploy action")
	deployResp.Body.Close()

	updateResp := env.DoRequest(t, "PUT",
		fmt.Sprintf("/api/projects/%s/applications/%s", projectID, appID),
		map[string]any{"name": "Renamed", "source_repo": "", "source_image": "nginx:latest"},
		testutil.AuthHeader(deployPlain))
	assert.Equal(t, http.StatusForbidden, updateResp.StatusCode, "deploy scope must not satisfy general write")
	updateResp.Body.Close()
}

// TestScope_DeployTokenCannotRead pins that deploy does not imply read
// either — the two are independent narrow grants, not a ladder.
func TestScope_DeployTokenCannotRead(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])

	deployPlain := mintScoped(t, adminToken, []string{"deploy"})

	resp := env.DoRequest(t, "GET", "/api/projects/"+projectID, nil, testutil.AuthHeader(deployPlain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// TestScope_WriteTokenCanAlsoDeploy pins that "write" satisfies "deploy" too
// — a general-purpose token loses nothing PR3 already gave it.
func TestScope_WriteTokenCanAlsoDeploy(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])
	app := minimalApp(t, adminToken, projectID)
	appID := extractID(app["id"])

	writePlain := mintScoped(t, adminToken, []string{"write"})

	resp := env.DoRequest(t, "POST",
		fmt.Sprintf("/api/projects/%s/applications/%s/deploy", projectID, appID),
		nil, testutil.AuthHeader(writePlain))
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// TestScope_MetricsTokenCannotReadGenerally pins that "metrics" is narrower
// than "read" — a Prometheus-style scrape token must not be able to browse
// arbitrary project data just because metrics are technically readable.
func TestScope_MetricsTokenCannotReadGenerally(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])

	metricsPlain := mintScoped(t, adminToken, []string{"metrics"})

	metricsResp := env.DoRequest(t, "GET", "/api/projects/"+projectID+"/metrics", nil, testutil.AuthHeader(metricsPlain))
	assert.Equal(t, http.StatusOK, metricsResp.StatusCode, "metrics scope must satisfy the metrics endpoint")
	metricsResp.Body.Close()

	readResp := env.DoRequest(t, "GET", "/api/projects/"+projectID, nil, testutil.AuthHeader(metricsPlain))
	assert.Equal(t, http.StatusForbidden, readResp.StatusCode, "metrics scope must not satisfy a general read")
	readResp.Body.Close()
}

// TestScope_ReadTokenCanAlsoReadMetrics pins that general "read" already
// covers the narrower metrics endpoints too.
func TestScope_ReadTokenCanAlsoReadMetrics(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])

	readPlain := mintScoped(t, adminToken, []string{"read"})

	resp := env.DoRequest(t, "GET", "/api/projects/"+projectID+"/metrics", nil, testutil.AuthHeader(readPlain))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// TestScope_SessionBypassesScopeEntirely is the control: a session
// authenticates with no scope restriction at all, regardless of what a token
// would need for the same route.
func TestScope_SessionBypassesScopeEntirely(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])

	resp := env.DoRequest(t, "PUT", "/api/projects/"+projectID, map[string]any{"name": "x"}, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// TestProjectPin_CannotReachADifferentProject pins the second axis of token
// reach alongside scope: a token pinned to project X, even with full scope,
// must not reach project Y — the query-string style filter runs regardless
// of what the underlying user could otherwise access.
func TestProjectPin_CannotReachADifferentProject(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	adminID := extractID(testutil.ReadJSON(t, env.DoRequest(t, "GET", "/api/auth/me", nil, testutil.AuthHeader(adminToken)))["id"])

	projectX := env.CreateProject(t, adminToken, "Project X", "project-x")
	projectXID := extractID(projectX["id"])
	projectY := env.CreateProject(t, adminToken, "Project Y", "project-y")
	projectYID := extractID(projectY["id"])

	pinnedPlain := createPinnedAPIToken(t, adminID, projectXID, service.AllScopes)

	respX := env.DoRequest(t, "GET", "/api/projects/"+projectXID, nil, testutil.AuthHeader(pinnedPlain))
	assert.Equal(t, http.StatusOK, respX.StatusCode, "a pinned token must still reach its own project")
	respX.Body.Close()

	respY := env.DoRequest(t, "GET", "/api/projects/"+projectYID, nil, testutil.AuthHeader(pinnedPlain))
	assert.Equal(t, http.StatusForbidden, respY.StatusCode, "a pinned token must not reach a different project, even with full scope")
	respY.Body.Close()
}

// TestProjectPin_LostAccessReachesNothing pins the plan's own test bullet: a
// token pinned to a project the owner lost access to reaches nothing. The
// owner here is a Member whose access came entirely from sharing (never an
// owner), so revoking it leaves them with no access at all — proving the
// pre-existing PR1 access check and this PR's project-pin check compose
// rather than either one alone silently covering the gap.
func TestProjectPin_LostAccessReachesNothing(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	memberID, memberToken := createMember(t, adminToken, "member@test.com")

	project := env.CreateProject(t, adminToken, "Shared Project", "shared-project")
	projectID := extractID(project["id"])

	shareResp := env.DoRequest(t, "PUT", "/api/projects/"+projectID+"/sharing", map[string]any{"shared": true}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, shareResp.StatusCode)
	shareResp.Body.Close()

	pinnedPlain := createPinnedAPIToken(t, memberID, projectID, service.AllScopes)

	okResp := env.DoRequest(t, "GET", "/api/projects/"+projectID, nil, testutil.AuthHeader(pinnedPlain))
	assert.Equal(t, http.StatusOK, okResp.StatusCode, "sanity: reachable while shared")
	okResp.Body.Close()

	unshareResp := env.DoRequest(t, "PUT", "/api/projects/"+projectID+"/sharing", map[string]any{"shared": false}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, unshareResp.StatusCode)
	unshareResp.Body.Close()

	goneResp := env.DoRequest(t, "GET", "/api/projects/"+projectID, nil, testutil.AuthHeader(pinnedPlain))
	assert.Equal(t, http.StatusForbidden, goneResp.StatusCode, "losing the underlying access must cut the pinned token off too")
	goneResp.Body.Close()

	_ = memberToken
}

// TestTerminal_RequiresSession pins the gap a 2026-09-05 review flagged
// explicitly for this PR: a PAT reaching interactive container exec with no
// step-up at all. Both the session creation endpoint and the WS tunnel now
// require a live session — a PAT never gets far enough to attempt the
// protocol upgrade, so this is testable as an ordinary rejected HTTP request.
func TestTerminal_RequiresSession(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Terminal Project", "terminal-project")
	projectID := extractID(project["id"])
	app := minimalApp(t, adminToken, projectID)
	appID := extractID(app["id"])

	fullPlain := mintScoped(t, adminToken, service.AllScopes)

	createResp := env.DoRequest(t, "POST",
		fmt.Sprintf("/api/projects/%s/applications/%s/terminal", projectID, appID),
		nil, testutil.AuthHeader(fullPlain))
	assert.Equal(t, http.StatusForbidden, createResp.StatusCode, "a PAT — even with every scope — must not create a terminal session")
	createResp.Body.Close()

	wsResp := env.DoRequest(t, "GET", "/api/ws/terminal/00000000-0000-0000-0000-000000000000", nil, testutil.AuthHeader(fullPlain))
	assert.Equal(t, http.StatusForbidden, wsResp.StatusCode, "a PAT must not reach the terminal WS tunnel either")
	wsResp.Body.Close()
}

// TestWebSocketHub_RequiresReadScope pins that the general WS hub (live
// updates) is scope-gated like any other read endpoint — a deploy-only token
// must not subscribe to it.
func TestWebSocketHub_RequiresReadScope(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	deployPlain := mintScoped(t, adminToken, []string{"deploy"})
	resp := env.DoRequest(t, "GET", "/api/ws", nil, testutil.AuthHeader(deployPlain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}
