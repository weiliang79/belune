package handler_test

import (
	"context"
	"crypto/sha256"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/service"
	"github.com/weiliang79/belune/internal/testutil"
)

// TestCreateAPIToken_ReturnsPlaintextOnceAndAuthenticates pins the create
// endpoint's core contract: the plaintext comes back exactly on this
// response, and it works as a Bearer credential immediately.
func TestCreateAPIToken_ReturnsPlaintextOnceAndAuthenticates(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "ci token",
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)

	plain, ok := body["token"].(string)
	require.True(t, ok, "create response must carry the plaintext token")
	assert.True(t, service.HasTokenPrefix(plain))
	assert.Equal(t, "ci token", body["name"])

	listResp := env.DoRequest(t, "GET", "/api/tokens", nil, testutil.AuthHeader(plain))
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	assert.Len(t, testutil.ReadJSONArray(t, listResp), 1, "the newly minted token must authenticate on its own")
}

// TestCreateAPIToken_ListNeverCarriesSecret pins that no endpoint past
// creation ever exposes the plaintext or the hash — the list DTO is a
// deliberately narrower shape than the stored row.
func TestCreateAPIToken_ListNeverCarriesSecret(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	createResp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "list-secret-check",
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	created := testutil.ReadJSON(t, createResp)
	plain := created["token"].(string)

	listResp := env.DoRequest(t, "GET", "/api/tokens", nil, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	items := testutil.ReadJSONArray(t, listResp)
	require.Len(t, items, 1)
	row := items[0].(map[string]any)

	_, hasToken := row["token"]
	_, hasHash := row["token_hash"]
	assert.False(t, hasToken, "the list must never carry the plaintext")
	assert.False(t, hasHash, "the list must never carry the hash")
	assert.NotEqual(t, plain, row["id"], "sanity: the id is not somehow the plaintext")
}

// TestCreateAPIToken_StoresSHA256HashOfPlaintext pins the storage format
// itself: what lands in token_hash is SHA-256 of the plaintext, not the
// plaintext, and not something reversible.
func TestCreateAPIToken_StoresSHA256HashOfPlaintext(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "hash-check",
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)
	plain := body["token"].(string)
	id := body["id"].(string)

	var idUUID pgtype.UUID
	require.NoError(t, idUUID.Scan(id))

	var storedHash []byte
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		"SELECT token_hash FROM api_tokens WHERE id = $1", idUUID).Scan(&storedHash))

	wantHash := sha256.Sum256([]byte(plain))
	assert.Equal(t, wantHash[:], storedHash)
}

// TestCreateAPIToken_RequiresName pins basic validation: an empty (or
// all-whitespace) name is rejected before anything is minted.
func TestCreateAPIToken_RequiresName(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "   ",
	}, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// TestCreateAPIToken_RejectsExpiryOutsideThePicker pins that the API enforces
// the same 1/7/14/30/60/90 list the UI offers — a client cannot request an
// expiry the UI never presents.
func TestCreateAPIToken_RejectsExpiryOutsideThePicker(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":            "bad-expiry",
		"expires_in_days": 5,
	}, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// TestCreateAPIToken_OmittedExpiryNeverExpires pins the "no expiry" choice:
// leaving expires_in_days out entirely (not zero) must store NULL.
func TestCreateAPIToken_OmittedExpiryNeverExpires(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "forever",
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)
	assert.Nil(t, body["expires_at"])
}

// TestCreateAPIToken_IgnoresClientSuppliedScopes is the structural half of
// the PR3 decision recorded in project_v016_plan: there is no scope picker,
// and this proves it is not merely absent from the UI — a client that sends a
// scopes field anyway gets AllScopes back, never what it asked for.
func TestCreateAPIToken_IgnoresClientSuppliedScopes(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "scope-smuggle",
		"scopes": []string{"admin"},
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)

	gotScopes, ok := body["scopes"].([]any)
	require.True(t, ok)
	got := make([]string, len(gotScopes))
	for i, s := range gotScopes {
		got[i] = s.(string)
	}
	assert.ElementsMatch(t, service.AllScopes, got)
}

