package handler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ungweiliang/selfhost-paas/internal/testutil"
)

func TestCreateGitCredential(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/git-credentials", map[string]any{
		"name":     "My GitHub Token",
		"provider": "github",
		"token":    "ghp_test123456",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	result := testutil.ReadJSON(t, resp)
	assert.Equal(t, "My GitHub Token", result["name"])
	assert.Equal(t, "github", result["provider"])
	// Token should not be returned
	assert.Empty(t, result["token"])
}

func TestListGitCredentials(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")

	// Create two credentials
	env.DoRequest(t, "POST", "/api/git-credentials", map[string]any{
		"name": "Cred 1", "provider": "github", "token": "token1",
	}, testutil.AuthHeader(token)).Body.Close()
	env.DoRequest(t, "POST", "/api/git-credentials", map[string]any{
		"name": "Cred 2", "provider": "gitlab", "token": "token2",
	}, testutil.AuthHeader(token)).Body.Close()

	resp := env.DoRequest(t, "GET", "/api/git-credentials", nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	creds := testutil.ReadJSONArray(t, resp)
	assert.Len(t, creds, 2)
}

func TestUpdateGitCredential(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")

	// Create
	resp := env.DoRequest(t, "POST", "/api/git-credentials", map[string]any{
		"name": "Original", "provider": "github", "token": "token1",
	}, testutil.AuthHeader(token))
	created := testutil.ReadJSON(t, resp)
	credID := extractID(created["id"])

	// Update
	resp = env.DoRequest(t, "PUT", "/api/git-credentials/"+credID, map[string]any{
		"name":     "Updated Name",
		"provider": "github",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	updated := testutil.ReadJSON(t, resp)
	assert.Equal(t, "Updated Name", updated["name"])
}

func TestDeleteGitCredential(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")

	// Create
	resp := env.DoRequest(t, "POST", "/api/git-credentials", map[string]any{
		"name": "ToDelete", "provider": "github", "token": "token1",
	}, testutil.AuthHeader(token))
	created := testutil.ReadJSON(t, resp)
	credID := extractID(created["id"])

	// Delete
	resp = env.DoRequest(t, "DELETE", "/api/git-credentials/"+credID, nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify list is empty
	resp = env.DoRequest(t, "GET", "/api/git-credentials", nil, testutil.AuthHeader(token))
	creds := testutil.ReadJSONArray(t, resp)
	assert.Len(t, creds, 0)
}

func TestCreateGitCredential_InvalidProvider(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/git-credentials", map[string]any{
		"name": "Bad", "provider": "invalid", "token": "token1",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestCreateGitCredential_Unauthenticated(t *testing.T) {
	resetDB(t)

	resp := env.DoRequest(t, "POST", "/api/git-credentials", map[string]any{
		"name": "Test", "provider": "github", "token": "token1",
	}, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

