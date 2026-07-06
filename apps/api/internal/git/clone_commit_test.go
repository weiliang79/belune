package git

import "testing"

func TestIsHexSHA(t *testing.T) {
	valid := []string{
		"abcdef1", // 7-char short sha
		"abcdef1234567890abcdef1234567890abcdef12",                         // 40-char sha1
		"ABCDEF1234567890ABCDEF1234567890ABCDEF12",                         // uppercase
		"1234567890123456789012345678901234567890123456789012345678901234", // 64-char sha256
	}
	for _, s := range valid {
		if !isHexSHA(s) {
			t.Errorf("isHexSHA(%q) = false, want true", s)
		}
	}

	invalid := []string{
		"",
		"abc",               // too short
		"main",              // branch name, non-hex
		"abcdefg",           // 'g' not hex
		"../etc/passwd",     // path traversal attempt
		"abcdef1; rm -rf /", // injection attempt
		"12345678901234567890123456789012345678901234567890123456789012345", // 65 chars, too long
	}
	for _, s := range invalid {
		if isHexSHA(s) {
			t.Errorf("isHexSHA(%q) = true, want false", s)
		}
	}
}
