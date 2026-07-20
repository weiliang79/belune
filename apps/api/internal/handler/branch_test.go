package handler

import "testing"

func TestValidBranchName(t *testing.T) {
	valid := []string{
		"",              // the repository's default ref
		"main",          //
		"master",        // the case the branch field exists to fix
		"release/2.1",   // slashes are ordinary in refs
		"feature/ABC-1", //
		"v1.0.0",        // a tag is a legal --branch value too
		"fix.bug",       // a single dot is fine; ".." is not
	}
	for _, b := range valid {
		if !validBranchName(b) {
			t.Errorf("validBranchName(%q) = false, want true", b)
		}
	}

	invalid := map[string]string{
		"--upload-pack=x": "leading dash could be read as a flag",
		"-b":              "leading dash",
		"has space":       "whitespace",
		"tab\there":       "control character",
		"new\nline":       "control character",
		"a..b":            "double dot is invalid in a ref",
		"branch~1":        "tilde is invalid in a ref",
		"branch^":         "caret is invalid in a ref",
		"br:anch":         "colon is invalid in a ref",
		"br?anch":         "question mark is invalid in a ref",
		"br*anch":         "glob is invalid in a ref",
		"br[anch":         "bracket is invalid in a ref",
		"br\\anch":        "backslash is invalid in a ref",
	}
	for b, why := range invalid {
		if validBranchName(b) {
			t.Errorf("validBranchName(%q) = true, want false (%s)", b, why)
		}
	}

	long := make([]byte, 256)
	for i := range long {
		long[i] = 'a'
	}
	if validBranchName(string(long)) {
		t.Error("validBranchName should reject a 256-char branch name")
	}
}
