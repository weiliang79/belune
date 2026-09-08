// apidoc_generate_test.go generates apps/site/content/docs/api/*.mdx
// by walking the REAL registered router and probing every route with a
// personal access token — the same mechanism destroy_boundary_test.go and
// reveal_boundary_test.go already use, extended from "does this route reject
// a PAT" to "what scope does this route need, and is it session-only". It
// documents OBSERVED behavior, not a reading of routes.go, so it cannot drift
// the way a hand-written reference already has twice this month (the /metrics
// scope mismatch, the GetDatabase reveal gap).
//
// Not a correctness test — a generation tool that happens to reuse the test
// harness (a real Postgres via testcontainers; Docker, Redis, and asynq are
// mocked, see testutil.SetupTestServer, so even a route this generator's
// full-scope probe actually executes never touches anything outside this
// ephemeral run). Skipped unless GENERATE_API_REFERENCE=1 is set, so it never
// runs as part of the ordinary suite. Invoke via `task generate:api-docs`.
//
// Two probes per route, in the same spirit as the RequireScope/RequireSession
// split those boundary tests already rely on:
//
//  1. A ZERO-SCOPE token (scopes = []string{}, non-nil-but-empty — minted by
//     inserting directly via queries.CreateAPIToken, since POST /api/tokens
//     itself now rejects an empty scopes array). RequireScope/RequireScopeByMethod
//     reject it with "token lacks required scope: X", naming the requirement
//     directly — and the handler NEVER RUNS, because scopeSatisfies's loop
//     over an empty slice can't match anything. This probe is SAFE ON EVERY
//     ROUTE, no exceptions, and is the only one skip-listed routes still get.
//
//  2. A FULL-SCOPE token (service.AllScopes — satisfies every RequireScope*
//     check by construction). This probe reveals RequireSession (the route
//     executes past the scope gate and hits the ", not a personal access
//     token" message), but on any route that isn't otherwise gated, it
//     EXECUTES THE HANDLER. With a dummy UUID substituted for path params, an
//     id-having route 404s harmlessly first. The routes that don't get that
//     protection are id-less mutating ones — withheld from this probe
//     entirely; see apidocSkipFullScopeReason. Skipping costs nothing but
//     Session accuracy for that one row: the zero-scope probe already gave
//     the scope, and the row is still emitted, marked "not probed" — never
//     silently dropped.
//
// Admin-role gating is NOT probed — it's read structurally off the same
// chi.Walk call, for a reason specific to what reflection can and can't
// recover: RequireRole("admin") and RequireSession are parameterless/fixed
// (their presence in the middleware chain is the whole story, and every
// RequireRole call in this codebase names "admin", never anything else), so
// a function-name check via runtime.FuncForPC is exact and free — no second
// HTTP round trip earns anything. RequireScope(required) is different: it's
// parameterized by a scope STRING captured in the closure, which a function
// name can't recover without reaching into unexported runtime internals —
// that's the one thing only a live probe can actually tell you, which is
// exactly why probing is this generator's primary technique for scope, not a
// stylistic choice.
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/service"
	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/testutil"
)

// apidocErrorMessage reads a response's "error" field if it's a 403, without
// assuming the body is any particular shape otherwise — a 200 on a list
// endpoint is a JSON ARRAY, not an object (testutil.ReadJSON would panic on
// exactly that), and this generator only ever needs the body on a 403.
func apidocErrorMessage(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		return ""
	}
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return "" // non-JSON 403 (shouldn't happen on this router, but never fatal)
	}
	return body.Error
}

// apidocRoute is one route's fully observed (or structurally read, for the
// admin flag) shape.
type apidocRoute struct {
	Method       string
	Path         string
	Handler      string
	Scope        string // "read"/"write"/"deploy"/"metrics"; see the UNRECOGNIZED fallback below if nothing matched
	Session      bool   // RequireSession observed via the full-scope probe
	Admin        bool   // RequireRole("admin") read from the middleware chain
	Pinned       bool   // path contains {projectId} — RequireProjectAccess applies
	NotProbed    bool   // full-scope probe withheld; Session is unknown, not false
	NotProbedWhy string // human-readable reason, only meaningful when NotProbed
}

