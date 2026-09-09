// apidoc_generate_test.go generates apps/site/public/openapi.json — an
// OpenAPI 3.1 spec — by combining two independent sources of truth about the
// real API, neither of which a hand-written reference can get right at this
// scale (212 routes, 1,829 response field slots):
//
//  1. STATIC extraction (apidocExtractTypes) reads request/response Go types
//     directly via golang.org/x/tools/go/packages — a full go/types load of
//     this package, the same information `go vet` has. It resolves every
//     writeJSON call's data argument and every json.Decoder.Decode target to
//     its static type, then walks that type into a JSON Schema.
//  2. DYNAMIC probing (the same chi.Walk + PAT mechanism destroy_boundary_test.go
//     and reveal_boundary_test.go already use) tells us what static types
//     alone cannot: which scope a route needs, and whether it's session-only.
//     A scope requirement is a runtime CLOSURE VALUE (RequireScope("write")) —
//     no amount of reading routes.go recovers "write" from that without
//     actually calling the route and reading the rejection message.
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
//     the scope, and the row is still emitted — marked with
//     x-belune-session-gate: unverified in the spec, never silently dropped
//     or silently claimed session-only.
//
// Admin-role gating is NOT probed — it's read structurally off the same
// chi.Walk call, for a reason specific to what reflection can and can't
// recover: RequireRole("admin") and RequireSession are parameterless/fixed
// (their presence in the middleware chain is the whole story, and every
// RequireRole call in this codebase names "admin", never anything else — see
// internal/server/routes_test.go's TestRequireRoleOnlyEverNamesAdmin, which
// enforces that invariant at the source), so a function-name check via
// runtime.FuncForPC is exact and free — no second HTTP round trip earns
// anything. RequireScope(required) is different: it's parameterized by a
// scope STRING captured in the closure, which a function name can't recover
// without reaching into unexported runtime internals — that's the one thing
// only a live probe can actually tell you, which is exactly why probing is
// this generator's primary technique for scope, not a stylistic choice.
//
// SECURITY ENCODING: OpenAPI's `scopes` array on a security requirement is
// legal ONLY for oauth2/openIdConnect schemes — for http/bearer it MUST be
// empty in every ratified version of the spec (3.0.x through 3.2.0; a
// "MAY contain role names" relaxation exists in secondary sources but was
// never merged into a published spec). Belune has no OAuth2 authorization
// server for PATs (scopes are chosen at mint time in the dashboard, no
// flows/authorizationUrl/tokenUrl exist), so declaring oauth2 to unlock that
// array would make codegen attempt a handshake against endpoints that don't
// exist — strictly worse than not having it. Instead the requirement is
// carried in the SECURITY SCHEME NAME, named by what a token must SATISFY
// (not what it holds), reusing the scope lattice's own total order
// (metrics ⊂ read ⊂ deploy ⊂ write — see the comment on scopeGrants in
// middleware/scope.go): patMetrics/patRead/patDeploy/patWrite/session/adminRole.
// A route needing `read` lists ONLY `patRead` — a token holding read, deploy,
// OR write all satisfy it, so naming by minimum keeps it to one scheme per
// route instead of enumerating every tier that would also work. `session` is
// OR'd into every non-session-exclusive requirement, since RequireScope lets
// a session JWT pass unconditionally (ScopesFromContext returns nil for one).
// `adminRole` is ANDed in separately when RequireRole is present, applying to
// both the PAT and session alternatives — RequireRole reads role from
// request context uniformly regardless of auth method.
package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"

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
			row.Scope = "none"
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

	sig, reg := apidocExtractTypes(t)
	doc := apidocBuildDocument(t, routes, sig, reg)
	apidocWriteSpec(t, doc)
	// Per-domain site pages are generated separately, by
	// apps/site/scripts/generate-api-pages.mjs (fumadocs-openapi's own
	// generateFiles, run against this file) — see task generate:api-docs,
	// which runs both in sequence. Writing them from Go, guessing at
	// <APIPage>'s prop shape by hand, was tried first and abandoned: the
	// library's own generator wires the exact props (payload, operations,
	// webhooks) and imports correctly, which reading .d.ts files alone did
	// not reliably predict.
}

