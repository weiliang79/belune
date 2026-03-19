package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/testutil"
)

func TestAddDomain(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Domain App", "type": "git", "build_type": "dockerfile",
	})
	appID := extractID(app["id"])

	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/domains", projectID, appID), map[string]any{
		"hostname":    "example.com",
		"ssl_enabled": true,
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	result := testutil.ReadJSON(t, resp)
	assert.Equal(t, "example.com", result["hostname"])

	// Verify proxy AddRoute was called
	require.Len(t, env.Proxy.AddedRoutes, 1)
	assert.Equal(t, "example.com", env.Proxy.AddedRoutes[0].Hostname)
}

func TestRemoveDomain(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Domain App", "type": "git", "build_type": "dockerfile",
	})
	appID := extractID(app["id"])

	// Add domain
	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/domains", projectID, appID), map[string]any{
		"hostname": "remove.example.com",
	}, testutil.AuthHeader(token))
	domain := testutil.ReadJSON(t, resp)
	domainID := extractID(domain["id"])

	// Remove domain
	resp = env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s/applications/%s/domains/%s", projectID, appID, domainID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify proxy RemoveRoute was called
	require.Len(t, env.Proxy.RemovedRoutes, 1)
	assert.Equal(t, "remove.example.com", env.Proxy.RemovedRoutes[0])
}

func TestListDomains(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Domain App", "type": "git", "build_type": "dockerfile",
	})
	appID := extractID(app["id"])

	// Add two domains
	env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/domains", projectID, appID), map[string]any{
		"hostname": "one.example.com",
	}, testutil.AuthHeader(token)).Body.Close()
	env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/domains", projectID, appID), map[string]any{
		"hostname": "two.example.com",
	}, testutil.AuthHeader(token)).Body.Close()

	resp := env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications/%s/domains", projectID, appID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	domains := testutil.ReadJSONArray(t, resp)
	assert.Len(t, domains, 2)
}