// apidocSkipPublicPrefixes are the routes with no Auth() middleware at all —
// a PAT has no meaning on them (Authorization is either ignored, or for the
// deploy-hook/webhook routes not even a bearer credential at all), so there
// is no scope/session story to document. Small and stable: a new PUBLIC
// route is a rare, deliberate, security-relevant decision that gets scrutiny
// on its own, unlike the destroy/reveal boundaries this project has actually
// gotten wrong twice.
var apidocSkipPublicPrefixes = []string{
	"/healthz",
	"/api/auth/login",
	"/api/auth/login/verify",
	"/api/auth/setup",
	"/api/features",
	"/api/version",
	"/api/auth/refresh",
	"/api/auth/forgot-password",
	"/api/auth/reset-password",
	"/api/auth/invitation",
	"/api/auth/accept-invitation",
	"/api/webhooks/push",
	"/api/git/webhooks/",
	"/api/webhooks/deploy/",
	"/api/git/providers/github/manifest/callback",
	"/api/git/integrations/callback",
}

func apidocIsPublic(path string) bool {
	for _, p := range apidocSkipPublicPrefixes {
		if path == p || strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// apidocSkipFullScopeReason decides whether it's safe to let the full-scope
// probe actually execute a route's handler, returning "" when it is and a
// human-readable reason when it isn't. Two independent hazards, both derived
// structurally rather than off a hand-typed list of specific paths — the
// "hand-maintained list rots" lesson this project already applies to
// destroy_boundary_test.go's own resource set:
//
//   - An id-less MUTATING route: with no existing resource for a dummy UUID
//     to 404 against, the handler runs to completion. GET/HEAD/OPTIONS are
//     always safe regardless of path shape (RequireScopeByMethod treats them
//     as read, and nothing in this codebase mutates on a safe method); an
//     id-having route is always safe too — a dummy UUID 404s first.
//   - A STREAMING route (SSE): the connection never closes on its own, so a
//     plain bounded GET blocks forever reading a body that has no EOF — found
//     empirically, this generator's first run hung on StreamHostMetrics.
//     Every streaming handler in this codebase is named Stream*, checked on
//     the handler name reflection already extracts rather than a second
//     hand-typed path list.
func apidocSkipFullScopeReason(method, path, handlerName string) string {
	if strings.HasPrefix(handlerName, "Stream") {
		return "streaming (SSE) route — the connection never closes, so a bounded probe can't observe it"
	}
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return ""
	}
	if strings.Contains(path, "{") {
		return ""
	}
	return "id-less mutating route, skipped for safety — see apidocSkipFullScopeReason"
}

// apidocFuncName returns the short name of the function a value wraps —
// reliable for this because Go compiles one machine-code body per closure
// literal in the source, shared across every instantiation; the returned
// symbol name is the declaring function's, not per-call-site.
func apidocFuncName(v any) string {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Func {
		return ""
	}
	full := runtime.FuncForPC(rv.Pointer()).Name()
	// "github.com/weiliang79/belune/internal/handler.(*Handler).GetProject-fm"
	// -> "GetProject". The "-fm" suffix marks a method value; trim it too.
	full = strings.TrimSuffix(full, "-fm")
	if i := strings.LastIndex(full, "."); i >= 0 {
		full = full[i+1:]
	}
	return full
}

// apidocMiddlewareContains reports whether any middleware in the chain was
// declared by (i.e. is a closure returned from) the named function — matched
// on the declaring function's own name, e.g. "RequireRole", regardless of
// package-path prefix or closure suffix.
func apidocMiddlewareContains(mws []func(http.Handler) http.Handler, declaredBy string) bool {
	for _, mw := range mws {
		name := runtime.FuncForPC(reflect.ValueOf(mw).Pointer()).Name()
		if strings.Contains(name, "."+declaredBy+".") {
			return true
		}
	}
	return false
}

// apidocZeroScopeToken inserts a token with a non-nil, empty scopes array
// directly via the queries layer — POST /api/tokens itself now rejects an
// empty scopes array (TestCreateAPIToken_RejectsEmptyScopes), so the public
// API can't mint this one. Same bypass createPinnedAPIToken already uses for
// pinned tokens; not a new pattern.
func apidocZeroScopeToken(t *testing.T, userID string) string {
	t.Helper()
	var uid pgtype.UUID
	require.NoError(t, uid.Scan(userID))

	plain, hash, err := service.GenerateToken()
	require.NoError(t, err)

	_, err = env.Queries.CreateAPIToken(context.Background(), generated.CreateAPITokenParams{
		UserID:      uid,
		Name:        "apidoc-zero-scope-probe",
		TokenHash:   hash,
		Scopes:      []string{},
		RoleAtIssue: "admin",
	})
	require.NoError(t, err)
	return plain
}

// substituteDummyIDs replaces every {param} segment with a well-formed but
// nonexistent UUID — the same technique destroy_boundary_test.go and
// reveal_boundary_test.go already use, so an id-having route 404s cleanly
// instead of touching a real row.
func substituteDummyIDs(route string) string {
	path := route
	for _, seg := range strings.Split(route, "/") {
		if strings.HasPrefix(seg, "{") {
			path = strings.Replace(path, seg, "00000000-0000-0000-0000-000000000000", 1)
		}
	}
	return path
}

// mustAuthMe fetches the caller's own user record via the one endpoint that
// doesn't need an id to look itself up.
func mustAuthMe(t *testing.T, sessionToken string) map[string]any {
	t.Helper()
	resp := env.DoRequest(t, "GET", "/api/auth/me", nil, testutil.AuthHeader(sessionToken))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return testutil.ReadJSON(t, resp)
}

func TestGenerateAPIReference(t *testing.T) {
	if os.Getenv("GENERATE_API_REFERENCE") != "1" {
		t.Skip("generation tool, not a correctness test — set GENERATE_API_REFERENCE=1 to run")
	}
	resetDB(t)

	adminToken := env.SetupAdmin(t, "apidoc-admin@test.com", "password123")
	adminUserID := extractID(mustAuthMe(t, adminToken)["id"])

	zeroScope := apidocZeroScopeToken(t, adminUserID)
	fullScope := mintScoped(t, adminToken, service.AllScopes)

	router, ok := env.Server.Config.Handler.(chi.Routes)
	require.True(t, ok, "test server handler must be walkable as chi.Routes")

	var routes []apidocRoute
	err := chi.Walk(router, func(method, route string, handler http.Handler, mws ...func(http.Handler) http.Handler) error {
		// "/*" is chi's own internal NotFound/MethodNotAllowed fallback, not
		// an application route — it never reaches Auth() at all, which is
		// exactly why it shows up 200 for every method with no scope check.
		if route == "/*" || apidocIsPublic(route) {
			return nil
		}

		row := apidocRoute{
			Method:  method,
			Path:    route,
			Handler: apidocFuncName(handler),
			Pinned:  strings.Contains(route, "{projectId}"),
			Admin:   apidocMiddlewareContains(mws, "RequireRole"),
		}

		path := substituteDummyIDs(route)

		// Probe 1: zero-scope. Safe on every route — the handler never runs,
		// since an empty scopes slice can't satisfy any RequireScope check.
		resp := env.DoRequest(t, method, path, nil, testutil.AuthHeader(zeroScope))
		status1 := resp.StatusCode
		msg := apidocErrorMessage(t, resp)
		switch {
		case strings.HasPrefix(msg, "token lacks required scope: "):
			row.Scope = strings.TrimPrefix(msg, "token lacks required scope: ")
		case msg == "this action requires a session, not a personal access token":
			// No RequireScope* sits in front of RequireSession on this route
			// (e.g. the WS terminal tunnel) — a PAT is rejected before scope
			// ever comes into it, at ANY scope including zero. Reading
			// routes.go would have called this "read" or "write" by the
			// blanket default; probing catches that there is no blanket
			// default here at all.
			row.Scope = "none — session gate applies regardless of scope"
			row.Session = true
		}
		if row.Scope == "" {
			// Every authenticated, non-public route in this codebase sits
			// behind some RequireScope* check — an empty result here means
			// the zero-scope probe was rejected for a DIFFERENT reason (or
			// wasn't rejected at all), which is itself worth surfacing
			// loudly rather than silently emitting an incomplete row.
			row.Scope = fmt.Sprintf("UNRECOGNIZED (%d %q)", status1, msg)
		}

		// Probe 2: full-scope, only where it's safe to let it actually run.
		if why := apidocSkipFullScopeReason(method, route, row.Handler); why != "" {
			row.NotProbed = true
			row.NotProbedWhy = why
		} else {
			resp2 := env.DoRequest(t, method, path, nil, testutil.AuthHeader(fullScope))
			if apidocErrorMessage(t, resp2) == "this action requires a session, not a personal access token" {
				row.Session = true
			}
		}

		routes = append(routes, row)
		return nil
	})
	require.NoError(t, err)

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})

	t.Logf("apidoc: probed %d routes", len(routes))
	for _, r := range routes {
		if strings.HasPrefix(r.Scope, "UNRECOGNIZED") {
			t.Errorf("apidoc: %s %s — %s", r.Method, r.Path, r.Scope)
		}
	}

	writeAPIReferenceMDX(t, routes)
}

