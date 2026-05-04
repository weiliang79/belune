package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loginAndExtractRefreshCookie performs a login and returns the access JWT
// plus the refresh_token cookie value. We intentionally read the cookie
// rather than the body field so the assertion exercises the same code path
// as a real browser.
func loginAndExtractRefreshCookie(t *testing.T, email, password string) (string, string) {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, err := http.NewRequest("POST", env.Server.URL+"/api/auth/login", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.Server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	access, _ := payload["token"].(string)
	require.NotEmpty(t, access)

	var refresh string
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			refresh = c.Value
		}
	}
	require.NotEmpty(t, refresh, "login should set refresh_token cookie")
	return access, refresh
}

// postWithRefreshCookie posts to a path with the refresh_token cookie set.
// CSRF is bypassed by using Authorization: Bearer (the CSRF middleware skips
// bearer-auth requests). For /api/auth/refresh we cannot use a bearer token
// because none has been issued yet — the test sets the csrf cookie + header
// together.
func postWithRefreshCookie(t *testing.T, path, refresh string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("POST", env.Server.URL+path, nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refresh})
	// Satisfy CSRF double-submit: send matching cookie + header.
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf"})
	req.Header.Set("X-CSRF-Token", "test-csrf")

	resp, err := env.Server.Client().Do(req)
	require.NoError(t, err)
	return resp
}

func TestLoginLockoutAfterFiveFailures(t *testing.T) {
	resetDB(t)
	env.SetupAdmin(t, "admin@test.com", "password123")

	// Five wrong-password attempts. The 5th should already 429 because the
	// handler re-checks the lockout immediately after recording the failure.
	for i := 1; i <= 4; i++ {
		resp := env.DoRequest(t, "POST", "/api/auth/login", map[string]string{
			"email":    "admin@test.com",
			"password": "wrong",
		}, nil)
		assert.Equalf(t, http.StatusUnauthorized, resp.StatusCode, "attempt %d", i)
		resp.Body.Close()
	}

	// The 5th hits the threshold and the handler returns 429 inline.
	resp := env.DoRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "admin@test.com",
		"password": "wrong",
	}, nil)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	retry := resp.Header.Get("Retry-After")
	require.NotEmpty(t, retry)
	secs, err := strconv.Atoi(retry)
	require.NoError(t, err)
	assert.Greater(t, secs, 0)
	resp.Body.Close()

	// Even the *correct* password is rejected while locked.
	resp = env.DoRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "admin@test.com",
		"password": "password123",
	}, nil)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	resp.Body.Close()
}

func TestLoginLockoutCaseInsensitiveEmail(t *testing.T) {
	resetDB(t)
	env.SetupAdmin(t, "admin@test.com", "password123")

	// Lowercase the email through five failures, then attack with uppercase
	// — both should be locked because we normalise on lookup.
	for i := 0; i < 5; i++ {
		resp := env.DoRequest(t, "POST", "/api/auth/login", map[string]string{
			"email":    "admin@test.com",
			"password": "wrong",
		}, nil)
		resp.Body.Close()
	}

	resp := env.DoRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "ADMIN@TEST.COM",
		"password": "password123",
	}, nil)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	resp.Body.Close()
}

func TestRefreshTokenRotation(t *testing.T) {
	resetDB(t)
	env.SetupAdmin(t, "admin@test.com", "password123")

	_, refresh := loginAndExtractRefreshCookie(t, "admin@test.com", "password123")

	// Refresh should succeed and return a new pair.
	resp := postWithRefreshCookie(t, "/api/auth/refresh", refresh)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotEmpty(t, payload["token"])
	assert.NotEmpty(t, payload["refresh_token"])

	var newRefresh string
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			newRefresh = c.Value
		}
	}
	require.NotEmpty(t, newRefresh)
	assert.NotEqual(t, refresh, newRefresh, "refresh token must rotate on use")

	// Re-using the original (rotated-out) refresh token must fail.
	resp = postWithRefreshCookie(t, "/api/auth/refresh", refresh)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestRefreshFailsWithoutCookie(t *testing.T) {
	resetDB(t)
	env.SetupAdmin(t, "admin@test.com", "password123")

	req, err := http.NewRequest("POST", env.Server.URL+"/api/auth/refresh", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf"})
	req.Header.Set("X-CSRF-Token", "test-csrf")

	resp, err := env.Server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAdminPasswordResetRevokesSessions(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	// Create a member user.
	resp := env.DoRequest(t, "POST", "/api/users", map[string]string{
		"email":    "member@test.com",
		"password": "password123",
		"role":     "member",
		"username": "member",
	}, AuthHeaderWithCSRF(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created := readJSONClose(t, resp)
	memberID, _ := created["id"].(string)
	require.NotEmpty(t, memberID)

	memberToken := env.LoginAs(t, "member@test.com", "password123")

	// Member's token works pre-reset.
	resp = env.DoRequest(t, "GET", "/api/auth/me", nil, AuthHeaderWithCSRF(memberToken))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Admin resets the member's password.
	resp = env.DoRequest(t, "PUT", fmt.Sprintf("/api/users/%s/password", memberID), map[string]string{
		"password": "newpassword123",
	}, AuthHeaderWithCSRF(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// The member's *previous* access token must now be rejected.
	resp = env.DoRequest(t, "GET", "/api/auth/me", nil, AuthHeaderWithCSRF(memberToken))
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// The new password works for a fresh login.
	newToken := env.LoginAs(t, "member@test.com", "newpassword123")
	require.NotEmpty(t, newToken)
}

func TestRoleChangeRevokesSessions(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "admin@test.com", "password123")

	resp := env.DoRequest(t, "POST", "/api/users", map[string]string{
		"email":    "member@test.com",
		"password": "password123",
		"role":     "member",
		"username": "member",
	}, AuthHeaderWithCSRF(adminToken))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created := readJSONClose(t, resp)
	memberID, _ := created["id"].(string)
	require.NotEmpty(t, memberID)

	memberToken := env.LoginAs(t, "member@test.com", "password123")

	resp = env.DoRequest(t, "PUT", fmt.Sprintf("/api/users/%s/role", memberID), map[string]string{
		"role": "admin",
	}, AuthHeaderWithCSRF(adminToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Old token must be rejected after role change.
	resp = env.DoRequest(t, "GET", "/api/auth/me", nil, AuthHeaderWithCSRF(memberToken))
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

// AuthHeaderWithCSRF mirrors AuthHeader but also satisfies the CSRF
// double-submit. Bearer-auth requests are exempt from CSRF, so this exists
// only because the test default uses cookie auth via Authorization header
// which still hits the same Auth middleware path.
func AuthHeaderWithCSRF(token string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + token,
	}
}

func readJSONClose(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}
