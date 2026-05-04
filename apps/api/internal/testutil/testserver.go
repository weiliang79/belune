package testutil

import (
	"net/http/httptest"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/server"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// TestEncryptionKey is a valid 32-byte hex-encoded key for AES-256 in tests.
const TestEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// TestJWTSecret is the JWT secret used in tests.
const TestJWTSecret = "test-jwt-secret-for-integration-tests"

// TestEnv holds all test dependencies.
type TestEnv struct {
	Server   *httptest.Server
	Pool     *pgxpool.Pool
	Queries  *generated.Queries
	Runtime  *MockContainerRuntime
	Proxy    *MockProxyManager
	Asynq    *MockTaskEnqueuer
	Config   *config.Config
	Redis    *redis.Client
	RedisSrv *miniredis.Miniredis
}

// SetupTestServer creates a full HTTP test server with real DB and mock external deps.
// Called from TestMain so does not take *testing.T.
func SetupTestServer(pool *pgxpool.Pool, queries *generated.Queries) *TestEnv {
	keyring, err := crypto.ParseKeyringEnv("", TestEncryptionKey, "")
	if err != nil {
		panic("testutil: build test keyring: " + err.Error())
	}
	cfg := &config.Config{
		Port:                8080,
		JWTSecret:           TestJWTSecret,
		Keyring:             keyring,
		CaddyAdminURL:       "http://localhost:2019",
		DisableRateLimiting: true,
	}

	mockRuntime := &MockContainerRuntime{}
	mockProxy := &MockProxyManager{}
	mockAsynq := &MockTaskEnqueuer{}

	// In-memory Redis so the auth service's revocation paths (per-JTI
	// blacklist + per-user invalidated-after flag) actually exercise their
	// Redis branches under test.
	mr, err := miniredis.Run()
	if err != nil {
		panic("testutil: start miniredis: " + err.Error())
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	srv := server.New(cfg, pool, queries, mockAsynq, mockRuntime, mockProxy, nil, rdb, nil, nil, nil)
	ts := httptest.NewServer(srv.Router())

	return &TestEnv{
		Server:   ts,
		Pool:     pool,
		Queries:  queries,
		Runtime:  mockRuntime,
		Proxy:    mockProxy,
		Asynq:    mockAsynq,
		Config:   cfg,
		Redis:    rdb,
		RedisSrv: mr,
	}
}
