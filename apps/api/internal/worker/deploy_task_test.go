package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/testutil"
	"github.com/ungweiliang/selfhost-paas/internal/worker"
)

var (
	testPool    *pgxpool.Pool
	testQueries *generated.Queries
)

func TestMain(m *testing.M) {
	pool, queries, teardown := testutil.SetupTestDB()
	testPool = pool
	testQueries = queries
	code := m.Run()
	teardown()
	os.Exit(code)
}

// failingProxy is a ProxyManager whose AddRoute always fails. Used to drive
// the wire_proxy stage into its compensator path so we can assert that the
// container created in stage 5 gets stopped + removed.
type failingProxy struct {
	mu          sync.Mutex
	addAttempts []string
}

func (p *failingProxy) AddRoute(_ context.Context, cfg proxy.RouteConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.addAttempts = append(p.addAttempts, cfg.Hostname)
	return errors.New("proxy unavailable")
}

func (p *failingProxy) RemoveRoute(_ context.Context, _ string) error             { return nil }
func (p *failingProxy) SetupTLS(_ context.Context, _, _, _, _ string) error       { return nil }
func (p *failingProxy) ListRoutes(_ context.Context) ([]proxy.RouteConfig, error) { return nil, nil }

// seedApp inserts a user, project, application (image type, no health check),
// and deployment (status=pending). Optional opts override fields before insert.
func seedApp(t *testing.T, opts ...func(app *generated.CreateApplicationParams, dep *generated.CreateDeploymentParams)) (generated.Application, generated.Deployment) {
	t.Helper()
	ctx := context.Background()

	suffix := randomSuffix(t)
	user, err := testQueries.CreateUser(ctx, generated.CreateUserParams{
		Email:        "deploy-" + suffix + "@test.com",
		PasswordHash: "x",
		Role:         "admin",
	})
	require.NoError(t, err)

	project, err := testQueries.CreateProject(ctx, generated.CreateProjectParams{
		Name:   "Test Project",
		Slug:   "proj-" + suffix,
		UserID: user.ID,
	})
	require.NoError(t, err)

	appParams := generated.CreateApplicationParams{
		ProjectID:     project.ID,
		Name:          "Test App",
		Slug:          project.Slug + "-app",
		Type:          "image",
		SourceImage:   pgtype.Text{String: "nginx:alpine", Valid: true},
		BuildType:     "image",
		WebhookSecret: pgtype.Text{String: "secret-" + suffix, Valid: true},
	}
	depParams := generated.CreateDeploymentParams{
		Status:      status.DeploymentPending,
		TriggeredBy: "manual",
	}
	for _, fn := range opts {
		fn(&appParams, &depParams)
	}

	app, err := testQueries.CreateApplication(ctx, appParams)
	require.NoError(t, err)

	depParams.ApplicationID = app.ID
	dep, err := testQueries.CreateDeployment(ctx, depParams)
	require.NoError(t, err)

	return app, dep
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	tok, err := crypto.GenerateWebhookSecret()
	require.NoError(t, err)
	return tok[:8]
}

func newTestHandler(rt *testutil.MockContainerRuntime, pm proxy.ProxyManager) *worker.TaskHandler {
	keyring, err := crypto.ParseKeyringEnv("", testutil.TestEncryptionKey, "")
	if err != nil {
		panic("build keyring: " + err.Error())
	}
	return &worker.TaskHandler{
		Runtime: rt,
		Proxy:   pm,
		DB:      testPool,
		Queries: testQueries,
		Keyring: keyring,
		Config: &config.Config{
			ImagePullTimeoutMinutes: 5,
			BuildTimeoutMinutes:     10,
		},
		// QuotaService nil → checkDeployQuota becomes a no-op (intentional).
	}
}

func uuidString(u pgtype.UUID) string {
	b := u.Bytes
	const hex = "0123456789abcdef"
	out := make([]byte, 36)
	idx := 0
	for i, by := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[idx] = '-'
			idx++
		}
		out[idx] = hex[by>>4]
		out[idx+1] = hex[by&0x0f]
		idx += 2
	}
	return string(out)
}

func mustMarshalPayload(t *testing.T, appID, depID pgtype.UUID) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]string{
		"application_id": uuidString(appID),
		"deployment_id":  uuidString(depID),
	})
	require.NoError(t, err)
	return b
}

