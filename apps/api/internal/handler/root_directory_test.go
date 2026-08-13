package handler

import "testing"

func TestValidRootDirectory(t *testing.T) {
	valid := []string{
		"",              // the repository root, today's only behavior
		"apps/web",      // ordinary subdirectory
		"examples/vite", //
		"a",             // single segment
		"a.b/c",         // a single dot inside a segment is fine
	}
	for _, d := range valid {
		if !validRootDirectory(d) {
			t.Errorf("validRootDirectory(%q) = false, want true", d)
		}
	}

	invalid := map[string]string{
		"/apps/web":    "leading slash — value is relative to the clone root",
		"../secrets":   "traversal segment",
		"apps/../web":  "traversal segment mid-path",
		"apps/..":      "trailing traversal segment",
		"apps//web":    "empty segment from a double slash",
		"apps/":        "trailing slash leaves an empty segment",
		".":            "single dot segment",
		"tab\there":    "control character",
		"new\nline":    "control character",
		"null\x00byte": "null byte",
	}
	for d, why := range invalid {
		if validRootDirectory(d) {
			t.Errorf("validRootDirectory(%q) = true, want false (%s)", d, why)
		}
	}

	long := make([]byte, 501)
	for i := range long {
		long[i] = 'a'
	}
	if validRootDirectory(string(long)) {
		t.Error("validRootDirectory should reject a 501-char value")
	}
}
