package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/testutil"
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

func TestAddDomainWithConfig(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Domain App", "type": "git", "build_type": "dockerfile",
	})
	appID := extractID(app["id"])

	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/domains", projectID, appID), map[string]any{
		"hostname":       "config.example.com",
		"ssl_enabled":    true,
		"container_port": 3000,
		"force_https":    true,
		"ssl_mode":       "automatic",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	result := testutil.ReadJSON(t, resp)
	assert.Equal(t, "config.example.com", result["hostname"])
	assert.Equal(t, true, result["force_https"])
	assert.Equal(t, "automatic", result["ssl_mode"])

	// Verify proxy received the config
	require.Len(t, env.Proxy.AddedRoutes, 1)
	assert.Equal(t, true, env.Proxy.AddedRoutes[0].ForceHTTPS)
}

func TestUpdateDomain(t *testing.T) {
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
		"hostname": "update.example.com",
	}, testutil.AuthHeader(token))
	domain := testutil.ReadJSON(t, resp)
	domainID := extractID(domain["id"])

	// Update domain
	resp = env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/applications/%s/domains/%s", projectID, appID, domainID), map[string]any{
		"hostname":    "update.example.com",
		"force_https": true,
		"ssl_mode":    "off",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	updated := testutil.ReadJSON(t, resp)
	assert.Equal(t, "off", updated["ssl_mode"])
}

func TestAddDomain_InvalidSSLMode(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Domain App", "type": "git", "build_type": "dockerfile",
	})
	appID := extractID(app["id"])

	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/domains", projectID, appID), map[string]any{
		"hostname": "bad.example.com",
		"ssl_mode": "invalid_mode",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
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