// apidocDomain is one output page: a title plus the rows that classified
// into it, in probe order (already path-sorted by the caller). Used both to
// tag each operation (operation.Tags) and to group the per-domain stub .mdx
// pages fumadocs-openapi's <APIPage> renders from.
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

func apidocGroupByDomain(routes []apidocRoute) []apidocDomain {
	bySlug := make(map[string]*apidocDomain, len(apidocDomainOrder))
	for _, o := range apidocDomainOrder {
		bySlug[o.slug] = &apidocDomain{Slug: o.slug, Title: o.title}
	}
	for _, r := range routes {
		slug := apidocClassify(r)
		d, ok := bySlug[slug]
		if !ok {
			panic(fmt.Sprintf("apidocClassify returned %q, which is not in apidocDomainOrder", slug))
		}
		d.Rows = append(d.Rows, r)
	}
	var out []apidocDomain
	for _, o := range apidocDomainOrder {
		d := *bySlug[o.slug]
		if len(d.Rows) > 0 {
			out = append(out, d)
		}
	}
	return out
}

// === Static extraction: request/response types via golang.org/x/tools/go/packages ===
//
// apidocSchema is a JSON Schema node, kept as a plain map rather than a rigid
// struct because JSON Schema's shape genuinely varies (object/array/$ref/
// primitive) — a struct would need a field for every keyword combination.
type apidocSchema map[string]any

// apidocOperationTypes holds one handler function's request/response schemas,
// as resolved by apidocExtractTypes. Keyed by handler function name (the same
// short name apidocFuncName recovers from the running router via reflection),
// so the static (types) and dynamic (probe) halves of this generator join on
// a name both sides compute the same way.
type apidocOperationTypes struct {
	Request   apidocSchema         // nil if the handler decodes no body
	Responses map[int]apidocSchema // status code -> response body schema
}

// apidocSchemaRegistry accumulates named component schemas (components.schemas
// in the emitted spec) as apidocWalkType resolves named struct types. 100
// named structs are reachable from only 94 distinct root response types, so
// defining each once and referencing it by $ref avoids duplicating shared
// types (most visibly generated.Database and friends) at every call site.
type apidocSchemaRegistry struct {
	schemas  map[string]apidocSchema
	inFlight map[string]bool // guards self-referential types against infinite recursion
}

func newApidocSchemaRegistry() *apidocSchemaRegistry {
	return &apidocSchemaRegistry{schemas: map[string]apidocSchema{}, inFlight: map[string]bool{}}
}