// apidocDomain is one output page: a title plus the rows that classified
// into it, in probe order (already path-sorted by the caller).
type apidocDomain struct {
	Slug, Title string
	Rows        []apidocRoute
}

// apidocDomainOrder is the generated reference's table of contents — a
// curated presentation order, not a safety mechanism. Unlike
// apidocSkipPublicPrefixes or apidocSkipFullScopeReason, getting this list
// wrong costs nothing but tidiness: apidocClassify's default case ("uncategorized")
// guarantees a route can never be silently dropped for lack of a matching
// domain, only poorly filed until someone adds a rule for it.
var apidocDomainOrder = []struct{ slug, title string }{
	{"live-updates", "Live Updates (WebSocket Hub)"},
	{"terminal", "Terminal Access"},
	{"tokens", "Personal Access Tokens"},
	{"account", "Session & Account"},
	{"git-integrations", "Git Integrations"},
	{"git-providers", "Admin — Git Provider Configs"},
	{"templates", "App Templates"},
	{"deploy-actions", "Deploy & Lifecycle Actions"},
	{"metrics", "Metrics"},
	{"admin-metrics", "Admin — Metrics & Live Streams"},
	{"previews", "Preview Environments"},
	{"deployments", "Deployments"},
	{"logs", "Logs & Request Traces"},
	{"domains", "Domains & TLS"},
	{"volumes", "Application Volumes & Backups"},
	{"file-mounts", "File Mounts"},
	{"env", "Environment Variables"},
	{"databases", "Databases & Backups"},
	{"applications", "Applications"},
	{"stats-notifications", "Stats & Notifications"},
	{"projects", "Projects"},
	{"admin-users", "Admin — Users & Invitations"},
	{"admin-backups", "Admin — Platform Backups"},
	{"admin-platform", "Admin — Platform"},
	{"uncategorized", "Uncategorized"},
}

