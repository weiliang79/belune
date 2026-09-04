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