// walkType resolves a Go static type into a JSON Schema node. Five traps —
// four anticipated before this was written (see the top-of-file design note
// and project_api_reference_generator_design.md), a fifth ([]byte/
// json.RawMessage) found only by actually running the generator and reading
// its output, exactly the argument this whole project makes for generation
// over hand-writing:
//
//  1. pgtype values implement json.Marshaler and marshal to a bare nullable
//     scalar — naive reflection would emit their internal struct shape
//     ({"String":"x","Valid":true}), which is wrong. Checked before any
//     other case, by exact type string, from a table verified by actually
//     marshaling each of the four pgtype types this response surface uses.
//  2. Embedded-struct promotion — handled in buildObjectSchema, not here.
//  3. map[string]any roots — handled by the AST-literal path in
//     apidocResolveResponseExpr, not here; by the time a map[string]any
//     reaches walkType (e.g. built from a variable, not an inline literal),
//     its static type carries no field information, so it becomes a
//     generic-but-honest {"type":"object","additionalProperties":{}}.
//  4. interface{} leaves are genuinely opaque — emitted as {} (unconstrained
//     JSON Schema), not guessed at.
//  5. encoding/json special-cases []byte (base64 STRING on the wire, not an
//     array of small integers — the generic slice rule would say otherwise)
//     and separately special-cases json.RawMessage (passed through as
//     embedded JSON verbatim, neither base64 nor an array). First generated
//     spec had 15 fields wrong this way (credentials_encrypted,
//     advanced_config, notification channel configs, audit log details, …) —
//     caught by reading the actual output, not by design review.
func (reg *apidocSchemaRegistry) walkType(t types.Type) apidocSchema {
	switch t.String() {
	case "github.com/jackc/pgx/v5/pgtype.UUID":
		return apidocSchema{"type": []string{"string", "null"}, "format": "uuid"}
	case "github.com/jackc/pgx/v5/pgtype.Timestamptz":
		return apidocSchema{"type": []string{"string", "null"}, "format": "date-time"}
	case "github.com/jackc/pgx/v5/pgtype.Text":
		return apidocSchema{"type": []string{"string", "null"}}
	case "github.com/jackc/pgx/v5/pgtype.Int4":
		return apidocSchema{"type": []string{"integer", "null"}}
	case "encoding/json.RawMessage":
		// Passed through verbatim by json.RawMessage's own MarshalJSON — it
		// IS already-valid JSON on the wire, not a byte string. Genuinely
		// arbitrary shape (that's the whole point of using RawMessage), same
		// honest {} as an interface{} leaf.
		return apidocSchema{"description": "arbitrary JSON — see the handler for the exact shape"}
	}
	if strings.HasPrefix(t.String(), "github.com/jackc/pgx/v5/pgtype.") {
		// A pgtype value outside the four verified above — every pgtype type
		// implements json.Marshaler with its own wire shape, so guessing at
		// its struct fields would very likely be wrong. Fail loud instead:
		// this shows up as a visible schema description, not a silent
		// mis-emission, and names exactly what needs adding to the table.
		return apidocSchema{"description": "UNHANDLED pgtype: " + t.String() + " — add to the lookup table in apidoc_generate_test.go's walkType"}
	}
	if slice, ok := t.Underlying().(*types.Slice); ok {
		if b, ok := slice.Elem().Underlying().(*types.Basic); ok && b.Kind() == types.Uint8 {
			// []byte (including any named type underlain by it) — base64
			// string per encoding/json's default encoding, not an array.
			return apidocSchema{"type": "string", "format": "byte"}
		}
	}

	switch u := t.Underlying().(type) {
	case *types.Pointer:
		return apidocNullable(reg.walkType(u.Elem()))
	case *types.Slice:
		return apidocSchema{"type": "array", "items": reg.walkType(u.Elem())}
	case *types.Array:
		return apidocSchema{"type": "array", "items": reg.walkType(u.Elem())}
	case *types.Map:
		return apidocSchema{"type": "object", "additionalProperties": reg.walkType(u.Elem())}
	case *types.Struct:
		if named, ok := t.(*types.Named); ok {
			return reg.registerNamed(named, u)
		}
		return reg.buildObjectSchema(u) // anonymous struct literal type (rare) — inline it
	case *types.Basic:
		return apidocBasicSchema(u)
	case *types.Interface:
		return apidocSchema{} // genuinely opaque — trap #4, no static shape to report
	default:
		return apidocSchema{"description": "unhandled Go type shape: " + t.String()}
	}
}

// apidocNullable wraps a schema to also permit null, the JSON Schema 2020-12
// (OpenAPI 3.1) way — a "type" array gains "null"; a $ref (which JSON Schema
// allows sibling keywords for as of 2020-12, but wrapping is the more widely
// understood idiom) becomes an anyOf against {"type":"null"}.
func apidocNullable(s apidocSchema) apidocSchema {
	if ref, ok := s["$ref"]; ok {
		return apidocSchema{"anyOf": []apidocSchema{{"$ref": ref}, {"type": "null"}}}
	}
	switch v := s["type"].(type) {
	case string:
		s["type"] = []string{v, "null"}
	case []string:
		for _, x := range v {
			if x == "null" {
				return s
			}
		}
		s["type"] = append(v, "null")
	}
	return s // no "type" key at all (e.g. {} for interface{}) already permits null
}

// registerNamed resolves a named struct type, registering it into the
// component schema registry at most once (self-referential and repeatedly-
// reached types both short-circuit via inFlight/the existing entry) and
// returning a $ref to it.
func (reg *apidocSchemaRegistry) registerNamed(named *types.Named, u *types.Struct) apidocSchema {
	name := named.Obj().Name()
	ref := apidocSchema{"$ref": "#/components/schemas/" + name}
	if reg.inFlight[name] {
		return ref
	}
	if _, ok := reg.schemas[name]; ok {
		return ref
	}
	reg.inFlight[name] = true
	schema := reg.buildObjectSchema(u)
	delete(reg.inFlight, name)
	reg.schemas[name] = schema
	return ref
}

