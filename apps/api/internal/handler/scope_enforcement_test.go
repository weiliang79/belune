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

// TestScope_DeployTokenCanAlsoRead pins that "deploy" grants "read" too — a
// CI token that can trigger a deploy must also be able to poll its result,
// or triggering one is a self-inflicted footgun. This is the one place the
// lattice is NOT symmetric with "metrics": deploy grants read, but read (and
// write) still do not grant deploy back, and metrics grants neither.
func TestScope_DeployTokenCanAlsoRead(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])

	deployPlain := mintScoped(t, adminToken, []string{"deploy"})

	resp := env.DoRequest(t, "GET", "/api/projects/"+projectID, nil, testutil.AuthHeader(deployPlain))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// TestScope_DeployTokenCannotReadMetrics pins that the read grant deploy
// picked up is ordinary read, not a transitive hop into "metrics" too —
// scopeSatisfies is a flat per-requirement lookup, not a chain through an
// intermediate scope.
func TestScope_DeployTokenCannotReadMetrics(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "Scope Project", "scope-project")
	projectID := extractID(project["id"])

	deployPlain := mintScoped(t, adminToken, []string{"deploy"})

	resp := env.DoRequest(t, "GET", "/api/projects/"+projectID+"/metrics", nil, testutil.AuthHeader(deployPlain))
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
// updates) is scope-gated like any other read endpoint — a metrics-only
// token (the narrowest scope that still does NOT satisfy "read" — deploy now
// does, see TestScope_DeployTokenCanAlsoRead) must not subscribe to it.
func TestWebSocketHub_RequiresReadScope(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	metricsPlain := mintScoped(t, adminToken, []string{"metrics"})
	resp := env.DoRequest(t, "GET", "/api/ws", nil, testutil.AuthHeader(metricsPlain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// TestUserManagement_RequiresSession pins the fix for a review finding: an
// admin-role, write-scoped PAT could POST /api/users with role:"admin" and a
// chosen password, then log in with those credentials for a real session —
// which sails through every RequireSession gate in the app, including the
// ones guarding token self-mint and the destroy boundary. ResetUserPassword
// and AdminResetUserTOTP are worse: they take over an EXISTING admin account
// with no re-verification at all. All six user-management mutations now
// require a session; only the read (ListUsers) stays PAT-accessible.
func TestUserManagement_RequiresSession(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	memberID, _ := createMember(t, adminToken, "member@test.com")

	fullPlain := mintScoped(t, adminToken, service.AllScopes)

	cases := []struct {
		name, method, path string
		body               map[string]any
	}{
		{"create user", "POST", "/api/users", map[string]any{
			"email": "escalate@test.com", "password": "password123", "role": "admin",
		}},
		{"invite user", "POST", "/api/users/invite", map[string]any{
			"email": "escalate2@test.com", "role": "admin",
		}},
		{"update role", "PUT", "/api/users/" + memberID + "/role", map[string]any{"role": "admin"}},
		{"reset password", "PUT", "/api/users/" + memberID + "/password", map[string]any{"password": "newpassword123"}},
		{"reset totp", "POST", "/api/users/" + memberID + "/totp/reset", nil},
		{"delete user", "DELETE", "/api/users/" + memberID, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := env.DoRequest(t, tc.method, tc.path, tc.body, testutil.AuthHeader(fullPlain))
			defer resp.Body.Close()
			assert.Equal(t, http.StatusForbidden, resp.StatusCode, "%s must require a session", tc.name)
		})
	}

	// The read stays PAT-accessible — only the mutations are gated.
	listResp := env.DoRequest(t, "GET", "/api/users", nil, testutil.AuthHeader(fullPlain))
	assert.Equal(t, http.StatusOK, listResp.StatusCode)
	listResp.Body.Close()
}

// TestProjectPin_CannotCreateProject pins a review finding: RequireProjectAccess
// only ever compares against a {projectId} URL param, so project creation
// (which has no existing id to compare against) needed its own explicit
// check — a pinned token creating a new, unrelated project would otherwise
// make the pin meaningless as a boundary.
func TestProjectPin_CannotCreateProject(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	adminID := extractID(testutil.ReadJSON(t, env.DoRequest(t, "GET", "/api/auth/me", nil, testutil.AuthHeader(adminToken)))["id"])

	project := env.CreateProject(t, adminToken, "Pinned Project", "pinned-project")
	pinnedPlain := createPinnedAPIToken(t, adminID, extractID(project["id"]), service.AllScopes)

	resp := env.DoRequest(t, "POST", "/api/projects", map[string]any{
		"name": "Escape Project", "slug": "escape-project",
	}, testutil.AuthHeader(pinnedPlain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// TestProjectPin_TemplateInstantiate_CannotTargetDifferentProject pins the
// same review finding for template instantiation: the target project arrives
// in the request BODY, not a {projectId} URL param, so RequireProjectAccess
// never sees it — resolveTemplateProject has to check the pin itself.
func TestProjectPin_TemplateInstantiate_CannotTargetDifferentProject(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	adminID := extractID(testutil.ReadJSON(t, env.DoRequest(t, "GET", "/api/auth/me", nil, testutil.AuthHeader(adminToken)))["id"])

	pinnedProject := env.CreateProject(t, adminToken, "Pinned Project", "pinned-project")
	otherProject := env.CreateProject(t, adminToken, "Other Project", "other-project")
	pinnedPlain := createPinnedAPIToken(t, adminID, extractID(pinnedProject["id"]), service.AllScopes)

	resp := env.DoRequest(t, "POST", "/api/templates/excalidraw/instantiate", map[string]any{
		"project_id": extractID(otherProject["id"]),
	}, testutil.AuthHeader(pinnedPlain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// TestProjectPin_TemplateInstantiate_CannotCreateNewProject pins the other
// half: omitting project_id targets a brand new project, which by
// definition is not the one a pinned token was narrowed to.
func TestProjectPin_TemplateInstantiate_CannotCreateNewProject(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	adminID := extractID(testutil.ReadJSON(t, env.DoRequest(t, "GET", "/api/auth/me", nil, testutil.AuthHeader(adminToken)))["id"])

	pinnedProject := env.CreateProject(t, adminToken, "Pinned Project", "pinned-project")
	pinnedPlain := createPinnedAPIToken(t, adminID, extractID(pinnedProject["id"]), service.AllScopes)

	resp := env.DoRequest(t, "POST", "/api/templates/excalidraw/instantiate", map[string]any{
		"new_project_name": "Escape Project",
	}, testutil.AuthHeader(pinnedPlain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// TestProjectPin_GlobalDeployments_RejectsMismatchedFilter pins that the
// project_id query filter on GET /api/deployments is checked against the pin
// too — it is a query param, not a {projectId} URL param.
func TestProjectPin_GlobalDeployments_RejectsMismatchedFilter(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	adminID := extractID(testutil.ReadJSON(t, env.DoRequest(t, "GET", "/api/auth/me", nil, testutil.AuthHeader(adminToken)))["id"])

	pinnedProject := env.CreateProject(t, adminToken, "Pinned Project", "pinned-project")
	otherProject := env.CreateProject(t, adminToken, "Other Project", "other-project")
	pinnedPlain := createPinnedAPIToken(t, adminID, extractID(pinnedProject["id"]), service.AllScopes)

	resp := env.DoRequest(t, "GET", "/api/deployments?project_id="+extractID(otherProject["id"]), nil, testutil.AuthHeader(pinnedPlain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// TestProjectPin_GlobalDeployments_FiltersToOwnProjectWhenUnfiltered pins the
// other half: an absent filter must not fall through to every project the
// token's owner can reach — it is silently narrowed to the pin instead.
func TestProjectPin_GlobalDeployments_FiltersToOwnProjectWhenUnfiltered(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	adminID := extractID(testutil.ReadJSON(t, env.DoRequest(t, "GET", "/api/auth/me", nil, testutil.AuthHeader(adminToken)))["id"])

	pinnedProject := env.CreateProject(t, adminToken, "Pinned Project", "pinned-project")
	pinnedProjectID := extractID(pinnedProject["id"])
	pinnedApp := minimalApp(t, adminToken, pinnedProjectID)

	otherProject := env.CreateProject(t, adminToken, "Other Project", "other-project")
	otherApp := minimalApp(t, adminToken, extractID(otherProject["id"]))

	var pinnedAppUUID, otherAppUUID pgtype.UUID
	require.NoError(t, pinnedAppUUID.Scan(extractID(pinnedApp["id"])))
	require.NoError(t, otherAppUUID.Scan(extractID(otherApp["id"])))
	_, err := env.Queries.CreateDeployment(context.Background(), generated.CreateDeploymentParams{
		ApplicationID: pinnedAppUUID, Status: "success", TriggeredBy: "manual",
	})
	require.NoError(t, err)
	_, err = env.Queries.CreateDeployment(context.Background(), generated.CreateDeploymentParams{
		ApplicationID: otherAppUUID, Status: "success", TriggeredBy: "manual",
	})
	require.NoError(t, err)

	pinnedPlain := createPinnedAPIToken(t, adminID, pinnedProjectID, service.AllScopes)

	resp := env.DoRequest(t, "GET", "/api/deployments", nil, testutil.AuthHeader(pinnedPlain))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	rows := testutil.ReadJSONArray(t, resp)
	require.Len(t, rows, 1, "an unfiltered query must be narrowed to the pin, not span every project the owner can reach")
	row := rows[0].(map[string]any)
	assert.Equal(t, extractID(pinnedApp["id"]), row["application_id"])
}

// TestProjectPin_ListProjects_FiltersToOwnProject pins that GET /api/projects
// — another endpoint with no {projectId} URL param — does not let a pinned
// token enumerate every other project's name and slug.
func TestProjectPin_ListProjects_FiltersToOwnProject(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	adminID := extractID(testutil.ReadJSON(t, env.DoRequest(t, "GET", "/api/auth/me", nil, testutil.AuthHeader(adminToken)))["id"])

	pinnedProject := env.CreateProject(t, adminToken, "Pinned Project", "pinned-project")
	pinnedProjectID := extractID(pinnedProject["id"])
	env.CreateProject(t, adminToken, "Other Project", "other-project")

	pinnedPlain := createPinnedAPIToken(t, adminID, pinnedProjectID, service.AllScopes)

	resp := env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(pinnedPlain))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	rows := testutil.ReadJSONArray(t, resp)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	assert.Equal(t, pinnedProjectID, row["id"])
}