// apidocClassify assigns a route to a domain slug, evaluated as a priority
// list — first match wins. Some rules match on the route's OBSERVED scope
// (deploy, metrics) rather than its path, which is both simpler and more
// honest than pattern-matching the path shape a second time: the domain
// literally IS "the routes reachable at this scope" for those two.
func apidocClassify(r apidocRoute) string {
	switch {
	case r.Path == "/api/ws":
		return "live-updates"
	case strings.Contains(r.Path, "/terminal"):
		return "terminal"
	case strings.HasPrefix(r.Path, "/api/tokens"):
		return "tokens"
	case strings.HasPrefix(r.Path, "/api/auth/") || strings.HasPrefix(r.Path, "/api/account/"):
		return "account"
	case strings.HasPrefix(r.Path, "/api/git/integrations"):
		return "git-integrations"
	case strings.HasPrefix(r.Path, "/api/git/providers"):
		return "git-providers"
	case strings.HasPrefix(r.Path, "/api/templates"):
		return "templates"
	case r.Scope == "deploy":
		return "deploy-actions"
	case r.Scope == "metrics" && r.Admin:
		return "admin-metrics"
	case r.Scope == "metrics":
		return "metrics"
	case strings.Contains(r.Path, "/previews"):
		return "previews"
	case strings.Contains(r.Path, "/deployments") || r.Path == "/api/deployments":
		return "deployments"
	case strings.Contains(r.Path, "/logs") || strings.Contains(r.Path, "/requests"):
		return "logs"
	case strings.Contains(r.Path, "/domains"):
		return "domains"
	case strings.Contains(r.Path, "/volumes"):
		return "volumes"
	case strings.Contains(r.Path, "/file-mounts"):
		return "file-mounts"
	case strings.Contains(r.Path, "/env"):
		return "env"
	case strings.Contains(r.Path, "/databases") || strings.Contains(r.Path, "/orphaned-backups") || strings.Contains(r.Path, "/backup-destinations"):
		return "databases"
	case strings.Contains(r.Path, "/applications"):
		return "applications"
	case strings.HasPrefix(r.Path, "/api/stats") || strings.HasPrefix(r.Path, "/api/notifications"):
		return "stats-notifications"
	case strings.HasPrefix(r.Path, "/api/projects"):
		return "projects"
	case strings.HasPrefix(r.Path, "/api/users"):
		return "admin-users"
	case strings.HasPrefix(r.Path, "/api/backups"):
		return "admin-backups"
	case r.Admin:
		return "admin-platform"
	default:
		return "uncategorized"
	}
}

