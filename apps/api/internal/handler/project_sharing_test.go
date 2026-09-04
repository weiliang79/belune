package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/testutil"
)

// createMember creates a member user and returns (id, token).
func createMember(t *testing.T, adminToken, email string) (string, string) {
	t.Helper()
	resp := env.DoRequest(t, "POST", "/api/users", map[string]string{
		"email":    email,
		"password": "password123",
		"role":     "member",
	}, testutil.AuthHeader(adminToken))
	body := testutil.ReadJSON(t, resp)
	id := extractID(body["id"])
	return id, env.LoginAs(t, email, "password123")
}

// TestUpdateProjectSharing_WidensAccessToEveryMember pins the core behaviour:
// a shared project becomes reachable — read, list, stats — by a Member who is
// neither its owner nor an admin, and reverts when unshared.
func TestUpdateProjectSharing_WidensAccessToEveryMember(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	_, ownerToken := createMember(t, adminToken, "owner@test.com")
	_, otherToken := createMember(t, adminToken, "other@test.com")

	project := env.CreateProject(t, ownerToken, "Team Project", "team-project")
	projectID := extractID(project["id"])

	// Not shared yet: other member is blocked, and does not see it listed.
	resp := env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s", projectID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()

	resp = env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, testutil.ReadJSONArray(t, resp), 0)

	// A Member who owns the project can share it — not admin-only.
	resp = env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/sharing", projectID), map[string]bool{
		"shared": true,
	}, testutil.AuthHeader(ownerToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	shared := testutil.ReadJSON(t, resp)
	assert.Equal(t, true, shared["shared"])

	// Now the other member can read it, and sees it in their list.
	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s", projectID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, testutil.ReadJSONArray(t, resp), 1)

	// A stats aggregate scoped by user must also include the shared project.
	env.CreateApplication(t, ownerToken, projectID, map[string]any{
		"name":        "Shared App",
		"type":        "git",
		"build_type":  "dockerfile",
		"source_repo": "https://github.com/test/repo",
	})
	resp = env.DoRequest(t, "GET", "/api/stats", nil, testutil.AuthHeader(otherToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	stats := testutil.ReadJSON(t, resp)
	appHealth := stats["app_health"].(map[string]any)
	assert.EqualValues(t, 1, appHealth["total"], "the shared project's application must count toward a member's stats")

	// Unsharing revokes access again.
	resp = env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/sharing", projectID), map[string]bool{
		"shared": false,
	}, testutil.AuthHeader(ownerToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s", projectID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// TestUpdateProjectSharing_OwnerOnly pins that sharing access does NOT grant
// the right to change sharing, delete, or transfer the project — those stay
// owner (or admin) only, even once a member can otherwise use the project.
func TestUpdateProjectSharing_OwnerOnly(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	_, ownerToken := createMember(t, adminToken, "owner@test.com")
	_, otherToken := createMember(t, adminToken, "other@test.com")

	project := env.CreateProject(t, ownerToken, "Owner Only", "owner-only")
	projectID := extractID(project["id"])

	// Non-owner cannot share a project it does not own.
	resp := env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/sharing", projectID), map[string]bool{
		"shared": true,
	}, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()

	// Admin shares it on the owner's behalf.
	resp = env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/sharing", projectID), map[string]bool{
		"shared": true,
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// The other member now has access, but still cannot unshare or delete it.
	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s", projectID), nil, testutil.AuthHeader(otherToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/sharing", projectID), map[string]bool{
		"shared": false,
	}, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "shared access must not grant the right to unshare")
	resp.Body.Close()

	resp = env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s", projectID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "shared access must not grant delete rights")
	resp.Body.Close()
}

// TestUpdateProjectSharing_QuotaStaysOwnerScoped is the trap the plan called
// out explicitly: quotas follow ownership, not access. A shared project's
// applications must count only against its owner's quota usage, never against
// a member who merely has shared access to it.
func TestUpdateProjectSharing_QuotaStaysOwnerScoped(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	ownerID, ownerToken := createMember(t, adminToken, "owner@test.com")
	otherID, _ := createMember(t, adminToken, "other@test.com")

	project := env.CreateProject(t, ownerToken, "Quota Project", "quota-project")
	projectID := extractID(project["id"])
	env.CreateApplication(t, ownerToken, projectID, map[string]any{
		"name":        "App",
		"type":        "git",
		"build_type":  "dockerfile",
		"source_repo": "https://github.com/test/repo",
	})

	resp := env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/sharing", projectID), map[string]bool{
		"shared": true,
	}, testutil.AuthHeader(ownerToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/quotas/user/%s", ownerID), nil, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	ownerUsage := testutil.ReadJSON(t, resp)["usage"].(map[string]any)
	assert.EqualValues(t, 1, ownerUsage["applications"], "the owner's quota usage must count the application")

	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/quotas/user/%s", otherID), nil, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	otherUsage := testutil.ReadJSON(t, resp)["usage"].(map[string]any)
	assert.EqualValues(t, 0, otherUsage["applications"],
		"a shared project's applications must NOT count against a non-owner's quota")
}

