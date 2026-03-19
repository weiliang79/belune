# Integration Testing

## Prerequisites

- **Docker** must be running (the tests use [testcontainers-go](https://golang.testcontainers.org/) to spin up a real PostgreSQL 16 container)
- **Go 1.23+**

No other services (Redis, Caddy, etc.) are needed — Docker runtime, Caddy proxy, and the Asynq task queue are all mocked.

## Running Tests

From the `apps/api/` directory:

```bash
# Run all integration tests
go test -v -count=1 -timeout=300s ./internal/handler/...

# Run a specific test file (e.g., auth tests only)
go test -v -count=1 -timeout=300s ./internal/handler/ -run "TestSetup|TestLogin|TestMe|TestChangePassword"

# Run a single test
go test -v -count=1 -timeout=300s ./internal/handler/ -run TestWebhookPush_GitHub
```

The first run may take longer as Docker pulls the `postgres:16-alpine` image. Subsequent runs reuse the cached image and typically complete in ~12 seconds.

## What's Tested

| Test File | Area | Tests |
|-----------|------|-------|
| `auth_test.go` | Authentication | Setup flow, login, /me endpoint, password change |
| `users_test.go` | User Management | CRUD, role updates, admin-only access, last-admin protection |
| `projects_test.go` | Projects | CRUD, owner-based access control, admin access, transfer, cascade delete |
| `applications_test.go` | Applications | CRUD, deploy, stop/start/restart, build, task enqueue verification |
| `databases_test.go` | Databases | Create with encrypted credentials, get with decryption, delete |
| `domains_test.go` | Domains | Add/remove/list, proxy mock verification |
| `envvars_test.go` | Env Variables | Set and list with AES encryption/decryption, secret masking |
| `webhooks_test.go` | Webhooks | GitHub push-to-deploy with HMAC, branch mismatch filtering |
| `metrics_test.go` | Admin Endpoints | Metrics (admin-only), cleanup task trigger |

## Architecture

```
internal/
├── testutil/
│   ├── testutil.go       # PostgreSQL testcontainer setup, migration runner, TruncateAll
│   ├── testserver.go     # Creates httptest.Server with real Chi router + mocked externals
│   ├── mocks.go          # MockContainerRuntime, MockProxyManager, MockTaskEnqueuer
│   └── fixtures.go       # Helpers: SetupAdmin, LoginAs, CreateProject, DoRequest, etc.
├── migrations/
│   └── embed.go          # go:embed for SQL migration files
└── handler/
    └── *_test.go         # Integration test files
```

### How It Works

1. **TestMain** (in `auth_test.go`) runs once per test package:
   - Spins up a PostgreSQL 16 container via testcontainers
   - Runs all `.up.sql` migrations
   - Creates a full HTTP test server (`httptest.Server`) with the real Chi router
   - External dependencies (Docker, Caddy, Asynq) are replaced with in-memory mocks

2. **Each test** calls `resetDB(t)` which:
   - Truncates all database tables (`CASCADE`) for isolation
   - Resets mock call recordings (stop/start/remove calls, enqueued tasks, proxy routes)

3. **Tests make real HTTP requests** to the test server and assert on:
   - HTTP status codes
   - JSON response bodies
   - Database state (via subsequent API calls)
   - Mock interactions (e.g., "was StopContainer called?", "was a deploy task enqueued?")

### Mocking Strategy

| Component | Real or Mocked | Why |
|-----------|---------------|-----|
| PostgreSQL | **Real** (testcontainer) | Tests actual SQL queries, migrations, and constraints |
| Chi Router + Middleware | **Real** | Tests full request lifecycle including auth and RBAC |
| Docker Runtime | **Mocked** | No need for real containers; tests verify correct calls |
| Caddy Proxy | **Mocked** | No need for real proxy; tests verify route add/remove |
| Asynq (task queue) | **Mocked** | Tests verify tasks are enqueued with correct type/payload |
| Redis (pub/sub) | **Skipped** (nil) | Only used for SSE build log streaming, not tested |

## Writing New Tests

1. Add your test function to the appropriate `*_test.go` file (or create a new one in `internal/handler/`)
2. Start with `resetDB(t)` for a clean database
3. Use fixture helpers to set up state:

```go
func TestMyFeature(t *testing.T) {
    resetDB(t)
    token := env.SetupAdmin(t, "admin@test.com", "password123")
    project := env.CreateProject(t, token, "My Project", "my-project")
    projectID := extractID(project["id"])

    // Make requests
    resp := env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s", projectID), nil, testutil.AuthHeader(token))
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    result := testutil.ReadJSON(t, resp)
    assert.Equal(t, "My Project", result["name"])

    // Verify mock interactions
    assert.Len(t, env.Asynq.Tasks, 0)
}
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `failed to start postgres container` | Ensure Docker is running |
| Tests hang on first run | Docker is pulling `postgres:16-alpine` — wait for it |
| `connection refused` errors | The testcontainer may not be ready; increase `WithStartupTimeout` |
| Flaky test failures | Check if `resetDB(t)` is called at the start of each test |