// buildObjectSchema walks a struct's exported fields into an object schema.
// Handles trap #2: an embedded field with no json tag is PROMOTED by
// encoding/json — its own fields appear at the parent's top level on the
// wire, not nested under a field named after the embedded type. Naive
// extraction would emit a nested object that doesn't exist
// (databaseResponse embeds generated.Database with no tag; 6 call sites).
func (reg *apidocSchemaRegistry) buildObjectSchema(u *types.Struct) apidocSchema {
	props := apidocSchema{}
	for i := 0; i < u.NumFields(); i++ {
		f := u.Field(i)
		if !f.Exported() {
			continue
		}
		jsonTag, hasJSONTag := reflect.StructTag(u.Tag(i)).Lookup("json")
		name, _, _ := strings.Cut(jsonTag, ",")

		if f.Embedded() && !hasJSONTag {
			if embeddedProps, ok := reg.propertiesOf(reg.walkType(f.Type())); ok {
				for k, v := range embeddedProps {
					props[k] = v
				}
			}
			continue
		}
		if name == "-" {
			continue // explicitly excluded from the wire
		}
		if name == "" {
			name = f.Name() // no json tag at all -> encoding/json uses the Go field name verbatim
		}
		props[name] = reg.walkType(f.Type())
	}
	return apidocSchema{"type": "object", "properties": props}
}

// propertiesOf dereferences a schema back to its "properties" map, following
// a $ref into the registry if needed — used only for embedding-promotion,
// where a named embedded type's fields need to be merged into its parent
// rather than kept as its own nested $ref.
func (reg *apidocSchemaRegistry) propertiesOf(s apidocSchema) (apidocSchema, bool) {
	if ref, ok := s["$ref"].(string); ok {
		target, ok := reg.schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
		if !ok {
			return nil, false // self-referential embedding — pathological, shouldn't occur
		}
		s = target
	}
	props, ok := s["properties"].(apidocSchema)
	return props, ok
}

func apidocBasicSchema(b *types.Basic) apidocSchema {
	switch {
	case b.Info()&types.IsInteger != 0:
		return apidocSchema{"type": "integer"}
	case b.Info()&types.IsFloat != 0:
		return apidocSchema{"type": "number"}
	case b.Info()&types.IsBoolean != 0:
		return apidocSchema{"type": "boolean"}
	case b.Info()&types.IsString != 0:
		return apidocSchema{"type": "string"}
	default:
		return apidocSchema{}
	}
}

// apidocIsHandlerReceiver reports whether a function declaration's receiver
// is `*Handler` — the type every route's handler method is declared on.
func apidocIsHandlerReceiver(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	id, ok := star.X.(*ast.Ident)
	return ok && id.Name == "Handler"
}

// apidocConstInt resolves a constant integer expression (an http.StatusOK
// identifier, a literal 429, etc.) via the type-checker's own constant
// folding — works for any of net/http's Status* constants without a
// hand-typed name table, since go/types already resolves what they equal.
func apidocConstInt(info *types.Info, expr ast.Expr) (int, bool) {
	tv, ok := info.Types[expr]
	if !ok || tv.Value == nil {
		return 0, false
	}
	i, ok := constant.Int64Val(tv.Value)
	if !ok {
		return 0, false
	}
	return int(i), true
}

// apidocResolveResponseExpr resolves one writeJSON call's data argument to a
// schema. Trap #3: when the argument is an inline map[string]any{...}
// literal, its STATIC TYPE carries zero field information (every such root
// resolves to the identical "map[string]any") — the actual shape only exists
// in the AST as literal keys and per-value expressions, so this is a second
// code path, not a variation of walkType's struct walk. A map[string]any
// reached through a variable instead (built up across several statements
// before the writeJSON call) falls through to the static-type path and gets
// an honest, generic {"type":"object","additionalProperties":{}} rather than
// a guessed-at shape — a known, deliberate precision limit for that specific
// sub-case, not a silent gap: nothing here claims a wrong shape.
func apidocResolveResponseExpr(reg *apidocSchemaRegistry, info *types.Info, expr ast.Expr) apidocSchema {
	if lit, ok := expr.(*ast.CompositeLit); ok {
		if _, isMapType := lit.Type.(*ast.MapType); isMapType {
			if schema, ok := apidocWalkMapLiteral(reg, info, lit); ok {
				return schema
			}
		}
	}
	t := info.TypeOf(expr)
	if t == nil {
		return apidocSchema{"description": "unresolved response type"}
	}
	return reg.walkType(t)
}

