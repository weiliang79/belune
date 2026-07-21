package handler_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/status"
	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/testutil"
)

// "Needs attention" counts what is still broken, not every failure in a window.
// The property that matters is that it self-clears: a failed deploy followed by
// a successful one is resolved and must stop being counted, while the 7-day
// deploy-success statistic keeps counting both.
func TestNeedsAttention_FailedDeploysAreResolvedBySuccess(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	project := env.CreateProject(t, adminToken, "Attention Project", "attention-project")
	projectID := extractID(project["id"])
	app := env.CreateApplication(t, adminToken, projectID, map[string]any{
		"name": "Attention App", "type": "image", "build_type": "image",
		"source_image": "nginx:latest",
	})
	appID := extractID(app["id"])
	var appUUID pgtype.UUID
	require.NoError(t, appUUID.Scan(appID))

	ctx := context.Background()
	deploy := func(st string) {
		d, err := env.Queries.CreateDeployment(ctx, generated.CreateDeploymentParams{
			ApplicationID: appUUID, Status: status.DeploymentPending, TriggeredBy: "manual",
		})
		require.NoError(t, err)
		_, err = env.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID: d.ID, Status: st,
		})
		require.NoError(t, err)
	}

	attention := func() map[string]any {
		resp := env.DoRequest(t, "GET", "/api/stats", nil, testutil.AuthHeader(adminToken))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		return testutil.ReadJSON(t, resp)["needs_attention"].(map[string]any)
	}

	// A failure with nothing after it is outstanding.
	deploy(status.DeploymentFailed)
	assert.EqualValues(t, 1, attention()["failed_deploys"],
		"an unfixed failed deploy should need attention")

	// A later success resolves it — this is the whole point of the change.
	deploy(status.DeploymentSuccess)
	assert.EqualValues(t, 0, attention()["failed_deploys"],
		"a failure followed by a successful deploy is resolved")

	// The 7-day statistic is unaffected: it still reports both deploys.
	resp := env.DoRequest(t, "GET", "/api/stats", nil, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	deploy7d := testutil.ReadJSON(t, resp)["deploy_7d"].(map[string]any)
	assert.EqualValues(t, 1, deploy7d["failed"],
		"the success-rate statistic stays historical")
	assert.EqualValues(t, 2, deploy7d["total"])

	// Regressing again re-raises it.
	deploy(status.DeploymentFailed)
	assert.EqualValues(t, 1, attention()["failed_deploys"],
		"a new failure after a success needs attention again")

	// A failed deploy usually also flags the application errored. That is one
	// broken app, so it must be one issue: the deploy bucket yields to the
	// errored bucket rather than both counting the same incident.
	_, err := env.Queries.UpdateApplicationStatus(ctx, generated.UpdateApplicationStatusParams{
		ID: appUUID, Status: status.ApplicationError,
	})
	require.NoError(t, err)

	na := attention()
	assert.EqualValues(t, 1, na["error_services"], "the app is errored")
	assert.EqualValues(t, 0, na["failed_deploys"],
		"an errored app must not also be counted as a failed deploy")
	assert.EqualValues(t, 1, na["total"],
		"one broken application is one issue, not two")
}
