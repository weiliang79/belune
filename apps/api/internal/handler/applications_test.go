package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/testutil"
)

func TestCreateApplication(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	// Git type application
	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name":        "My App",
		"type":        "git",
		"build_type":  "dockerfile",
		"source_repo": "https://github.com/test/repo",
	})
	assert.Equal(t, "My App", app["name"])
	assert.Equal(t, "git", app["type"])
	assert.Contains(t, app["slug"], "test-project")

	// Image type application
	app2 := env.CreateApplication(t, token, projectID, map[string]any{
		"name":         "Image App",
		"type":         "image",
		"build_type":   "image",
		"source_image": "nginx:latest",
	})
	assert.Equal(t, "Image App", app2["name"])
	assert.Equal(t, "image", app2["type"])
}

func TestListApplications(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	env.CreateApplication(t, token, projectID, map[string]any{
		"name": "App 1", "type": "git", "build_type": "dockerfile",
	})
	env.CreateApplication(t, token, projectID, map[string]any{
		"name": "App 2", "type": "git", "build_type": "dockerfile",
	})

	resp := env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications", projectID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	apps := testutil.ReadJSONArray(t, resp)
	assert.Len(t, apps, 2)
}

func TestUpdateApplication(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Original", "type": "git", "build_type": "dockerfile",
	})
	appID := extractID(app["id"])

	resp := env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/applications/%s", projectID, appID), map[string]any{
		"name":        "Updated",
		"source_repo": "https://github.com/new/repo",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := testutil.ReadJSON(t, resp)
	assert.Equal(t, "Updated", result["name"])
}

func TestDeleteApplication(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "To Delete", "type": "git", "build_type": "dockerfile",
	})
	appID := extractID(app["id"])

	resp := env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s/applications/%s", projectID, appID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify runtime was called
	require.Greater(t, len(env.Runtime.StopCalls), 0)
	require.Greater(t, len(env.Runtime.RemoveCalls), 0)
}

func TestDeployApplication(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Deploy Me", "type": "git", "build_type": "dockerfile",
		"source_repo": "https://github.com/test/repo",
	})
	appID := extractID(app["id"])

	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/deploy", projectID, appID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	result := testutil.ReadJSON(t, resp)
	assert.Equal(t, "pending", result["status"])

	// Verify asynq task was enqueued
	require.Len(t, env.Asynq.Tasks, 1)
	assert.Equal(t, "deploy", env.Asynq.Tasks[0].TypeName)
}

func TestStopStartRestartApplication(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Lifecycle App", "type": "git", "build_type": "dockerfile",
	})
	appID := extractID(app["id"])

	// Stop
	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/stop", projectID, appID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := testutil.ReadJSON(t, resp)
	assert.Equal(t, "stopped", result["status"])
	require.Greater(t, len(env.Runtime.StopCalls), 0)

	// Start
	resp = env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/start", projectID, appID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = testutil.ReadJSON(t, resp)
	assert.Equal(t, "running", result["status"])
	require.Greater(t, len(env.Runtime.StartCalls), 0)

	// Restart
	env.Runtime.StopCalls = nil
	env.Runtime.StartCalls = nil
	resp = env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/restart", projectID, appID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = testutil.ReadJSON(t, resp)
	assert.Equal(t, "running", result["status"])
	require.Greater(t, len(env.Runtime.StopCalls), 0)
	require.Greater(t, len(env.Runtime.StartCalls), 0)
}

func TestBuildApplication(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Build Me", "type": "git", "build_type": "dockerfile",
		"source_repo": "https://github.com/test/repo",
	})
	appID := extractID(app["id"])

	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/applications/%s/build", projectID, appID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	// Verify asynq task was enqueued with "build" type
	require.Len(t, env.Asynq.Tasks, 1)
	assert.Equal(t, "build", env.Asynq.Tasks[0].TypeName)
}
