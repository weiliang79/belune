package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeDomainPath(t *testing.T) {
	cases := map[string]string{
		// The empty string is the one that matters: sqlc sends the column
		// explicitly, so an unset field reaches the database as '' — not as the
		// DEFAULT '/' — and fails the rooted-path check at insert.
		"":         "/",
		"   ":      "/",
		"/":        "/",
		"api":      "/api", // rooted for the operator who typed it bare
		"/api/":    "/api", // trailing slash dropped, so /api and /api/ are one path
		"/api":     "/api",
		"/api/v2/": "/api/v2",
	}
	for in, want := range cases {
		assert.Equal(t, want, normalizeDomainPath(in), "normalizeDomainPath(%q)", in)
	}
}

func TestValidateDomainPath_Accepts(t *testing.T) {
	for _, p := range []string{"/", "/api", "/api/v2", "/a-b_c.d~e"} {
		assert.Empty(t, validateDomainPath(p), "should accept %q", p)
	}
}

func TestValidateDomainPath_Rejects(t *testing.T) {
	// A wildcard is the interesting rejection. The route builder derives "/api"
	// and "/api/*" itself, so an operator who wrote the star would get "/api/*/*"
	// — a matcher that silently matches nothing they meant.
	for _, p := range []string{"/api/*", "/api?x=1", "/api users", "/a//b", "/api#frag"} {
		assert.NotEmpty(t, validateDomainPath(p), "should reject %q", p)
	}
}

func TestValidateDomainPath_RejectsOverlongPath(t *testing.T) {
	long := "/"
	for i := 0; i < 300; i++ {
		long += "a"
	}
	assert.NotEmpty(t, validateDomainPath(long))
}
