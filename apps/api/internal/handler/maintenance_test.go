package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/proxy"
	"github.com/weiliang79/belune/internal/testutil"
)

func TestTriggerCleanup_ForwardsActions(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	env.Asynq.Tasks = nil
	resp := env.DoRequest(t, "POST", "/api/cleanup",
		map[string]any{"actions": []string{"images", "volumes"}},
		testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	require.Len(t, env.Asynq.Tasks, 1)
	var pl map[string]any
	require.NoError(t, json.Unmarshal(env.Asynq.Tasks[0].Payload, &pl))
	actions, ok := pl["actions"].([]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []any{"images", "volumes"}, actions)

	// Invalid action → 400.
	resp = env.DoRequest(t, "POST", "/api/cleanup",
		map[string]any{"actions": []string{"nuke_everything"}},
		testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestReconcileProxy(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	env.Reconciler.ReconcileNowCalls = 0
	env.Reconciler.Status_ = proxy.ReconcilerStatus{LastAdded: 2, LastRemoved: 1, RunCount: 5}

	resp := env.DoRequest(t, "POST", "/api/proxy/reconcile", nil, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)

	assert.Equal(t, 1, env.Reconciler.ReconcileNowCalls)
	assert.EqualValues(t, 2, body["last_added"])
	assert.EqualValues(t, 1, body["last_removed"])
}

func TestQueueStatusAndClear(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	env.Inspector.Info = map[string]*asynq.QueueInfo{
		"critical": {Queue: "critical", Pending: 1, Active: 1, Retry: 2, Archived: 3},
		"low":      {Queue: "low", Retry: 1, Archived: 0},
	}
	env.Inspector.ArchivedByQ = map[string]int{"critical": 3}
	env.Inspector.RetryByQ = map[string]int{"critical": 2, "low": 1}
	env.Inspector.DeleteArchivedCalls = nil
	env.Inspector.DeleteRetryCalls = nil

	// Status: total_stuck = archived+retry across queues = (3+2) + (0+1) = 6.
	resp := env.DoRequest(t, "GET", "/api/maintenance/queue", nil, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	status := testutil.ReadJSON(t, resp)
	assert.EqualValues(t, 6, status["total_stuck"])

	// Clear: deletes archived+retry across the three queues; returns total.
	resp = env.DoRequest(t, "POST", "/api/maintenance/queue/clear", nil, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cleared := testutil.ReadJSON(t, resp)
	assert.EqualValues(t, 6, cleared["cleared"]) // 3 archived + (2+1) retry
	assert.Len(t, env.Inspector.DeleteArchivedCalls, 3)
	assert.Len(t, env.Inspector.DeleteRetryCalls, 3)
}

func TestMaintenance_NonAdminForbidden(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	env.DoRequest(t, "POST", "/api/users", map[string]string{
		"email": "member@test.com", "password": "password123", "role": "member",
	}, testutil.AuthHeader(adminToken)).Body.Close()
	memberToken := env.LoginAs(t, "member@test.com", "password123")

	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/proxy/reconcile"},
		{"GET", "/api/maintenance/queue"},
		{"POST", "/api/maintenance/queue/clear"},
	} {
		resp := env.DoRequest(t, tc.method, tc.path, nil, testutil.AuthHeader(memberToken))
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "%s %s", tc.method, tc.path)
		resp.Body.Close()
	}
}
