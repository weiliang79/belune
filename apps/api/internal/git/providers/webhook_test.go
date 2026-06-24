package providers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
)

func hmacSig(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyAndParseWebhook_GitHub(t *testing.T) {
	secret := "whsec"
	body := []byte(`{"ref":"refs/heads/main","after":"abc123","repository":{"clone_url":"https://github.com/u/r.git"}}`)
	h := http.Header{}
	h.Set("X-Hub-Signature-256", "sha256="+hmacSig(body, secret))

	ev, err := VerifyAndParseWebhook("github", h, body, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.RepoURL != "https://github.com/u/r.git" || ev.Branch != "main" || ev.CommitSHA != "abc123" {
		t.Fatalf("unexpected event: %+v", ev)
	}

	// Tampered secret must fail.
	if _, err := VerifyAndParseWebhook("github", h, body, "wrong"); err == nil {
		t.Fatal("expected signature mismatch error")
	}
}

func TestVerifyAndParseWebhook_GitLab(t *testing.T) {
	secret := "tok"
	body := []byte(`{"ref":"refs/heads/dev","after":"deadbeef","project":{"git_http_url":"https://gitlab.com/u/r.git"}}`)
	h := http.Header{}
	h.Set("X-Gitlab-Token", secret)

	ev, err := VerifyAndParseWebhook("gitlab", h, body, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.RepoURL != "https://gitlab.com/u/r.git" || ev.Branch != "dev" || ev.CommitSHA != "deadbeef" {
		t.Fatalf("unexpected event: %+v", ev)
	}

	hbad := http.Header{}
	hbad.Set("X-Gitlab-Token", "nope")
	if _, err := VerifyAndParseWebhook("gitlab", hbad, body, secret); err == nil {
		t.Fatal("expected token mismatch error")
	}
}

func TestVerifyAndParseWebhook_Gitea(t *testing.T) {
	secret := "gsecret"
	body := []byte(`{"ref":"refs/heads/main","after":"sha1","repository":{"clone_url":"https://gitea.example.com/u/r.git"}}`)
	h := http.Header{}
	h.Set("X-Gitea-Signature", hmacSig(body, secret))

	ev, err := VerifyAndParseWebhook("gitea", h, body, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.RepoURL != "https://gitea.example.com/u/r.git" || ev.Branch != "main" || ev.CommitSHA != "sha1" {
		t.Fatalf("unexpected event: %+v", ev)
	}

	hbad := http.Header{}
	hbad.Set("X-Gitea-Signature", hmacSig(body, "other"))
	if _, err := VerifyAndParseWebhook("gitea", hbad, body, secret); err == nil {
		t.Fatal("expected signature mismatch error")
	}
}

func TestVerifyAndParseWebhook_Bitbucket(t *testing.T) {
	secret := "bbsecret"
	body := []byte(`{"push":{"changes":[{"new":{"name":"main","target":{"hash":"cafe"}}}]},"repository":{"full_name":"ws/repo"}}`)
	h := http.Header{}
	h.Set("X-Hub-Signature", "sha256="+hmacSig(body, secret))

	ev, err := VerifyAndParseWebhook("bitbucket", h, body, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.RepoURL != "https://bitbucket.org/ws/repo.git" || ev.Branch != "main" || ev.CommitSHA != "cafe" {
		t.Fatalf("unexpected event: %+v", ev)
	}

	if _, err := VerifyAndParseWebhook("bitbucket", h, body, "wrong"); err == nil {
		t.Fatal("expected signature mismatch error")
	}
}

func TestVerifyAndParseWebhook_UnknownProvider(t *testing.T) {
	if _, err := VerifyAndParseWebhook("svn", http.Header{}, []byte(`{}`), "s"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
