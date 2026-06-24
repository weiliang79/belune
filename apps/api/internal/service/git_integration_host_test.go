package service

import "testing"

func TestProviderHost(t *testing.T) {
	cases := []struct {
		provider string
		baseURL  string
		want     string
	}{
		{"github", "", "github.com"},
		{"gitlab", "", "gitlab.com"},
		{"gitlab", "https://gitlab.example.com", "gitlab.example.com"},
		{"bitbucket", "", "bitbucket.org"},
		{"gitea", "https://git.example.com", "git.example.com"},
		{"gitea", "", ""}, // gitea requires a base URL
	}
	for _, c := range cases {
		if got := providerHost(c.provider, c.baseURL); got != c.want {
			t.Errorf("providerHost(%q, %q) = %q, want %q", c.provider, c.baseURL, got, c.want)
		}
	}
}

func TestRepoHost(t *testing.T) {
	cases := map[string]string{
		"https://github.com/u/r.git":          "github.com",
		"https://GitHub.com/u/r.git":          "github.com",
		"https://gitlab.example.com:8443/u/r": "gitlab.example.com:8443",
		"not a url":                           "",
		"":                                    "",
	}
	for in, want := range cases {
		if got := repoHost(in); got != want {
			t.Errorf("repoHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// The host gate is what stops an integration token from being minted for a
// source_repo on a different host (credential exfiltration guard).
func TestHostGateMismatch(t *testing.T) {
	// github integration with a non-github source repo must not match.
	if providerHost("github", "") == repoHost("https://evil.example.com/u/r.git") {
		t.Fatal("github provider host must not equal a foreign repo host")
	}
	// matching host passes.
	if providerHost("github", "") != repoHost("https://github.com/u/r.git") {
		t.Fatal("github provider host should match a github repo host")
	}
}