// TestCreateAPIToken_UnpinnedByDefault pins that a PR3-minted token has no
// project_id — every project the owner can reach, evaluated at use time —
// since PR3 ships no project picker either.
func TestCreateAPIToken_UnpinnedByDefault(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "unpinned",
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)
	id := body["id"].(string)

	var idUUID pgtype.UUID
	require.NoError(t, idUUID.Scan(id))
	var projectID pgtype.UUID
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		"SELECT project_id FROM api_tokens WHERE id = $1", idUUID).Scan(&projectID))
	assert.False(t, projectID.Valid)
}

// TestListAPITokens_ScopedToCallingUser pins per-user isolation: there is no
// cross-user token oversight view in v1, admin or not.
func TestListAPITokens_ScopedToCallingUser(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	_, memberToken := createMember(t, adminToken, "member@test.com")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "admin token",
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	memberList := env.DoRequest(t, "GET", "/api/tokens", nil, testutil.AuthHeader(memberToken))
	require.Equal(t, http.StatusOK, memberList.StatusCode)
	assert.Len(t, testutil.ReadJSONArray(t, memberList), 0, "a member must not see the admin's tokens")

	adminList := env.DoRequest(t, "GET", "/api/tokens", nil, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, adminList.StatusCode)
	assert.Len(t, testutil.ReadJSONArray(t, adminList), 1)
}

// TestDeleteAPIToken_OwnerCanDeleteAndItStopsAuthenticating pins the full
// revoke path end to end.
func TestDeleteAPIToken_OwnerCanDeleteAndItStopsAuthenticating(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	createResp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "revoke-me",
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	created := testutil.ReadJSON(t, createResp)
	id := created["id"].(string)
	plain := created["token"].(string)

	delResp := env.DoRequest(t, "DELETE", "/api/tokens/"+id, nil, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, delResp.StatusCode)
	delResp.Body.Close()

	resp := env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "a deleted token must stop authenticating immediately")
	resp.Body.Close()
}

// TestDeleteAPIToken_CannotDeleteAnotherUsersToken pins that ownership is
// enforced by the query shape itself (WHERE id = $1 AND user_id = $2), not a
// separate check that could be forgotten on a future endpoint.
func TestDeleteAPIToken_CannotDeleteAnotherUsersToken(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")
	_, memberToken := createMember(t, adminToken, "member@test.com")

	createResp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "admin-owned",
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	created := testutil.ReadJSON(t, createResp)
	id := created["id"].(string)
	plain := created["token"].(string)

	delResp := env.DoRequest(t, "DELETE", "/api/tokens/"+id, nil, testutil.AuthHeader(memberToken))
	assert.Equal(t, http.StatusNotFound, delResp.StatusCode, "a token belonging to someone else must read as not-found, not forbidden")
	delResp.Body.Close()

	// Still very much alive.
	resp := env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// TestDeleteAPIToken_UnknownIDNotFound pins the not-found path for an id that
// simply does not exist.
func TestDeleteAPIToken_UnknownIDNotFound(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "DELETE", "/api/tokens/00000000-0000-0000-0000-000000000000", nil, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// TestDeleteAPIToken_SelfRevocationSucceeds is a regression test for a window
// project_v016_plan flagged explicitly: a token deleting itself (the same
// credential authenticating the DELETE request that removes its own row).
// Auth already resolved and populated the request context before the handler
// runs, so this must succeed exactly like deleting any other token — nothing
// about "am I currently authenticating with this" should matter.
func TestDeleteAPIToken_SelfRevocationSucceeds(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	createResp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name": "self-revoke",
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	created := testutil.ReadJSON(t, createResp)
	id := created["id"].(string)
	plain := created["token"].(string)

	// Authenticate the DELETE with the very token being deleted.
	delResp := env.DoRequest(t, "DELETE", "/api/tokens/"+id, nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusOK, delResp.StatusCode)
	delResp.Body.Close()

	resp := env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}
