package server

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestRequireRoleCallsAgreeOnRoleSet guards an assumption
// apidoc_generate_test.go's role-gating detection depends on. The generator
// reads RequireRole's PRESENCE in a route's middleware chain via reflection
// (runtime.FuncForPC recovers the declaring function name, never the
// closure-captured arguments — the same limitation that forces RequireScope
// to be probed), and reads WHICH roles that call named by parsing this
// file's source (apidocRequireRoleSet). That parse yields a single answer
// only if every RequireRole call site names the same set: reflection can't
// say which route got which, so two different sets would make attribution a
// guess. This enforces the agreement at the source — a deliberate global
// role change flows through, a divergent one fails here.
func TestRequireRoleCallsAgreeOnRoleSet(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatal(err)
	}
	calls := regexp.MustCompile(`middleware\.RequireRole\(([^)]*)\)`).FindAllSubmatch(src, -1)
	if len(calls) == 0 {
		t.Fatal("expected at least one middleware.RequireRole(...) call in routes.go — if this legitimately dropped to zero, apidoc_generate_test.go's role detection reads a dead code path and this test's premise is moot, not passing for the right reason")
	}

	// Parse an argument list into a sorted role set — same normalization
	// apidoc_generate_test.go's apidocRequireRoleSet does, so "owner","admin"
	// and "admin","owner" are recognized as the same set.
	parse := func(argList []byte) string {
		var roles []string
		for _, a := range strings.Split(string(argList), ",") {
			if a = strings.Trim(strings.TrimSpace(a), `"`); a != "" {
				roles = append(roles, a)
			}
		}
		sort.Strings(roles)
		return strings.Join(roles, ",")
	}

	want := parse(calls[0][1])
	for _, m := range calls[1:] {
		if got := parse(m[1]); got != want {
			t.Errorf("middleware.RequireRole call sites name different role sets (%q vs %q) — apidoc_generate_test.go attributes ONE set to every RequireRole-gated route (reflection can't tell them apart), so all call sites must name the same set", want, got)
		}
	}
	if want == "" {
		t.Errorf("middleware.RequireRole(...) called with no role argument")
	}
}