func TestHandleDeployTask_SkipsAlreadyTerminal(t *testing.T) {
	t.Cleanup(func() { _ = testutil.TruncateAll(context.Background(), testPool) })

	app, dep := seedApp(t, func(_ *generated.CreateApplicationParams, dp *generated.CreateDeploymentParams) {
		dp.Status = status.DeploymentSuccess
	})

	rt := &testutil.MockContainerRuntime{}
	pm := &testutil.MockProxyManager{}
	h := newTestHandler(rt, pm)

	task := asynq.NewTask("deploy", mustMarshalPayload(t, app.ID, dep.ID))
	err := h.HandleDeployTask(context.Background(), task)

	assert.NoError(t, err, "terminal deployment should be skipped silently")
	assert.Empty(t, rt.CreateCalls, "no container should be created when skipping")
	assert.Empty(t, rt.StartCalls, "no container should be started when skipping")
	assert.Empty(t, pm.AddedRoutes, "no proxy route should be added when skipping")
}

func TestHandleDeployTask_CompensatesOnProxyFailure(t *testing.T) {
	t.Cleanup(func() { _ = testutil.TruncateAll(context.Background(), testPool) })

	app, dep := seedApp(t)

	// Wire a domain so wireProxy enters its loop and our failing proxy fires.
	_, err := testQueries.CreateDomain(context.Background(), generated.CreateDomainParams{
		ApplicationID: app.ID,
		Hostname:      "broken.example.com",
		SslEnabled:    false,
		ContainerPort: pgtype.Int4{Int32: 8080, Valid: true},
		ForceHttps:    false,
		SslMode:       "off",
	})
	require.NoError(t, err)

	rt := &testutil.MockContainerRuntime{}
	pm := &failingProxy{}
	h := newTestHandler(rt, pm)

	task := asynq.NewTask("deploy", mustMarshalPayload(t, app.ID, dep.ID))
	err = h.HandleDeployTask(context.Background(), task)

	// HandleDeployTask returns the proxy error so asynq retries.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wire proxy")

	// Container was created and started in stage 5 …
	assert.Len(t, rt.CreateCalls, 1, "container should be created before proxy stage")
	assert.Len(t, rt.StartCalls, 1, "container should be started before proxy stage")

	// … and the compensator must have stopped + removed it after the proxy
	// failure, otherwise we'd leak a half-broken container behind a missing
	// route.
	assert.Contains(t, rt.StopCalls, "mock-container-id",
		"compensator should stop the container on wire_proxy failure")
	assert.Contains(t, rt.RemoveCalls, "mock-container-id",
		"compensator should remove the container on wire_proxy failure")

	// Deployment row should be marked failed (retried=0, maxRetry=0 →
	// failDeployment writes the failure status).
	final, err := testQueries.GetDeployment(context.Background(), dep.ID)
	require.NoError(t, err)
	assert.Equal(t, status.DeploymentFailed, final.Status)
}

func TestHandleDeployTask_HappyPathSkipsHealthCheck(t *testing.T) {
	t.Cleanup(func() { _ = testutil.TruncateAll(context.Background(), testPool) })

	app, dep := seedApp(t) // image type, no domains, no health_check_path

	rt := &testutil.MockContainerRuntime{}
	pm := &testutil.MockProxyManager{}
	h := newTestHandler(rt, pm)

	task := asynq.NewTask("deploy", mustMarshalPayload(t, app.ID, dep.ID))
	err := h.HandleDeployTask(context.Background(), task)
	require.NoError(t, err)

	// Container created + started, no proxy routes (no domains).
	assert.Len(t, rt.CreateCalls, 1)
	assert.Len(t, rt.StartCalls, 1)
	assert.Empty(t, pm.AddedRoutes)

	// Deployment finalized → success; health probe recorded as skipped.
	final, err := testQueries.GetDeployment(context.Background(), dep.ID)
	require.NoError(t, err)
	assert.Equal(t, status.DeploymentSuccess, final.Status)
	assert.True(t, final.HealthStatus.Valid)
	assert.Equal(t, "skipped", final.HealthStatus.String,
		"apps without health_check_path should record skipped, not passing")

	// App row flipped to running.
	appRow, err := testQueries.GetApplication(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Equal(t, status.ApplicationRunning, appRow.Status)
}
