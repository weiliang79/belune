package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// scopeGrants maps a required scope to the token scopes that satisfy it.
// "write" is the umbrella: it satisfies a "read" or "deploy" requirement too,
// since a general-purpose token (today, every token — no picker existed
// before this) can already do everything a narrower one can. "read" in turn
// satisfies "metrics", since metrics are just a narrower slice of what a
// general read already covers. "deploy" ALSO satisfies "read" — the design's
// own CI use case ("let CI deploy app X") almost always polls the deployment
// afterward, so a deploy-only token that could trigger a deploy but never
// observe it would be a self-inflicted footgun, not a meaningful narrowing.
// The remaining direction never holds: deploy does NOT imply write (a CI
// deploy token must not also be able to rewrite env vars or delete a
// backup), and metrics does not imply read or deploy (a Prometheus scrape
// token must not be able to read arbitrary project data or trigger a
// deploy) — capability only ever shrinks along those edges, never
// round-trips.
var scopeGrants = map[string][]string{
	"read":    {"read", "write", "deploy"},
	"write":   {"write"},
	"deploy":  {"deploy", "write"},
	"metrics": {"metrics", "read", "write"},
}

func scopeSatisfies(tokenScopes []string, required string) bool {
	granting := scopeGrants[required]
	for _, s := range tokenScopes {
		for _, g := range granting {
			if s == g {
				return true
			}
		}
	}
	return false
}

// writeForbiddenScope writes the standard "token lacks scope" 403. Kept as
// one place so RequireScope and RequireScopeByMethod produce an identical
// body regardless of how the requirement was derived.
func writeForbiddenScope(w http.ResponseWriter, required string) {
	http.Error(w, `{"error":"token lacks required scope: `+required+`"}`, http.StatusForbidden)
}

// RequireScope returns a middleware that rejects a PAT lacking the given
// scope. A session JWT always passes — ScopesFromContext returns nil for one,
// and a session implies every scope. Must be used after Auth.
func RequireScope(required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scopes := ScopesFromContext(r.Context())
			if scopes == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !scopeSatisfies(scopes, required) {
				writeForbiddenScope(w, required)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireScopeByMethod returns a middleware that requires "read" for a safe
// method (GET/HEAD/OPTIONS) and "write" for everything else. This is the
// default every ordinary CRUD route gets — a new route added under a group
// carrying this middleware needs no per-route scope annotation to be safe by
// default. Routes needing a narrower or different scope (deploy actions,
// metrics reads) are registered in their own group with RequireScope instead
// of this one — see routes.go.
func RequireScopeByMethod() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			required := "write"
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				required = "read"
			}
			scopes := ScopesFromContext(r.Context())
			if scopes == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !scopeSatisfies(scopes, required) {
				writeForbiddenScope(w, required)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireProjectAccess returns a middleware that rejects a request whose
// {projectId} URL param does not match a project-pinned PAT's pin. A route
// with no projectId param, or a token that isn't pinned (every existing
// token today, and a session always), passes through untouched. Must be used
// after Auth, and after chi has parsed URL params (i.e. inside the route
// tree, not before it).
func RequireProjectAccess() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pinned := TokenProjectFromContext(r.Context())
			if pinned == "" {
				next.ServeHTTP(w, r)
				return
			}
			if requested := chi.URLParam(r, "projectId"); requested != "" && requested != pinned {
				http.Error(w, `{"error":"token is pinned to a different project"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
