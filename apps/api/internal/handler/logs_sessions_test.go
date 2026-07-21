package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/status"
	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/testutil"
)

// TestContainerLogSessions verifies that container logs are grouped into
// per-deployment sessions and can be filtered by a specific deployment or by
// the unassigned ("earlier") bucket.
func TestContainerLogSessions(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	project := env.CreateProject(t, adminToken, "Log Project", "log-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, adminToken, projectID, map[string]any{
		"name": "Log App", "type": "image", "build_type": "image",
		"source_image": "nginx:latest",
	})
	appID := extractID(app["id"])
	var appUUID pgtype.UUID
	require.NoError(t, appUUID.Scan(appID))

	ctx := context.Background()

	// Two deployments (two image generations) plus an unassigned line.
	dep1, err := env.Queries.CreateDeployment(ctx, generated.CreateDeploymentParams{
		ApplicationID: appUUID, Status: status.DeploymentSuccess, TriggeredBy: "manual",
	})
	require.NoError(t, err)
	dep2, err := env.Queries.CreateDeployment(ctx, generated.CreateDeploymentParams{
		ApplicationID: appUUID, Status: status.DeploymentSuccess, TriggeredBy: "rollback",
	})
	require.NoError(t, err)

	insert := func(msg string, dep pgtype.UUID) {
		require.NoError(t, env.Queries.InsertContainerLog(ctx, generated.InsertContainerLogParams{
			SourceType:   "application",
			SourceID:     appUUID,
			Level:        "info",
			Stream:       "stdout",
			Message:      msg,
			DeploymentID: dep,
		}))
	}
	insert("line from dep1", dep1.ID)
	insert("line from dep1 again", dep1.ID)
	insert("line from dep2", dep2.ID)
	insert("orphan line", pgtype.UUID{}) // NULL deployment → "earlier" bucket

	// Sessions endpoint: three groups (dep1, dep2, NULL), each enriched.
	resp := env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications/%s/logs/sessions", projectID, appID), nil, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	sessions := testutil.ReadJSONArray(t, resp)
	require.Len(t, sessions, 3)

	// Locate dep1's session and check its line count + enrichment.
	dep1IDStr := fmt.Sprintf("%x-%x-%x-%x-%x",
		dep1.ID.Bytes[0:4], dep1.ID.Bytes[4:6], dep1.ID.Bytes[6:8], dep1.ID.Bytes[8:10], dep1.ID.Bytes[10:16])
	var found bool
	for _, s := range sessions {
		row := s.(map[string]any)
		if row["deployment_id"] == dep1IDStr {
			found = true
			assert.EqualValues(t, 2, row["line_count"])
			assert.Equal(t, "manual", row["triggered_by"])
		}
	}
	assert.True(t, found, "dep1 session should be present with its metadata")

	// Filter history to dep1: exactly its two lines.
	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications/%s/logs/history?deployment_id=%s", projectID, appID, dep1IDStr), nil, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, testutil.ReadJSONArray(t, resp), 2)

	// Filter to the unassigned bucket: only the orphan line.
	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications/%s/logs/history?deployment_id=none", projectID, appID), nil, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	noneLogs := testutil.ReadJSONArray(t, resp)
	require.Len(t, noneLogs, 1)
	assert.Equal(t, "orphan line", noneLogs[0].(map[string]any)["message"])

	// No filter: all four lines.
	resp = env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s/applications/%s/logs/history", projectID, appID), nil, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, testutil.ReadJSONArray(t, resp), 4)
}