func apidocWalkMapLiteral(reg *apidocSchemaRegistry, info *types.Info, lit *ast.CompositeLit) (apidocSchema, bool) {
	props := apidocSchema{}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil, false
		}
		keyLit, ok := kv.Key.(*ast.BasicLit)
		if !ok {
			return nil, false
		}
		key, err := strconv.Unquote(keyLit.Value)
		if err != nil {
			return nil, false
		}
		valType := info.TypeOf(kv.Value)
		if valType == nil {
			return nil, false
		}
		props[key] = reg.walkType(valType)
	}
	return apidocSchema{"type": "object", "properties": props}, true
}

// apidocExtractTypes loads ./internal/handler (this package) via go/packages
// — a full go/types load, the same information `go vet` has — and resolves
// every *Handler method's writeJSON calls and json.Decoder.Decode target to
// a static type. "." as both Dir and pattern: `go test` always runs with the
// working directory set to the package under test, so "." IS
// apps/api/internal/handler already; no absolute path needed.
func apidocExtractTypes(t *testing.T) (map[string]apidocOperationTypes, *apidocSchemaRegistry) {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
			packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports,
		Dir: ".",
	}
	pkgs, err := packages.Load(cfg, ".")
	require.NoError(t, err)
	require.Len(t, pkgs, 1, "expected exactly one package for \".\"")
	pkg := pkgs[0]
	require.Empty(t, pkg.Errors, "static extraction requires a clean compile: %v", pkg.Errors)

	// First pass: every *Handler method name, so a call like h.listContainerLogs(...)
	// can be recognized as a delegate to ANOTHER handler method (as opposed to a
	// call to h.queries.X or an unrelated helper) — found necessary only by
	// running the generator: several route handlers (e.g. ListApplicationLogs)
	// have no writeJSON/Decode call of their own at all, delegating entirely to
	// a shared, non-route *Handler method that does.
	handlerMethods := map[string]bool{}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && len(fn.Recv.List) == 1 && apidocIsHandlerReceiver(fn.Recv.List[0].Type) {
				handlerMethods[fn.Name.Name] = true
			}
		}
	}

	reg := newApidocSchemaRegistry()
	result := map[string]apidocOperationTypes{}
	delegates := map[string][]string{}

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			if !apidocIsHandlerReceiver(fn.Recv.List[0].Type) {
				continue
			}

			ops := apidocOperationTypes{Responses: map[int]apidocSchema{}}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "writeJSON" && len(call.Args) == 3 {
					if status, ok := apidocConstInt(pkg.TypesInfo, call.Args[1]); ok {
						ops.Responses[status] = apidocResolveResponseExpr(reg, pkg.TypesInfo, call.Args[2])
					}
					// A dynamic (non-constant) status is left undocumented for
					// that call site rather than guessed — 2 such call sites
					// per the design doc's static-extraction count.
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if sel.Sel.Name == "Decode" && len(call.Args) == 1 {
						if unary, ok := call.Args[0].(*ast.UnaryExpr); ok {
							if tv := pkg.TypesInfo.TypeOf(unary.X); tv != nil {
								ops.Request = reg.walkType(tv)
							}
						}
						return true
					}
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "h" && handlerMethods[sel.Sel.Name] && sel.Sel.Name != fn.Name.Name {
						delegates[fn.Name.Name] = append(delegates[fn.Name.Name], sel.Sel.Name)
					}
				}
				return true
			})

			result[fn.Name.Name] = ops
		}
	}

	// Fixed-point merge: a handler with nothing of its own inherits its
	// delegate(s)' resolved responses/request — bounded iteration since a
	// delegate can itself delegate one level further (ListApplicationLogs ->
	// listContainerLogs, observed depth 1, but not assumed to be the only
	// depth that will ever exist).
	for range 5 {
		changed := false
		for name, ops := range result {
			if len(ops.Responses) > 0 && ops.Request != nil {
				continue
			}
			for _, dep := range delegates[name] {
				depOps, ok := result[dep]
				if !ok {
					continue
				}
				for status, schema := range depOps.Responses {
					if _, exists := ops.Responses[status]; !exists {
						ops.Responses[status] = schema
						changed = true
					}
				}
				if ops.Request == nil && depOps.Request != nil {
					ops.Request = depOps.Request
					changed = true
				}
			}
			result[name] = ops
		}
		if !changed {
			break
		}
	}

	for name, ops := range result {
		if len(ops.Responses) == 0 && ops.Request == nil {
			delete(result, name) // no signal at all, direct or delegated — apidocBuildDocument's zero-value lookup on a missing key is equivalent, dropping it just keeps the map small
		}
	}
	return result, reg
}

