package providers

import (
	"net/http"
	"strings"
	"testing"
)

// GitHub names the tip commit outright in head_commit.
func TestParseWebhook_GitHubCommitDetails(t *testing.T) {
	secret := "whsec"
	body := []byte(`{
		"ref":"refs/heads/main",
		"after":"abc123",
		"head_commit":{"id":"abc123","message":"Fix login redirect","author":{"name":"Alice","username":"alice","email":"alice@example.com"}},
		"commits":[
			{"id":"old999","message":"Earlier commit","author":{"name":"Bob"}},
			{"id":"abc123","message":"Fix login redirect","author":{"name":"Alice"}}
		],
		"repository":{"clone_url":"https://github.com/u/r.git"}
	}`)
	h := http.Header{}
	h.Set("X-Hub-Signature-256", "sha256="+hmacSig(body, secret))

	ev, err := VerifyAndParseWebhook("github", h, body, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.CommitMessage != "Fix login redirect" {
		t.Fatalf("message = %q", ev.CommitMessage)
	}
	if ev.CommitAuthor != "Alice" {
		t.Fatalf("author = %q", ev.CommitAuthor)
	}
}

// Gitea sends the same shape but without head_commit, so the tip has to be
// found by matching `after` — NOT by taking commits[0], which is the oldest.
func TestParseWebhook_GiteaPicksTipNotFirstCommit(t *testing.T) {
	secret := "whsec"
	body := []byte(`{
		"ref":"refs/heads/main",
		"after":"tip222",
		"commits":[
			{"id":"old111","message":"Oldest in push","author":{"name":"Bob"}},
			{"id":"tip222","message":"Newest in push","author":{"name":"Carol","username":"carol"}}
		],
		"repository":{"clone_url":"https://gitea.example.com/u/r.git"}
	}`)
	h := http.Header{}
	h.Set("X-Gitea-Signature", hmacSig(body, secret))

	ev, err := VerifyAndParseWebhook("gitea", h, body, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.CommitMessage != "Newest in push" {
		t.Fatalf("message = %q, want the commit matching `after`", ev.CommitMessage)
	}
	if ev.CommitAuthor != "Carol" {
		t.Fatalf("author = %q", ev.CommitAuthor)
	}
}

func TestParseWebhook_GitLabCommitDetails(t *testing.T) {
	secret := "tok"
	body := []byte(`{
		"ref":"refs/heads/dev",
		"after":"deadbeef",
		"commits":[{"id":"deadbeef","message":"Bump deps","author":{"name":"Dana","email":"dana@example.com"}}],
		"project":{"git_http_url":"https://gitlab.com/u/r.git"}
	}`)
	h := http.Header{}
	h.Set("X-Gitlab-Token", secret)

	ev, err := VerifyAndParseWebhook("gitlab", h, body, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.CommitMessage != "Bump deps" || ev.CommitAuthor != "Dana" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

// A push that carries no commit array at all (branch creation, tag push) must
// still parse — the deploy just has no provenance to show.
func TestParseWebhook_NoCommitsIsNotAnError(t *testing.T) {
	secret := "whsec"
	body := []byte(`{"ref":"refs/heads/main","after":"abc123","repository":{"clone_url":"https://github.com/u/r.git"}}`)
	h := http.Header{}
	h.Set("X-Hub-Signature-256", "sha256="+hmacSig(body, secret))

	ev, err := VerifyAndParseWebhook("github", h, body, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.CommitSHA != "abc123" {
		t.Fatalf("sha = %q", ev.CommitSHA)
	}
	if ev.CommitMessage != "" || ev.CommitAuthor != "" {
		t.Fatalf("expected empty provenance, got %+v", ev)
	}
}

func TestCommitDetails_AuthorFallback(t *testing.T) {
	c := &commitObject{Message: "m"}
	c.Author.Username = "u"
	if _, author := commitDetails(c); author != "u" {
		t.Fatalf("expected username fallback, got %q", author)
	}

	c2 := &commitObject{Message: "m"}
	c2.Author.Email = "e@example.com"
	if _, author := commitDetails(c2); author != "e@example.com" {
		t.Fatalf("expected email fallback, got %q", author)
	}
}

func TestTruncateMessage(t *testing.T) {
	if got := truncateMessage("  hello  "); got != "hello" {
		t.Fatalf("expected trim, got %q", got)
	}

	long := strings.Repeat("a", commitMessageLimit+50)
	got := truncateMessage(long)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix")
	}
	if len([]rune(got)) > commitMessageLimit+1 {
		t.Fatalf("truncated message too long: %d runes", len([]rune(got)))
	}

	// A multi-byte rune straddling the cut must not be split into mojibake.
	multi := strings.Repeat("é", commitMessageLimit)
	if !isValidUTF8(truncateMessage(multi)) {
		t.Fatal("truncation split a multi-byte rune")
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}
