package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiling79/belune/internal/testutil"
)

func TestCreateDatabase(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/databases", projectID), map[string]any{
		"name": "mydb",
		"type": "postgres",
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	result := testutil.ReadJSON(t, resp)
	assert.Equal(t, "mydb", result["name"])
	assert.Equal(t, "postgres", result["type"])
	assert.Equal(t, "creating", result["status"])

	// Verify provision task enqueued
	require.Len(t, env.Asynq.Tasks, 1)
	assert.Equal(t, "provision_db", env.Asynq.Tasks[0].TypeName)
}

func TestGetDatabase(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	// Create database
	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/databases", projectID), map[string]any{
		"name": "mydb",
		"type": "postgres",
	}, testutil.AuthHeader(token))
	db := testutil.ReadJSON(t, resp)
	dbID := extractID(db["id"])

	// Get database — should have decrypted credentials
	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/databases/%s", projectID, dbID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := testutil.ReadJSON(t, resp)
	assert.Equal(t, "mydb", result["name"])

	// Credentials should be present and decrypted
	creds, ok := result["credentials"].(map[string]any)
	require.True(t, ok, "credentials should be a map")
	assert.NotEmpty(t, creds["user"])
	assert.NotEmpty(t, creds["password"])
}

func TestUpdateDatabaseResources(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/databases", projectID), map[string]any{
		"name": "mydb",
		"type": "postgres",
	}, testutil.AuthHeader(token))
	db := testutil.ReadJSON(t, resp)
	dbID := extractID(db["id"])

	// Update resource limits.
	resp = env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/databases/%s", projectID, dbID), map[string]any{
		"cpu_limit":    0.5,
		"memory_limit": 536870912,
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	updated := testutil.ReadJSON(t, resp)
	assert.Equal(t, 0.5, updated["cpu_limit"])
	assert.Equal(t, float64(536870912), updated["memory_limit"])

	// Negative values are rejected.
	resp = env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/databases/%s", projectID, dbID), map[string]any{
		"cpu_limit":    -1,
		"memory_limit": 0,
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestSetDatabaseExternalAccess_RequiresRunning(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/databases", projectID), map[string]any{
		"name": "mydb",
		"type": "postgres",
	}, testutil.AuthHeader(token))
	db := testutil.ReadJSON(t, resp)
	dbID := extractID(db["id"])

	// Provisioning is async + mocked in tests, so the database stays "creating";
	// toggling external access must be refused until it is running.
	resp = env.DoRequest(t, "POST",
		fmt.Sprintf("/api/projects/%s/databases/%s/external-access", projectID, dbID),
		map[string]any{"enabled": true}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteDatabase(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])

	// Create database
	resp := env.DoRequest(t, "POST", fmt.Sprintf("/api/projects/%s/databases", projectID), map[string]any{
		"name": "todelete",
		"type": "redis",
	}, testutil.AuthHeader(token))
	db := testutil.ReadJSON(t, resp)
	dbID := extractID(db["id"])

	// Reset mock calls
	env.Runtime.StopCalls = nil
	env.Runtime.RemoveCalls = nil

	resp = env.DoRequest(t, "DELETE", fmt.Sprintf("/api/projects/%s/databases/%s", projectID, dbID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify runtime stop/remove was called
	require.Greater(t, len(env.Runtime.StopCalls), 0)
	require.Greater(t, len(env.Runtime.RemoveCalls), 0)
}
