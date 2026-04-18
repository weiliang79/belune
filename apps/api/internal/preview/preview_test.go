package preview

import "testing"

func TestMatchesPattern(t *testing.T) {
	cases := []struct {
		pattern string
		branch  string
		want    bool
	}{
		{"", "anything", false},
		{"*", "anything", false}, // bare "*" rejected as safety guard
		{"feature/*", "feature/login", true},
		{"feature/*", "feature/login/nested", false},
		{"feature/**", "feature/login/nested", true},
		{"release/*", "release/2026-04", true},
		{"release/*", "main", false},
		{"develop", "develop", true},
		{"develop", "feature/x", false},
		{"pr-*", "pr-123", true},
		{"pr-*", "PR-123", false},
	}
	for _, c := range cases {
		if got := MatchesPattern(c.pattern, c.branch); got != c.want {
			t.Errorf("MatchesPattern(%q, %q) = %v, want %v", c.pattern, c.branch, got, c.want)
		}
	}
}

func TestSlugifyBranch(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"main", "main"},
		{"Feature/Login", "feature-login"},
		{"release/2026-04", "release-2026-04"},
		{"....", ""},
		{"----x----", "x"},
	}
	for _, c := range cases {
		if got := SlugifyBranch(c.in); got != c.want {
			t.Errorf("SlugifyBranch(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRenderDomainTemplate(t *testing.T) {
	got := RenderDomainTemplate("{branch}.{app}.preview.example.com", "feature/foo", "myapp")
	want := "feature-foo.myapp.preview.example.com"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if RenderDomainTemplate("{branch}.example.com", "...", "x") != "" {
		t.Error("empty slugified branch should return empty domain")
	}
}
