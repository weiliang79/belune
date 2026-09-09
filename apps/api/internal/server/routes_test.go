package server

import (
	"os"
	"regexp"
	"testing"
)

// TestRequireRoleOnlyEverNamesAdmin guards an assumption
// apidoc_generate_test.go's admin-role detection depends on: it reads
// RequireRole's mere PRESENCE in a route's middleware chain via reflection
// (runtime.FuncForPC), which recovers the declaring function's name but not
// the closure-captured role STRING it was called with — exactly the same
// limitation that makes RequireScope's argument unrecoverable by reflection
// and forces the generator to probe it instead. RequireRole is cheaper to
// read structurally only because every call site in this codebase happens to
// name "admin" — a fact this test enforces at the source, not a guarantee
// the type system gives for free.
func TestRequireRoleOnlyEverNamesAdmin(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatal(err)
	}
	calls := regexp.MustCompile(`middleware\.RequireRole\(([^)]*)\)`).FindAllSubmatch(src, -1)
	if len(calls) == 0 {
		t.Fatal("expected at least one middleware.RequireRole(...) call in routes.go — if this legitimately dropped to zero, apidoc_generate_test.go's Admin detection reads a dead code path and this test's premise is moot, not passing for the right reason")
	}
	for _, m := range calls {
		args := string(m[1])
		if args != `"admin"` {
			t.Errorf("middleware.RequireRole(%s) — every call in this codebase is assumed to name \"admin\" (apidoc_generate_test.go's Admin field, read via reflection, can't distinguish a role string it wasn't built to expect)", args)
		}
	}
}
