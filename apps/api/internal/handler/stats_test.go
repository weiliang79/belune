package handler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weiliang79/belune/internal/testutil"
)

func TestGetStats_AdminAndMemberSplit(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	env.DoRequest(t, "POST", "/api/users", map[string]string{
		"email":    "member@test.com",
		"password": "password123",
		"role":     "member",
	}, testutil.AuthHeader(adminToken)).Body.Close()
	memberToken := env.LoginAs(t, "member@test.com", "password123")

	env.CreateProject(t, adminToken, "Project 1", "project-1")

	// Admin: exercises every aggregate query, sees host + is_admin true.
	resp := env.DoRequest(t, "GET", "/api/stats", nil, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	admin := testutil.ReadJSON(t, resp)
	assert.Equal(t, true, admin["is_admin"])
	assert.NotNil(t, admin["host"])
	assert.Contains(t, admin, "app_health")
	assert.Contains(t, admin, "deploy_7d")
	assert.Contains(t, admin, "needs_attention")

	// Member: scoped view, no host snapshot.
	resp = env.DoRequest(t, "GET", "/api/stats", nil, testutil.AuthHeader(memberToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	member := testutil.ReadJSON(t, resp)
	assert.Equal(t, false, member["is_admin"])
	assert.Nil(t, member["host"])
}