// === OpenAPI 3.1 document assembly ===

type oasDocument struct {
	OpenAPI    string                 `json:"openapi"`
	Info       oasInfo                `json:"info"`
	Servers    []oasServer            `json:"servers"`
	Tags       []oasTag               `json:"tags"`
	Paths      map[string]oasPathItem `json:"paths"`
	Components oasComponents          `json:"components"`
}
type oasTag struct {
	Name string `json:"name"`
}
type oasInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
}
type oasServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}
type oasPathItem map[string]oasOperation
type oasParameter struct {
	Name     string       `json:"name"`
	In       string       `json:"in"`
	Required bool         `json:"required"`
	Schema   apidocSchema `json:"schema"`
}
type oasOperation struct {
	OperationID        string                 `json:"operationId"`
	Description        string                 `json:"description,omitempty"`
	Tags               []string               `json:"tags,omitempty"`
	Parameters         []oasParameter         `json:"parameters,omitempty"`
	RequestBody        *oasRequestBody        `json:"requestBody,omitempty"`
	Responses          map[string]oasResponse `json:"responses"`
	Security           []map[string][]string  `json:"security"`
	XBeluneSessionGate string                 `json:"x-belune-session-gate,omitempty"`
}
type oasRequestBody struct {
	Required bool                    `json:"required"`
	Content  map[string]oasMediaType `json:"content"`
}
type oasResponse struct {
	Description string                  `json:"description"`
	Content     map[string]oasMediaType `json:"content,omitempty"`
}
type oasMediaType struct {
	Schema apidocSchema `json:"schema"`
}
type oasComponents struct {
	Schemas         map[string]apidocSchema      `json:"schemas"`
	SecuritySchemes map[string]oasSecurityScheme `json:"securitySchemes"`
}
type oasSecurityScheme struct {
	Type        string `json:"type"`
	Scheme      string `json:"scheme"`
	Description string `json:"description"`
}

// apidocScopeScheme maps a probed scope requirement to the security scheme
// that SATISFIES it (not the scheme matching it exactly) — Belune's scopes
// are a total order (metrics ⊂ read ⊂ deploy ⊂ write; scopeGrants in
// middleware/scope.go), so a route needing `read` is satisfied by a token
// holding read, deploy, OR write. Naming the scheme by the minimum keeps
// each route to ONE scheme reference instead of enumerating every tier that
// would also work.
func apidocScopeScheme(scope string) string {
	switch scope {
	case "metrics":
		return "patMetrics"
	case "read":
		return "patRead"
	case "deploy":
		return "patDeploy"
	case "write":
		return "patWrite"
	default:
		return ""
	}
}

// apidocSecurity builds one operation's security requirement. OpenAPI's
// per-requirement `scopes` array is legal only for oauth2/openIdConnect
// schemes (MUST be empty otherwise, in every ratified spec version) — Belune
// has no OAuth2 authorization server for PATs, so the requirement is carried
// in the SCHEME NAME instead (see apidocScopeScheme), with `session` OR'd
// into every non-session-exclusive requirement (RequireScope lets a session
// JWT pass unconditionally) and `adminRole` ANDed in when RequireRole gates
// the route (applies uniformly to both PAT and session auth).
func apidocSecurity(r apidocRoute) []map[string][]string {
	var reqs []map[string][]string
	switch {
	case r.Scope == "none" && r.Session:
		// No RequireScope* at all — only RequireSession gates it (e.g. the WS
		// terminal tunnel). No PAT alternative exists at any scope.
		reqs = []map[string][]string{{"session": {}}}
	case r.Session:
		// A scope requirement exists too (probe 1 still resolved one), but
		// RequireSession rejects every PAT regardless — the scope is real but
		// moot for authorization purposes, so only session is listed.
		reqs = []map[string][]string{{"session": {}}}
	default:
		scheme := apidocScopeScheme(r.Scope)
		if scheme == "" {
			return nil // UNRECOGNIZED — TestGenerateAPIReference already fails loudly on this
		}
		reqs = []map[string][]string{{scheme: {}}, {"session": {}}}
	}
	if !r.Admin {
		return reqs
	}
	out := make([]map[string][]string, len(reqs))
	for i, req := range reqs {
		merged := map[string][]string{"adminRole": {}}
		for k, v := range req {
			merged[k] = v
		}
		out[i] = merged
	}
	return out
}