func apidocNotes(r apidocRoute) string {
	var notes []string
	if r.Pinned {
		notes = append(notes, "project-pinned")
	}
	if r.Admin {
		notes = append(notes, "admin role required")
	}
	switch {
	case r.NotProbed:
		notes = append(notes, "session gate not probed ("+r.NotProbedWhy+")")
	case r.Session:
		notes = append(notes, "**session required — rejects a PAT regardless of scope**")
	}
	if notes == nil {
		return "—"
	}
	return strings.Join(notes, "; ")
}

// apidocOutDir is relative to this package's directory (apps/api/internal/handler),
// the same cross-module relative-path pattern internal/tlsstatus/expiry_parity_test.go
// already uses to read apps/web — three levels up reaches apps/, same depth. It's the
// API sidebar tab's own directory, shared with the hand-written index.mdx and
// access.mdx (the task guide) — see apidocGeneratedMarker for why that's safe.
const apidocOutDir = "../../../site/content/docs/api"

// apidocGeneratedMarker appears in every file writeDomainMDX writes. The stale-file
// cleanup below only ever deletes a file that carries it, rather than checking a
// hand-maintained allowlist of hand-written names — the allowlist shape was tried
// first and rejected: it fixes the CURRENT hand-written pages (index.mdx, access.mdx)
// but leaves the class, so the next one anyone adds to this directory silently
// vanishes on the next `task generate:api-docs`, with only a t.Logf as evidence.
// Marker-based deletion makes every hand-written page safe by construction, and
// flips the failure mode from "silently deletes your work" to "leaves a stale
// file" — the direction a destructive default should fail in.
const apidocGeneratedMarker = "DO NOT EDIT — generated by TestGenerateAPIReference"

