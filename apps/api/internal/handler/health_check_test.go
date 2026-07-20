package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/testutil"
)

func TestSetHealthCheck(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "HC App", "type": "image", "build_type": "image",
		"source_image": "nginx:1.25",
	})
	appID := extractID(app["id"])
	url := fmt.Sprintf("/api/projects/%s/applications/%s/health-check", projectID, appID)

	// Configure a command check.
	resp := env.DoRequest(t, "PUT", url, map[string]any{
		"type": "command", "command": "curl -f http://localhost/health",
		"interval_seconds": 15, "retries": 2,
	}, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	result := testutil.ReadJSON(t, resp)
	assert.Equal(t, "command", result["health_check_type"])
	assert.Equal(t, "curl -f http://localhost/health", result["health_check_command"])

	// Stored as expected.
	var typ, command string
	var interval int
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT health_check_type, COALESCE(health_check_command,''), COALESCE(health_check_interval_seconds,0)
		   FROM applications WHERE id = $1`, appID).Scan(&typ, &command, &interval))
	assert.Equal(t, "command", typ)
	assert.Equal(t, "curl -f http://localhost/health", command)
	assert.Equal(t, 15, interval)

	// Switching to http clears the command, so no stale check survives.
	resp = env.DoRequest(t, "PUT", url, map[string]any{
		"type": "http", "path": "/healthz",
	}, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	var hasCommand bool
	var path string
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT health_check_command IS NOT NULL, COALESCE(health_check_path,'')
		   FROM applications WHERE id = $1`, appID).Scan(&hasCommand, &path))
	assert.False(t, hasCommand, "the command must be cleared when switching to http")
	assert.Equal(t, "/healthz", path)
}

func TestSetHealthCheck_Rejections(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "HC App", "type": "image", "build_type": "image",
		"source_image": "nginx:1.25",
	})
	appID := extractID(app["id"])
	url := fmt.Sprintf("/api/projects/%s/applications/%s/health-check", projectID, appID)

	for name, body := range map[string]map[string]any{
		"command with no command": {"type": "command"},
		"http with no path":       {"type": "http"},
		"unknown type":            {"type": "tcp"},
	} {
		t.Run(name, func(t *testing.T) {
			resp := env.DoRequest(t, "PUT", url, body, testutil.AuthHeader(token))
			defer resp.Body.Close()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}
