package service_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/testutil"
)

// startAsynqAgainstMiniredis returns an asynq.Client + Inspector wired to a
// fresh in-process Redis. miniredis speaks the Redis wire protocol, so asynq
// can enqueue/dequeue against it without booting Docker. Caller closes both
// via t.Cleanup.
func startAsynqAgainstMiniredis(t *testing.T) (*asynq.Client, *asynq.Inspector) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	opt := asynq.RedisClientOpt{Addr: mr.Addr()}
	client := asynq.NewClient(opt)
	t.Cleanup(func() { _ = client.Close() })

	inspector := asynq.NewInspector(opt)
	t.Cleanup(func() { _ = inspector.Close() })

	return client, inspector
}

func seedImageApp(t *testing.T) generated.Application {
	t.Helper()
	_, project := seedUserAndProject(t)
	app, err := testQueries.CreateApplication(context.Background(), generated.CreateApplicationParams{
		ProjectID:     project.ID,
		Name:          "Image App",
		Slug:          project.Slug + "-img",
		Type:          "image",
		SourceImage:   pgtype.Text{String: "nginx:alpine", Valid: true},
		BuildType:     "image",
		WebhookSecret: pgtype.Text{String: "secret-" + randomSuffix(t), Valid: true},
	})
	require.NoError(t, err)
	return app
}

func TestDeployService_Deploy_CreatesPendingDeploymentAndEnqueues(t *testing.T) {
	t.Cleanup(func() { truncate(t) })

	app := seedImageApp(t)
	client, inspector := startAsynqAgainstMiniredis(t)
	rt := &testutil.MockContainerRuntime{}
	pm := &testutil.MockProxyManager{}
	svc := service.NewDeployService(rt, pm, testQueries, client, 30)

	dep, err := svc.Deploy(context.Background(), app.ID)
	require.NoError(t, err)

	// Deployment row should be persisted with status=pending so the UI can
	// render it immediately, before the worker picks it up.
	persisted, err := testQueries.GetDeployment(context.Background(), dep.ID)
	require.NoError(t, err)
	assert.Equal(t, status.DeploymentPending, persisted.Status)
	assert.Equal(t, "manual", persisted.TriggeredBy)

	// Task should be in the critical queue with a deterministic TaskID derived
	// from the application ID. The TaskID acts as an asynq-level dedup key:
	// repeated Deploy calls while the first one is still queued must collide
	// with this exact ID, otherwise we'd run multiple deploys in parallel.
	pending, err := inspector.ListPendingTasks("critical")
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, "deploy:"+uuidString(app.ID), pending[0].ID,
		"TaskID must equal deploy:{applicationID} for asynq-level dedup")
	assert.Equal(t, "deploy", pending[0].Type)
}

func TestDeployService_Deploy_SecondCallCollidesOnTaskID(t *testing.T) {
	t.Cleanup(func() { truncate(t) })

	app := seedImageApp(t)
	client, inspector := startAsynqAgainstMiniredis(t)
	rt := &testutil.MockContainerRuntime{}
	pm := &testutil.MockProxyManager{}
	svc := service.NewDeployService(rt, pm, testQueries, client, 30)

	// First Deploy enqueues successfully.
	_, err := svc.Deploy(context.Background(), app.ID)
	require.NoError(t, err)

	// Second concurrent Deploy must surface asynq's TaskIDConflict — we want
	// the caller (HTTP handler / webhook) to see the dedup, not silently
	// queue duplicate work.
	_, err = svc.Deploy(context.Background(), app.ID)
	require.Error(t, err, "duplicate Deploy with the same TaskID should fail")
	assert.Contains(t, err.Error(), "enqueue deploy task")

	pending, err := inspector.ListPendingTasks("critical")
	require.NoError(t, err)
	assert.Len(t, pending, 1, "queue must still hold exactly one task after a duplicate")
}

func TestDeployService_Stop_StopsContainerAndUpdatesAppStatus(t *testing.T) {
	t.Cleanup(func() { truncate(t) })

	app := seedImageApp(t)
	client, _ := startAsynqAgainstMiniredis(t)
	rt := &testutil.MockContainerRuntime{}
	pm := &testutil.MockProxyManager{}
	svc := service.NewDeployService(rt, pm, testQueries, client, 30)

	require.NoError(t, svc.Stop(context.Background(), app.ID))

	// One stop call against the canonical container name; the runtime mock
	// records the exact arg so we can verify the service used the slug from
	// the DB row, not a stale value.
	require.Len(t, rt.StopCalls, 1)
	assert.Contains(t, rt.StopCalls[0], app.Slug,
		"Stop should target a container name derived from the app's current slug")

	persisted, err := testQueries.GetApplication(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Equal(t, status.ApplicationStopped, persisted.Status,
		"Stop must transition the application row to 'stopped'")
}
