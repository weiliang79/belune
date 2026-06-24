package providers

import (
	"net/url"
	"strings"
	"testing"
)

func TestOAuthAuthURL_GitLabSaaS(t *testing.T) {
	p := newOAuthProvider("gitlab")
	cfg := AppConfig{Provider: "gitlab", ClientID: "cid"}
	got := p.AuthURL(cfg, "https://paas.example/cb", "st8")

	if !strings.HasPrefix(got, "https://gitlab.com/oauth/authorize?") {
		t.Fatalf("unexpected base: %s", got)
	}
	u, _ := url.Parse(got)
	q := u.Query()
	if q.Get("client_id") != "cid" || q.Get("state") != "st8" || q.Get("response_type") != "code" {
		t.Fatalf("unexpected query: %s", u.RawQuery)
	}
	if q.Get("redirect_uri") != "https://paas.example/cb" {
		t.Fatalf("unexpected redirect_uri: %s", q.Get("redirect_uri"))
	}
}

func TestOAuthAuthURL_GitLabSelfHosted(t *testing.T) {
	p := newOAuthProvider("gitlab")
	cfg := AppConfig{Provider: "gitlab", BaseURL: "https://gitlab.example.com", ClientID: "cid"}
	got := p.AuthURL(cfg, "https://paas.example/cb", "s")
	if !strings.HasPrefix(got, "https://gitlab.example.com/oauth/authorize?") {
		t.Fatalf("expected self-hosted base, got: %s", got)
	}
}

func TestOAuthAuthURL_Bitbucket(t *testing.T) {
	p := newOAuthProvider("bitbucket")
	got := p.AuthURL(AppConfig{Provider: "bitbucket", ClientID: "c"}, "https://paas.example/cb", "s")
	if !strings.HasPrefix(got, "https://bitbucket.org/site/oauth2/authorize?") {
		t.Fatalf("unexpected bitbucket auth url: %s", got)
	}
}

func TestOAuthAuthURL_Gitea(t *testing.T) {
	p := newOAuthProvider("gitea")
	got := p.AuthURL(AppConfig{Provider: "gitea", BaseURL: "https://gitea.example.com", ClientID: "c"}, "https://paas.example/cb", "s")
	if !strings.HasPrefix(got, "https://gitea.example.com/login/oauth/authorize?") {
		t.Fatalf("unexpected gitea auth url: %s", got)
	}
}

func TestOAuthTokenURL(t *testing.T) {
	cases := map[string]struct {
		provider string
		baseURL  string
		want     string
	}{
		"gitlab":    {"gitlab", "", "https://gitlab.com/oauth/token"},
		"bitbucket": {"bitbucket", "", "https://bitbucket.org/site/oauth2/access_token"},
		"gitea":     {"gitea", "https://gitea.example.com", "https://gitea.example.com/login/oauth/access_token"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := newOAuthProvider(c.provider)
			if got := p.tokenURL(AppConfig{Provider: c.provider, BaseURL: c.baseURL}); got != c.want {
				t.Fatalf("tokenURL = %s, want %s", got, c.want)
			}
		})
	}
}

func TestForUnknownProvider(t *testing.T) {
	if _, err := For("mercurial"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
	for _, p := range []string{"github", "gitlab", "bitbucket", "gitea"} {
		if _, err := For(p); err != nil {
			t.Fatalf("For(%q) unexpected error: %v", p, err)
		}
	}
}
