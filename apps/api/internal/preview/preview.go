// Package preview implements branch-driven preview environments.
//
// A "parent" application opts in by setting preview_branch_pattern and
// preview_domain_template. When a webhook push arrives whose branch matches
// the parent's pattern (but does not match the parent's auto_deploy_branch),
// a child preview application is found-or-created inheriting the parent's
// build config, and a domain is derived from the template. Children reuse
// every existing deploy/container/route/log path — the only schema addition
// is the parent_application_id + branch linkage.
package preview

import (
	"path"
	"regexp"
	"strings"
)

// MatchesPattern reports whether branch matches the glob-style pattern.
//
// Supported pattern features:
//   - "*" matches any run of characters except "/"
//   - "**" matches any run of characters including "/"
//   - exact names match themselves (e.g. "develop")
//   - empty or "*" pattern matches nothing (safety: don't treat empty as
//     match-all, since that would spray previews for every push)
//
// Matching is case-sensitive; git branch names are case-sensitive on most
// remotes even though many filesystems are not.
func MatchesPattern(pattern, branch string) bool {
	pattern = strings.TrimSpace(pattern)
	branch = strings.TrimSpace(branch)
	if pattern == "" || branch == "" {
		return false
	}
	// Bare "*" is rejected to prevent accidental match-all. Require at least
	// one literal anchor.
	if pattern == "*" {
		return false
	}
	// Fast path: no wildcards — literal equality.
	if !strings.ContainsAny(pattern, "*?[") {
		return pattern == branch
	}
	// Use path.Match semantics with "/" untreated specially — branch names
	// like "release/2026-04" should be matchable with "release/*".
	ok, err := path.Match(pattern, branch)
	if err == nil && ok {
		return true
	}
	// Support "**" as "match any path segments" by rewriting to a regex.
	if strings.Contains(pattern, "**") {
		return globToRegex(pattern).MatchString(branch)
	}
	return false
}

// globToRegex converts a glob with "**" support into an anchored regex.
// "*" → [^/]*, "**" → .*, literal special regex chars are escaped.
func globToRegex(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	i := 0
	for i < len(pattern) {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i += 2
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
		i++
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		// Fall back to a never-match regex so a malformed pattern cannot
		// accidentally spray previews.
		return regexp.MustCompile("$^")
	}
	return re
}

// SlugifyBranch converts a branch ref into a subdomain-safe slug.
// Git allows "/", ".", "_" and mixed case; DNS labels don't, so we reduce to
// [a-z0-9-], collapse separators, and truncate to DNS-safe length (63).
func SlugifyBranch(branch string) string {
	lower := strings.ToLower(branch)
	var b strings.Builder
	lastDash := false
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 63 {
		s = strings.TrimRight(s[:63], "-")
	}
	return s
}

// RenderDomainTemplate expands placeholders in a domain template.
//
// Supported placeholders:
//   - {branch} — branch name slugified for DNS (see SlugifyBranch)
//   - {app}    — parent application's base slug (DNS-safe already)
//
// Example:  template "{branch}.{app}.preview.example.com" with branch
// "feature/foo" and app slug "myapp" yields "feature-foo.myapp.preview.example.com".
//
// Returns "" when required placeholders cannot be filled (e.g. an empty
// slugified branch) — callers should treat empty as "skip preview".
func RenderDomainTemplate(template, branch, appSlug string) string {
	branchSlug := SlugifyBranch(branch)
	if branchSlug == "" {
		return ""
	}
	s := template
	s = strings.ReplaceAll(s, "{branch}", branchSlug)
	s = strings.ReplaceAll(s, "{app}", appSlug)
	s = strings.ToLower(strings.TrimSpace(s))
	return s
}
