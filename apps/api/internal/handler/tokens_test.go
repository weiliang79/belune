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
		"name":   "ci token",
		"scopes": service.AllScopes,
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
		"name":   "list-secret-check",
		"scopes": service.AllScopes,
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
		"name":   "hash-check",
		"scopes": service.AllScopes,
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
		"name":   "   ",
		"scopes": service.AllScopes,
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
		"scopes":          service.AllScopes,
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
		"name":   "forever",
		"scopes": service.AllScopes,
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)
	assert.Nil(t, body["expires_at"])
}

// TestCreateAPIToken_HonorsRequestedScopes pins PR4's picker: the scopes on
// the created token are exactly what the caller asked for, not automatically
// AllScopes (that was PR3, before the picker existed).
func TestCreateAPIToken_HonorsRequestedScopes(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "narrow",
		"scopes": []string{"read"},
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)

	gotScopes, ok := body["scopes"].([]any)
	require.True(t, ok)
	got := make([]string, len(gotScopes))
	for i, s := range gotScopes {
		got[i] = s.(string)
	}
	assert.ElementsMatch(t, []string{"read"}, got)
}

// TestCreateAPIToken_RejectsUnknownScope pins that a scope name outside
// service.AllScopes is rejected, not silently dropped or stored verbatim —
// PR4's enforcement only knows about the four canonical values.
func TestCreateAPIToken_RejectsUnknownScope(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "scope-smuggle",
		"scopes": []string{"admin"},
	}, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// TestCreateAPIToken_RejectsEmptyScopes pins that a token cannot be minted
// with zero scopes — it would authenticate but be able to do nothing, which
// is a footgun rather than a legitimately narrow choice.
func TestCreateAPIToken_RejectsEmptyScopes(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "no-scopes",
		"scopes": []string{},
	}, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// TestCreateAPIToken_DeduplicatesScopes pins that repeating a scope in the
// request does not produce a duplicate entry in the stored array.
func TestCreateAPIToken_DeduplicatesScopes(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "dupe-scopes",
		"scopes": []string{"read", "read", "write"},
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)

	gotScopes := body["scopes"].([]any)
	assert.Len(t, gotScopes, 2)
}

// TestCreateAPIToken_UnpinnedByDefault pins that a PR3-minted token has no
// project_id — every project the owner can reach, evaluated at use time —
// since PR3 ships no project picker either.
func TestCreateAPIToken_UnpinnedByDefault(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "unpinned",
		"scopes": service.AllScopes,
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
		"name":   "admin token",
		"scopes": service.AllScopes,
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
		"name":   "revoke-me",
		"scopes": service.AllScopes,
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
		"name":   "admin-owned",
		"scopes": service.AllScopes,
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
// This is now rejected: minting and revoking tokens requires a live session
// (middleware.RequireSession), specifically because a PAT could otherwise
// mint itself a longer-lived replacement and then revoke the original to
// cover its tracks — a self-propagation path scope enforcement alone cannot
// close, since token creation is a legitimate "write" action. The token used
// to authenticate this request must survive, unaffected by the rejection.
func TestDeleteAPIToken_PATCannotRevokeItself(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	createResp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "self-revoke",
		"scopes": service.AllScopes,
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	created := testutil.ReadJSON(t, createResp)
	id := created["id"].(string)
	plain := created["token"].(string)

	// Authenticate the DELETE with the very token being deleted.
	delResp := env.DoRequest(t, "DELETE", "/api/tokens/"+id, nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusForbidden, delResp.StatusCode)
	delResp.Body.Close()

	// Still alive — the rejection must not have deleted it anyway.
	resp := env.DoRequest(t, "GET", "/api/projects", nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// TestDeleteAPIToken_PATCannotRevokeAnyToken pins that the session
// requirement is not limited to a token deleting itself — a PAT is rejected
// on this route regardless of whose token id it names, including one of its
// own owner's OTHER tokens.
func TestDeleteAPIToken_PATCannotRevokeAnyToken(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	authTokenResp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "authenticator",
		"scopes": service.AllScopes,
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, authTokenResp.StatusCode)
	authPlain := testutil.ReadJSON(t, authTokenResp)["token"].(string)

	targetResp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "target",
		"scopes": service.AllScopes,
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, targetResp.StatusCode)
	targetID := testutil.ReadJSON(t, targetResp)["id"].(string)

	delResp := env.DoRequest(t, "DELETE", "/api/tokens/"+targetID, nil, testutil.AuthHeader(authPlain))
	assert.Equal(t, http.StatusForbidden, delResp.StatusCode)
	delResp.Body.Close()
}

// TestCreateAPIToken_RequiresSession pins the other half of the same fix: a
// PAT cannot mint a replacement token for itself either.
func TestCreateAPIToken_RequiresSession(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	createResp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "authenticator",
		"scopes": service.AllScopes,
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	plain := testutil.ReadJSON(t, createResp)["token"].(string)

	resp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "smuggled-replacement",
		"scopes": service.AllScopes,
	}, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}

// TestListAPITokens_AllowsPATAuth pins that the session requirement is
// deliberately narrow: listing is read-only, so a PAT may still call it —
// only minting and revoking are gated.
func TestListAPITokens_AllowsPATAuth(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	createResp := env.DoRequest(t, "POST", "/api/tokens", map[string]any{
		"name":   "list-via-pat",
		"scopes": service.AllScopes,
	}, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	plain := testutil.ReadJSON(t, createResp)["token"].(string)

	resp := env.DoRequest(t, "GET", "/api/tokens", nil, testutil.AuthHeader(plain))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, testutil.ReadJSONArray(t, resp), 1)
	resp.Body.Close()
}