// TestUpdateProjectSharing_MemberCanUseButNotDestroy pins the boundary a code
// review caught: sharing must grant a Member full operational use of the
// project's applications, databases, and domains, but NEVER the right to
// destroy them. Only the owner (or an admin) may delete an application,
// delete a database, or remove a domain — even once shared access already
// lets that Member reach, deploy, and manage those resources.
func TestUpdateProjectSharing_MemberCanUseButNotDestroy(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	_, ownerToken := createMember(t, adminToken, "owner@test.com")
	_, otherToken := createMember(t, adminToken, "other@test.com")

	project := env.CreateProject(t, ownerToken, "Use Not Destroy", "use-not-destroy")
	projectID := extractID(project["id"])

	app := env.CreateApplication(t, ownerToken, projectID, map[string]any{
		"name":        "App",
		"type":        "git",
		"build_type":  "dockerfile",
		"source_repo": "https://github.com/test/repo",
	})
	appID := extractID(app["id"])

	dbResp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/databases", projectID), map[string]any{
		"name": "db", "type": "postgres",
	}, testutil.AuthHeader(ownerToken))
	require.Equal(t, http.StatusAccepted, dbResp.StatusCode)
	dbID := extractID(testutil.ReadJSON(t, dbResp)["id"])

	domainResp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/domains", projectID, appID), map[string]any{
		"hostname": "shared.example.com",
	}, testutil.AuthHeader(ownerToken))
	require.Equal(t, http.StatusCreated, domainResp.StatusCode)
	domainID := extractID(testutil.ReadJSON(t, domainResp)["id"])

	shareResp := env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/sharing", projectID), map[string]bool{
		"shared": true,
	}, testutil.AuthHeader(ownerToken))
	require.Equal(t, http.StatusOK, shareResp.StatusCode)
	shareResp.Body.Close()

	// Positive: shared access reaches the application, database, and domain.
	resp := env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications/%s", projectID, appID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode, "a shared member must be able to read the application")
	resp.Body.Close()

	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/databases/%s", projectID, dbID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode, "a shared member must be able to read the database")
	resp.Body.Close()

	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications/%s/domains", projectID, appID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode, "a shared member must be able to list domains")
	resp.Body.Close()

	// Negative: shared access does NOT reach delete.
	resp = env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s/applications/%s", projectID, appID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a shared member must not be able to delete the application")
	resp.Body.Close()

	resp = env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s/databases/%s", projectID, dbID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a shared member must not be able to delete the database")
	resp.Body.Close()

	resp = env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s/applications/%s/domains/%s", projectID, appID, domainID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a shared member must not be able to remove the domain")
	resp.Body.Close()

	// The owner retains full destroy rights throughout.
	resp = env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s/applications/%s/domains/%s", projectID, appID, domainID), nil, testutil.AuthHeader(ownerToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode, "the owner must still be able to remove the domain")
	resp.Body.Close()

	resp = env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s/databases/%s", projectID, dbID), nil, testutil.AuthHeader(ownerToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode, "the owner must still be able to delete the database")
	resp.Body.Close()

	resp = env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s/applications/%s", projectID, appID), nil, testutil.AuthHeader(ownerToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode, "the owner must still be able to delete the application")
	resp.Body.Close()
}

// TestCanAccessApplication_NotShared_StillBlocked is the negative-direction
// coverage a review flagged as missing: a Member with no relationship to a
// PRIVATE project's application must stay blocked, so a bug that widened
// canAccessOwned's "OR shared" branch unconditionally would be caught here.
func TestCanAccessApplication_NotShared_StillBlocked(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	_, ownerToken := createMember(t, adminToken, "owner@test.com")
	_, otherToken := createMember(t, adminToken, "other@test.com")

	project := env.CreateProject(t, ownerToken, "Private", "private")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, ownerToken, projectID, map[string]any{
		"name":        "App",
		"type":        "git",
		"build_type":  "dockerfile",
		"source_repo": "https://github.com/test/repo",
	})
	appID := extractID(app["id"])

	resp := env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications/%s", projectID, appID), nil, testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a non-member must stay blocked from a private project's application")
	resp.Body.Close()
}