func writeAPIReferenceMDX(t *testing.T, routes []apidocRoute) {
	t.Helper()
	require.NoError(t, os.MkdirAll(apidocOutDir, 0o755))

	bySlug := make(map[string]*apidocDomain, len(apidocDomainOrder))
	for _, o := range apidocDomainOrder {
		bySlug[o.slug] = &apidocDomain{Slug: o.slug, Title: o.title}
	}
	for _, r := range routes {
		slug := apidocClassify(r)
		d, ok := bySlug[slug]
		require.True(t, ok, "apidocClassify returned %q, which is not in apidocDomainOrder", slug)
		d.Rows = append(d.Rows, r)
	}

	// "index" and "access" are hand-written (the tab's own landing page and its
	// task guide, moved in from api-access.mdx) — never generated, always listed.
	pages := []string{"index", "access"}
	writtenThisRun := map[string]bool{}
	for _, o := range apidocDomainOrder {
		d := bySlug[o.slug]
		if len(d.Rows) == 0 {
			continue // never emit an empty page or list it in meta.json
		}
		writeDomainMDX(t, *d)
		pages = append(pages, d.Slug)
		writtenThisRun[d.Slug+".mdx"] = true
	}

	// Remove stale output from a previous run whose domain no longer has any
	// routes (e.g. a route was reclassified, or a route disappeared) — an
	// orphaned .mdx file that isn't in meta.json's pages list still gets
	// picked up by Fumadocs and shown, with silently outdated content, which
	// is exactly the kind of drift this whole generator exists to prevent.
	// Only a file carrying apidocGeneratedMarker is a candidate for removal at
	// all, so index.mdx, access.mdx, and meta.json (rewritten separately below,
	// never marker-checked) are untouched regardless of what's in writtenThisRun.
	entries, err := os.ReadDir(apidocOutDir)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || writtenThisRun[e.Name()] {
			continue
		}
		b, err := os.ReadFile(filepath.Join(apidocOutDir, e.Name()))
		require.NoError(t, err)
		if !bytes.Contains(b, []byte(apidocGeneratedMarker)) {
			continue // not ours — hand-written, or otherwise not this generator's output
		}
		require.NoError(t, os.Remove(filepath.Join(apidocOutDir, e.Name())))
		t.Logf("apidoc: removed stale %s", e.Name())
	}

	meta := struct {
		Title string   `json:"title"`
		Root  bool     `json:"root"`
		Pages []string `json:"pages"`
	}{
		Title: "API",
		Root:  true,
		Pages: pages,
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(apidocOutDir, "meta.json"), append(b, '\n'), 0o644))
}

func writeDomainMDX(t *testing.T, d apidocDomain) {
	t.Helper()

	plural := "s"
	if len(d.Rows) == 1 {
		plural = ""
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "title: %s\n", d.Title)
	fmt.Fprintf(&sb, "description: %d probed endpoint%s.\n", len(d.Rows), plural)
	sb.WriteString("---\n\n")
	sb.WriteString("{/* DO NOT EDIT — generated by TestGenerateAPIReference\n")
	sb.WriteString("     (apps/api/internal/handler/apidoc_generate_test.go).\n")
	sb.WriteString("     Regenerate with `task generate:api-docs`. */}\n\n")
	sb.WriteString("| Method | Path | Handler | Scope | Notes |\n")
	sb.WriteString("|---|---|---|---|---|\n")
	for _, r := range d.Rows {
		fmt.Fprintf(&sb, "| %s | `%s` | `%s` | `%s` | %s |\n",
			r.Method, r.Path, r.Handler, r.Scope, apidocNotes(r))
	}

	path := filepath.Join(apidocOutDir, d.Slug+".mdx")
	require.NoError(t, os.WriteFile(path, []byte(sb.String()), 0o644))
}
