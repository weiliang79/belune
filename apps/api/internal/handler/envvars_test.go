package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/testutil"
)

func TestUpdateAndListEnvVars(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")
	project := env.CreateProject(t, token, "Test Project", "test-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, token, projectID, map[string]any{
		"name": "Env App", "type": "git", "build_type": "dockerfile",
	})
	appID := extractID(app["id"])

	// Set env vars
	resp := env.DoRequest(t, "PUT", fmt.Sprintf("/api/projects/%s/applications/%s/env", projectID, appID), map[string]any{
		"vars": []map[string]any{
			{"key": "DATABASE_URL", "value": "postgres://localhost/db", "is_secret": false},
			{"key": "API_KEY", "value": "secret-key-123", "is_secret": true},
		},
	}, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// List env vars
	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications/%s/env", projectID, appID), nil, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	vars := testutil.ReadJSONArray(t, resp)
	require.Len(t, vars, 2)

	// Check that non-secret values are decrypted and secrets are masked
	for _, v := range vars {
		ev := v.(map[string]any)
		key := ev["key"].(string)
		value := ev["value"].(string)

		if key == "DATABASE_URL" {
			assert.Equal(t, "postgres://localhost/db", value)
			assert.Equal(t, false, ev["is_secret"])
		} else if key == "API_KEY" {
			assert.Equal(t, "••••••••", value)
			assert.Equal(t, true, ev["is_secret"])
		}
	}
}
