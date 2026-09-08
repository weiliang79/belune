package server

import (
	"os"
	"regexp"
	"testing"
)

// TestRequireRoleOnlyEverNamesAdmin guards an assumption the generated API
// reference (apps/api/internal/handler/apidoc_generate_test.go) makes but
// cannot verify itself: every RequireRole(...) in this file names "admin".
//
// The generator reads a route's admin-gating structurally, off chi.Walk's
// own middleware chain via reflection on the declaring function's name — see
// apidocMiddlewareContains. That works because RequireRole's PRESENCE in the
// chain is (today) the whole story: reflection can see that some
// RequireRole call sits in front of a route, but the ROLE STRING it was
// called with is a closure-captured argument, invisible to a function-name
// lookup in exactly the same way RequireScope's captured scope string is —
// the reason this generator probes for scope instead of reading it. If a
// second role ever gets gated here (RequireRole("member"), say), the
// generator would keep labelling that route "admin role required" — wrong,
// and shipped looking authoritative because the reference says it was
// generated from observed behavior.
//
// Cheaper to guard the assumption at the source than to add a third probe
// pass for a role variant that doesn't exist yet: this test fails loudly the
// day someone adds one, rather than letting the reference go quietly wrong.
func TestRequireRoleOnlyEverNamesAdmin(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}

	calls := regexp.MustCompile(`middleware\.RequireRole\(([^)]*)\)`).FindAllSubmatch(src, -1)
	if len(calls) == 0 {
		t.Fatal("found no middleware.RequireRole(...) calls in routes.go — " +
			"has it been renamed or moved? apidoc_generate_test.go's admin-role " +
			"detection assumes this call shape exists")
	}

	for _, m := range calls {
		args := string(m[1])
		if args != `"admin"` {
			t.Errorf(`middleware.RequireRole(%s): every call in routes.go must name exactly "admin" `+
				"— the generated API reference labels ANY RequireRole match as \"admin role required\" "+
				"(it can't recover a closure-captured argument by reflection, the same reason it probes "+
				"for scope instead of reading it). Update apidocMiddlewareContains and the reference's "+
				"Notes column before this can be anything else.", args)
		}
	}
}
