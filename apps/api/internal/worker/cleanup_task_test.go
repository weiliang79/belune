package worker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/runtime"
	"github.com/weiliang79/belune/internal/testutil"
	"github.com/weiliang79/belune/internal/worker"
)

// runFullCleanup runs the cleanup exactly as the daily scheduled job does: no
// Actions, which means every step including the orphan container sweep.
func runFullCleanup(t *testing.T, h *worker.TaskHandler) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{})
	require.NoError(t, err)
	require.NoError(t, h.HandleCleanupTask(context.Background(), asynq.NewTask("cleanup", payload)))
}

// TestCleanupOrphanContainers_SparesManagedDatabases is the test whose absence
// let this ship: the sweep lists containers by the managed-by label — which
// databases carry — but built its allowlist from applications alone, so the
// daily run stopped and removed every managed database container. Volumes
// survive, so the data did, but each database needed re-provisioning.
func TestCleanupOrphanContainers_SparesManagedDatabases(t *testing.T) {
	rt := &testutil.MockContainerRuntime{}
	h := newTestHandler(rt, nil)

	// An application as well as a database, so the allowlist is never empty and
	// the empty-allowlist guard cannot be what saves the database here. What is
	// under test is the database being recognised, not the sweep being skipped.
	app, _ := seedApp(t)
	db := seedDatabase(t)
	old := time.Now().Add(-24 * time.Hour)

	// All three carry managed-by=belune and all are past the grace period. Only
	// the last belongs to nothing.
	rt.ListContainers_ = []runtime.ContainerInfo{
		{Name: app.Slug, CreatedAt: old},
		{Name: db.Slug, CreatedAt: old},
		{Name: "left-over-from-a-deleted-app", CreatedAt: old},
	}

	runFullCleanup(t, h)

	assert.NotContains(t, rt.RemoveCalls, db.Slug,
		"a live database's container must survive the orphan sweep")
	assert.NotContains(t, rt.StopCalls, db.Slug,
		"a live database's container must not even be stopped")
	assert.NotContains(t, rt.RemoveCalls, app.Slug,
		"a live application's container must survive too")
	assert.Contains(t, rt.RemoveCalls, "left-over-from-a-deleted-app",
		"a genuine orphan must still be reaped")
}

// TestCleanupOrphanContainers_RefusesAnEmptyAllowlist guards the failure shape
// rather than one missing table. Every container being unrecognised means the
// allowlist did not build, not that the host is full of garbage — and acting on
// that answer is what makes this function dangerous.
func TestCleanupOrphanContainers_RefusesAnEmptyAllowlist(t *testing.T) {
	rt := &testutil.MockContainerRuntime{}
	h := newTestHandler(rt, nil)

	// Worker tests share a database, so the allowlist is only genuinely empty
	// once the rows other tests seeded are gone.
	require.NoError(t, testutil.TruncateAll(context.Background(), testPool))

	// No applications and no databases, but managed containers exist.
	rt.ListContainers_ = []runtime.ContainerInfo{
		{Name: "some-managed-container", CreatedAt: time.Now().Add(-24 * time.Hour)},
	}

	runFullCleanup(t, h)

	assert.Empty(t, rt.RemoveCalls,
		"an empty allowlist must stop the sweep, not authorise removing everything")
}