func apidocPathParameters(path string) []oasParameter {
	var params []oasParameter
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			params = append(params, oasParameter{
				Name: seg[1 : len(seg)-1], In: "path", Required: true,
				Schema: apidocSchema{"type": "string", "format": "uuid"},
			})
		}
	}
	return params
}

// apidocOperationDescription surfaces the caveats a schema alone can't carry
// — project-pinning has no OpenAPI concept at all, and an unverified session
// gate needs prose alongside its x-belune-session-gate marker so a reader
// (not just a machine) sees the caveat.
func apidocOperationDescription(r apidocRoute) string {
	var notes []string
	if r.Pinned {
		notes = append(notes, "Scoped to one project — a token pinned to a different project is rejected outside it, regardless of scope.")
	}
	if r.NotProbed {
		notes = append(notes, fmt.Sprintf(
			"Session-gate UNVERIFIED (%s). Scope is confirmed — every route is probed at zero scope — but whether a personal access token is additionally rejected here by a session-only gate is not; both security alternatives below are listed, only one is guaranteed correct.",
			r.NotProbedWhy))
	}
	return strings.Join(notes, " ")
}

const apidocSpecDescription = `Generated from the running API by probing every registered route with a personal access token and statically resolving every request/response Go type — not hand-written, and not a reading of routes.go. See the API Access guide for how to authenticate and the scope model.

**"required" is intentionally omitted from every request schema.** Field optionality in this codebase is enforced imperatively (e.g. ` + "`if req.Name == \"\" { ... }`" + `), not type-encoded — most optional fields aren't even pointers. Inferring "required" from arbitrary validation code is a fundamentally fuzzier problem than the static type resolution the rest of this spec relies on, so it is left honestly unspecified rather than risked wrong.

**Session-gate unverified on some routes** (marked ` + "`x-belune-session-gate: \"unverified\"`" + ` on the operation): a small number of destructive admin actions and streaming (SSE) routes aren't safe to actually execute during generation, so whether a personal access token is additionally rejected by a session-only gate is unconfirmed for those. Both the scope-based and session-only security alternatives are listed on those operations; only one is guaranteed correct.

**Project-pinning has no OpenAPI representation** — a token pinned to one project rejected outside it is noted in the affected operations' descriptions, not encoded structurally.`

func apidocSecuritySchemes() map[string]oasSecurityScheme {
	return map[string]oasSecurityScheme{
		"patMetrics": {Type: "http", Scheme: "bearer", Description: "Personal access token whose scope satisfies `metrics` — metrics, read, deploy, or write all qualify (Belune's scopes form a total order)."},
		"patRead":    {Type: "http", Scheme: "bearer", Description: "Personal access token whose scope satisfies `read` — read, deploy, or write all qualify."},
		"patDeploy":  {Type: "http", Scheme: "bearer", Description: "Personal access token whose scope satisfies `deploy` — deploy or write qualify."},
		"patWrite":   {Type: "http", Scheme: "bearer", Description: "Personal access token whose scope satisfies `write`."},
		"session":    {Type: "http", Scheme: "bearer", Description: "Dashboard session JWT — satisfies every scope requirement unconditionally."},
		"adminRole":  {Type: "http", Scheme: "bearer", Description: "The authenticated user, by token or session, must have the Admin role."},
	}
}

