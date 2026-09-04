package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/service"
	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/testutil"
)

// createAPIToken inserts a token directly (there is no create endpoint yet —
// that is PR 3) and returns its plaintext value, ready to use in a Bearer
// header, plus its id.
func createAPIToken(t *testing.T, userID string, roleAtIssue string, expiresAt pgtype.Timestamptz) (plain string, tokenID string) {
	t.Helper()
	var uid pgtype.UUID
	require.NoError(t, uid.Scan(userID))

	plainTok, hash, err := service.GenerateToken()
	require.NoError(t, err)

	row, err := env.Queries.CreateAPIToken(context.Background(), generated.CreateAPITokenParams{
		UserID:      uid,
		Name:        "test token",
		TokenHash:   hash,
		Scopes:      []string{"read", "write"},
		RoleAtIssue: roleAtIssue,
		ExpiresAt:   expiresAt,
	})
	require.NoError(t, err)
	return plainTok, idStr(row.ID)
}

// TestPATAuth_ValidTokenAuthenticates pins the core Bearer branch: a PAT
// reaches a protected endpoint exactly like a session JWT, scoped to its
// owner.
func TestPATAuth_ValidTokenAuthenticates(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	memberID, _ := createMember(t, adminToken, "member@test.com")

	plain, _ := createAPIToken(t, memberID, "member", pgtype.Timestamptz{})

	project := env.CreateProject(t, adminToken, "Admin Project", "admin-project")
	projectID := extractID(project["id"])

	// The token's own owner (a member) cannot reach the admin's project.
	resp := env.DoRequest(t, "GET", fmt.Sprintf("/api/projects/%s", projectID), nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()

	// But the token DOES authenticate as its owner: it can list projects
	// (empty, since the member owns none) rather than 401ing.
	resp = env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, testutil.ReadJSONArray(t, resp), 0)
}

// TestPATAuth_UnknownOrGarbageTokenRejected pins the negative path: a
// PAT-shaped value that does not match any stored hash must 401, not panic
// or fall through to the JWT parser.
func TestPATAuth_UnknownOrGarbageTokenRejected(t *testing.T) {
	resetDB(t)
	env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(service.TokenPrefix+"does-not-exist"))
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

// TestPATAuth_ExpiredTokenRejected pins the expiry check.
func TestPATAuth_ExpiredTokenRejected(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	memberID, _ := createMember(t, adminToken, "member@test.com")

	plain, _ := createAPIToken(t, memberID, "member", pgtype.Timestamptz{
		Time: time.Now().Add(-time.Hour), Valid: true,
	})

	resp := env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

// TestPATAuth_RoleCanOnlyShrink pins the asymmetric role rule: a token whose
// owner was DEMOTED after issue gets the lower role immediately (restricts);
// a token whose owner was PROMOTED after issue does NOT retroactively gain
// the higher role (an existing credential must never silently escalate).
func TestPATAuth_RoleCanOnlyShrink(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	// Demotion: token issued while admin, owner demoted to member afterwards.
	demotedID, _ := createMember(t, adminToken, "demoted@test.com")
	resp := env.DoRequest(t, "PUT", "/api/users/"+demotedID+"/role", map[string]string{"role": "admin"}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
	demotedPlain, _ := createAPIToken(t, demotedID, "admin", pgtype.Timestamptz{})
	resp = env.DoRequest(t, "PUT", "/api/users/"+demotedID+"/role", map[string]string{"role": "member"}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// The token was minted as admin but the owner is a member now — an
	// admin-only endpoint must reject it.
	resp = env.DoRequest(t, "GET", "/api/users", nil, testutil.AuthHeader(demotedPlain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a demoted owner's token must lose admin access immediately")
	resp.Body.Close()

	// Promotion: token issued while member, owner promoted to admin afterwards.
	promotedID, _ := createMember(t, adminToken, "promoted@test.com")
	promotedPlain, _ := createAPIToken(t, promotedID, "member", pgtype.Timestamptz{})
	resp = env.DoRequest(t, "PUT", "/api/users/"+promotedID+"/role", map[string]string{"role": "admin"}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = env.DoRequest(t, "GET", "/api/users", nil, testutil.AuthHeader(promotedPlain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a token minted under a lower role must not gain admin access just because its owner was later promoted")
	resp.Body.Close()
}

// TestPATAuth_LastUsedAtCoarsened pins the write-coarsening rule: two
// requests inside the coarsening window must not produce two writes.
func TestPATAuth_LastUsedAtCoarsened(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	memberID, _ := createMember(t, adminToken, "member@test.com")
	plain, tokenID := createAPIToken(t, memberID, "member", pgtype.Timestamptz{})

	resp := env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(plain))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	var tid pgtype.UUID
	require.NoError(t, tid.Scan(tokenID))

	first := fetchTokenLastUsed(t, tid)
	require.True(t, first.Valid, "last_used_at must be set on first use")

	resp = env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(plain))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	second := fetchTokenLastUsed(t, tid)
	assert.Equal(t, first.Time, second.Time, "a second use inside the coarsening window must not rewrite last_used_at")
}

func fetchTokenLastUsed(t *testing.T, tokenID pgtype.UUID) pgtype.Timestamptz {
	t.Helper()
	var lastUsed pgtype.Timestamptz
	err := env.Pool.QueryRow(context.Background(), "SELECT last_used_at FROM api_tokens WHERE id = $1", tokenID).Scan(&lastUsed)
	require.NoError(t, err)
	return lastUsed
}

// TestAuditLog_AttributesToToken pins the point of PR 2: an audit entry can
// carry the id of the PAT that made it, not just the owning user — the gap
// that made the plan's audit-attribution step necessary before any token
// could be issued. Exercised at the AuditService/DB layer directly rather
// than through a live HTTP request: the integration harness wires
// h.auditSvc as nil (SetupTestServer) to keep every other test free of the
// async writer, so this is the level at which the new column and the
// widened Log() signature are actually reachable.
func TestAuditLog_AttributesToToken(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	memberID, _ := createMember(t, adminToken, "member@test.com")
	_, tokenID := createAPIToken(t, memberID, "member", pgtype.Timestamptz{})

	auditSvc := service.NewAuditService(env.Queries)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go auditSvc.Run(ctx)

	auditSvc.Log(memberID, tokenID, "127.0.0.1", "create_project", "project", "via-token-project", nil)

	var gotUserID, gotTokenID pgtype.UUID
	require.Eventually(t, func() bool {
		row := env.Pool.QueryRow(context.Background(),
			"SELECT user_id, token_id FROM audit_logs WHERE action = 'create_project' AND resource_id = 'via-token-project'")
		return row.Scan(&gotUserID, &gotTokenID) == nil
	}, 2*time.Second, 10*time.Millisecond, "audit entry should appear once the async writer drains it")

	assert.Equal(t, memberID, idStr(gotUserID))
	assert.Equal(t, tokenID, idStr(gotTokenID), "the audit entry must attribute the action to the token, not just the owning user")
}
