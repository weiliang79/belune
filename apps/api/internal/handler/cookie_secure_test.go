package handler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cookieByName returns the named Set-Cookie from a response.
func cookieByName(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// The dashboard's domain — and so whether it is served over HTTPS at all — is
// set from inside the UI at runtime. Session cookies must follow that without
// waiting for someone to edit .env and restart, or they stay non-Secure on an
// HTTPS panel and can leak over a plain-HTTP request.
func TestSessionCookies_SecureFollowsTheRequestScheme(t *testing.T) {
	resetDB(t)
	env.SetupAdmin(t, "admin@test.com", "password123")

	login := func(headers map[string]string) *http.Response {
		return env.DoRequest(t, "POST", "/api/auth/login", map[string]any{
			"email": "admin@test.com", "password": "password123",
		}, headers)
	}

	// Plain HTTP (the bare-IP bootstrap): a Secure cookie would never be sent
	// back, so the operator could not log in at all.
	resp := login(nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	token := cookieByName(resp, "token")
	require.NotNil(t, token)
	assert.False(t, token.Secure, "cookie must not be Secure on a plain-HTTP bootstrap")

	// Behind the proxy on HTTPS: Caddy sets X-Forwarded-Proto to the real scheme,
	// overwriting anything the client sent, so this is trustworthy.
	resp = login(map[string]string{"X-Forwarded-Proto": "https"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	for _, name := range []string{"token", "refresh_token", "csrf_token"} {
		if c := cookieByName(resp, name); c != nil {
			assert.True(t, c.Secure, "%s must be Secure once the panel is on HTTPS", name)
		}
	}
}

func TestSessionCookies_LogoutClearsWithMatchingSecure(t *testing.T) {
	resetDB(t)
	token := env.SetupAdmin(t, "admin@test.com", "password123")

	// A Set-Cookie that clears a cookie must carry the same attributes, or the
	// browser keeps the original and the session is never actually revoked.
	resp := env.DoRequest(t, "POST", "/api/auth/logout", nil, map[string]string{
		"Authorization":     "Bearer " + token,
		"X-Forwarded-Proto": "https",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	cleared := cookieByName(resp, "token")
	require.NotNil(t, cleared)
	assert.Equal(t, -1, cleared.MaxAge)
	assert.True(t, cleared.Secure, "the clearing cookie must match the Secure attribute it is replacing")
}