// apidocBuildDocument assembles the full OpenAPI document from the probed
// routes and the statically-extracted request/response types, joining the
// two on handler function name.
func apidocBuildDocument(t *testing.T, routes []apidocRoute, sig map[string]apidocOperationTypes, reg *apidocSchemaRegistry) oasDocument {
	t.Helper()

	domainTitleByRoute := map[apidocRoute]string{}
	for _, d := range apidocGroupByDomain(routes) {
		for _, r := range d.Rows {
			domainTitleByRoute[r] = d.Title
		}
	}

	reg.schemas["Error"] = apidocSchema{
		"type":       "object",
		"properties": apidocSchema{"error": apidocSchema{"type": "string"}},
	}
	errorRef := apidocSchema{"$ref": "#/components/schemas/Error"}

	paths := map[string]oasPathItem{}
	seenOperationIDs := map[string]bool{}
	for _, r := range routes {
		item, ok := paths[r.Path]
		if !ok {
			item = oasPathItem{}
		}

		opID := r.Handler
		if seenOperationIDs[opID] {
			opID = r.Handler + "_" + strings.ToLower(r.Method)
		}
		seenOperationIDs[opID] = true

		op := oasOperation{
			OperationID: opID,
			Tags:        []string{domainTitleByRoute[r]},
			Description: apidocOperationDescription(r),
			Parameters:  apidocPathParameters(r.Path),
			Security:    apidocSecurity(r),
			Responses:   map[string]oasResponse{},
		}
		if r.NotProbed {
			op.XBeluneSessionGate = "unverified"
		}

		ops := sig[r.Handler]
		if ops.Request != nil {
			op.RequestBody = &oasRequestBody{
				Required: true, // a Decode call exists — an empty/missing body fails it; this is a mechanical fact, unlike per-field "required"
				Content:  map[string]oasMediaType{"application/json": {Schema: ops.Request}},
			}
		}
		for status, schema := range ops.Responses {
			op.Responses[strconv.Itoa(status)] = oasResponse{
				Description: http.StatusText(status),
				Content:     map[string]oasMediaType{"application/json": {Schema: schema}},
			}
		}
		if len(op.Responses) == 0 {
			// No writeJSON call resolved for this handler (a 204, a redirect,
			// a dynamic status code, or a genuine extraction gap) — an
			// honestly-empty response entry beats inventing a 200 shape that
			// might not exist.
			op.Responses["default"] = oasResponse{Description: "Not resolved by static extraction — see the handler for the exact response."}
		}
		op.Responses["403"] = oasResponse{Description: "Forbidden", Content: map[string]oasMediaType{"application/json": {Schema: errorRef}}}

		item[strings.ToLower(r.Method)] = op
		paths[r.Path] = item
	}

	// Top-level tags, in apidocDomainOrder's curated order — the SOURCE fumadocs-openapi's
	// own per-tag page generation reads for page order too, so the site's sidebar
	// and this spec's own tag list stay in the one order this generator defines,
	// rather than falling back to whatever order operations happen to appear in.
	// Only domains that actually classified a route this run — "uncategorized"
	// is a permanent safety valve in apidocGroupByDomain (a route can never
	// silently drop for lack of a matching rule), not a domain meant to
	// always exist on the wire: an empty tag here becomes an empty, unlisted
	// .mdx page from apps/site/scripts/generate-api-pages.mjs, reachable by
	// direct URL despite meta.json never listing it.
	var tags []oasTag
	for _, d := range apidocGroupByDomain(routes) {
		tags = append(tags, oasTag{Name: d.Title})
	}

	return oasDocument{
		OpenAPI: "3.1.0",
		Info: oasInfo{
			Title:       "Belune API",
			Version:     "1.0.0", // the spec's own version, independent of Belune's release tag — bump by hand on a shape change; NOT a generation timestamp, which would fail the CI freshness check every day even with zero real drift
			Description: apidocSpecDescription,
		},
		Servers: []oasServer{{URL: "https://belune.example.com", Description: "Replace with your own Belune instance's origin."}},
		Tags:    tags,
		Paths:   paths,
		Components: oasComponents{
			Schemas:         reg.schemas,
			SecuritySchemes: apidocSecuritySchemes(),
		},
	}
}

// apidocSpecPath is relative to this package's directory, the same
// cross-module pattern apidocMDXOutDir (below) and
// internal/tlsstatus/expiry_parity_test.go already use. Public, not under
// content/docs, so it's a stable URL third parties can consume directly —
// the same shape apps/site/public/versions.json already is.
const apidocSpecPath = "../../../site/public/openapi.json"

func apidocWriteSpec(t *testing.T, doc oasDocument) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(apidocSpecPath), 0o755))
	b, err := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apidocSpecPath, append(b, '\n'), 0o644))
}
