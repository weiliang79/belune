package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const bitbucketAPIBase = "https://api.bitbucket.org/2.0"

// oauthProvider implements Connector for the OAuth2 providers (GitLab,
// Bitbucket, Gitea). Provider-specific endpoints and API shapes are switched on
// the name; token exchange/refresh is shared.
type oauthProvider struct {
	name string
}

func newOAuthProvider(name string) oauthProvider { return oauthProvider{name: name} }

// base returns the provider base URL (config override, else the SaaS default).
func (p oauthProvider) base(cfg AppConfig) string {
	if cfg.BaseURL != "" {
		return strings.TrimRight(cfg.BaseURL, "/")
	}
	switch p.name {
	case "gitlab":
		return "https://gitlab.com"
	case "bitbucket":
		return "https://bitbucket.org"
	default:
		return ""
	}
}

func (p oauthProvider) scopes() string {
	switch p.name {
	case "gitlab":
		return "read_api read_repository read_user"
	case "bitbucket":
		return "repository account"
	default: // gitea
		return "read:repository read:user"
	}
}

func (p oauthProvider) AuthURL(cfg AppConfig, redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	if s := p.scopes(); s != "" {
		q.Set("scope", s)
	}
	switch p.name {
	case "bitbucket":
		return "https://bitbucket.org/site/oauth2/authorize?" + q.Encode()
	case "gitea":
		return p.base(cfg) + "/login/oauth/authorize?" + q.Encode()
	default: // gitlab
		return p.base(cfg) + "/oauth/authorize?" + q.Encode()
	}
}

func (p oauthProvider) tokenURL(cfg AppConfig) string {
	switch p.name {
	case "bitbucket":
		return "https://bitbucket.org/site/oauth2/access_token"
	case "gitea":
		return p.base(cfg) + "/login/oauth/access_token"
	default: // gitlab
		return p.base(cfg) + "/oauth/token"
	}
}

type oauthTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// exchange performs an authorization_code (code set) or refresh_token
// (refreshToken set) grant.
func (p oauthProvider) exchange(ctx context.Context, cfg AppConfig, redirectURI, code, refreshToken string) (oauthTokenResp, error) {
	form := url.Values{}
	if refreshToken != "" {
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", refreshToken)
	} else {
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("redirect_uri", redirectURI)
	}
	// Bitbucket authenticates the client via HTTP Basic; the others accept the
	// client id/secret in the form body.
	useBasic := p.name == "bitbucket"
	if !useBasic {
		form.Set("client_id", cfg.ClientID)
		form.Set("client_secret", cfg.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL(cfg), strings.NewReader(form.Encode()))
	if err != nil {
		return oauthTokenResp{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if useBasic {
		req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return oauthTokenResp{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauthTokenResp{}, fmt.Errorf("oauth token exchange: status %d", resp.StatusCode)
	}
	var tok oauthTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return oauthTokenResp{}, err
	}
	return tok, nil
}

func (p oauthProvider) Connect(ctx context.Context, cfg AppConfig, redirectURI string, params url.Values) (Integration, error) {
	code := params.Get("code")
	if code == "" {
		return Integration{}, fmt.Errorf("missing code")
	}
	tok, err := p.exchange(ctx, cfg, redirectURI, code, "")
	if err != nil {
		return Integration{}, err
	}
	integ := Integration{
		Provider:     p.name,
		BaseURL:      cfg.BaseURL,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
	}
	if tok.ExpiresIn > 0 {
		integ.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	login, err := p.accountLogin(ctx, cfg, tok.AccessToken)
	if err != nil {
		return Integration{}, fmt.Errorf("fetch account: %w", err)
	}
	integ.AccountLogin = login
	return integ, nil
}

func (p oauthProvider) CloneToken(ctx context.Context, cfg AppConfig, integ *Integration) (string, bool, error) {
	// Reuse a still-valid access token.
	if integ.AccessToken != "" && (integ.Expiry.IsZero() || time.Now().Before(integ.Expiry.Add(-60*time.Second))) {
		return integ.AccessToken, false, nil
	}
	if integ.RefreshToken == "" {
		if integ.AccessToken != "" {
			return integ.AccessToken, false, nil
		}
		return "", false, fmt.Errorf("no token available")
	}
	tok, err := p.exchange(ctx, cfg, "", "", integ.RefreshToken)
	if err != nil {
		return "", false, err
	}
	integ.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		integ.RefreshToken = tok.RefreshToken
	}
	if tok.ExpiresIn > 0 {
		integ.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	return integ.AccessToken, true, nil
}

func (p oauthProvider) accountLogin(ctx context.Context, cfg AppConfig, token string) (string, error) {
	switch p.name {
	case "gitlab":
		var u struct {
			Username string `json:"username"`
		}
		if err := oauthAPIGet(ctx, p.base(cfg)+"/api/v4/user", token, &u); err != nil {
			return "", err
		}
		return u.Username, nil
	case "bitbucket":
		var u struct {
			Username string `json:"username"`
		}
		if err := oauthAPIGet(ctx, bitbucketAPIBase+"/user", token, &u); err != nil {
			return "", err
		}
		return u.Username, nil
	default: // gitea
		var u struct {
			Login string `json:"login"`
		}
		if err := oauthAPIGet(ctx, p.base(cfg)+"/api/v1/user", token, &u); err != nil {
			return "", err
		}
		return u.Login, nil
	}
}

func (p oauthProvider) ListRepos(ctx context.Context, cfg AppConfig, integ *Integration) ([]Repo, error) {
	token, _, err := p.CloneToken(ctx, cfg, integ)
	if err != nil {
		return nil, err
	}
	switch p.name {
	case "gitlab":
		var projs []struct {
			PathWithNamespace string `json:"path_with_namespace"`
			HTTPURLToRepo     string `json:"http_url_to_repo"`
			Visibility        string `json:"visibility"`
			DefaultBranch     string `json:"default_branch"`
		}
		if err := oauthAPIGet(ctx, p.base(cfg)+"/api/v4/projects?membership=true&per_page=100&simple=true", token, &projs); err != nil {
			return nil, err
		}
		out := make([]Repo, 0, len(projs))
		for _, r := range projs {
			out = append(out, Repo{FullName: r.PathWithNamespace, CloneURL: r.HTTPURLToRepo, Private: r.Visibility != "public", DefaultBranch: r.DefaultBranch})
		}
		return out, nil
	case "bitbucket":
		var resp struct {
			Values []struct {
				FullName   string `json:"full_name"`
				IsPrivate  bool   `json:"is_private"`
				Mainbranch struct {
					Name string `json:"name"`
				} `json:"mainbranch"`
				Links struct {
					Clone []struct {
						Name string `json:"name"`
						Href string `json:"href"`
					} `json:"clone"`
				} `json:"links"`
			} `json:"values"`
		}
		if err := oauthAPIGet(ctx, bitbucketAPIBase+"/repositories?role=member&pagelen=100", token, &resp); err != nil {
			return nil, err
		}
		out := make([]Repo, 0, len(resp.Values))
		for _, r := range resp.Values {
			clone := ""
			for _, c := range r.Links.Clone {
				if c.Name == "https" {
					clone = c.Href
				}
			}
			out = append(out, Repo{FullName: r.FullName, CloneURL: clone, Private: r.IsPrivate, DefaultBranch: r.Mainbranch.Name})
		}
		return out, nil
	default: // gitea
		var repos []struct {
			FullName      string `json:"full_name"`
			CloneURL      string `json:"clone_url"`
			Private       bool   `json:"private"`
			DefaultBranch string `json:"default_branch"`
		}
		if err := oauthAPIGet(ctx, p.base(cfg)+"/api/v1/user/repos?limit=50", token, &repos); err != nil {
			return nil, err
		}
		out := make([]Repo, 0, len(repos))
		for _, r := range repos {
			out = append(out, Repo{FullName: r.FullName, CloneURL: r.CloneURL, Private: r.Private, DefaultBranch: r.DefaultBranch})
		}
		return out, nil
	}
}

func (p oauthProvider) ListBranches(ctx context.Context, cfg AppConfig, integ *Integration, repoFullName string) ([]Branch, error) {
	token, _, err := p.CloneToken(ctx, cfg, integ)
	if err != nil {
		return nil, err
	}
	switch p.name {
	case "gitlab":
		var branches []struct {
			Name string `json:"name"`
		}
		u := p.base(cfg) + "/api/v4/projects/" + url.QueryEscape(repoFullName) + "/repository/branches?per_page=100"
		if err := oauthAPIGet(ctx, u, token, &branches); err != nil {
			return nil, err
		}
		return namesToBranches(toNames(branches)), nil
	case "bitbucket":
		var resp struct {
			Values []struct {
				Name string `json:"name"`
			} `json:"values"`
		}
		if err := oauthAPIGet(ctx, bitbucketAPIBase+"/repositories/"+repoFullName+"/refs/branches?pagelen=100", token, &resp); err != nil {
			return nil, err
		}
		out := make([]Branch, 0, len(resp.Values))
		for _, b := range resp.Values {
			out = append(out, Branch{Name: b.Name})
		}
		return out, nil
	default: // gitea
		var branches []struct {
			Name string `json:"name"`
		}
		if err := oauthAPIGet(ctx, p.base(cfg)+"/api/v1/repos/"+repoFullName+"/branches?limit=100", token, &branches); err != nil {
			return nil, err
		}
		return namesToBranches(toNames(branches)), nil
	}
}

func toNames(items []struct {
	Name string `json:"name"`
}) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.Name)
	}
	return out
}

func namesToBranches(names []string) []Branch {
	out := make([]Branch, 0, len(names))
	for _, n := range names {
		out = append(out, Branch{Name: n})
	}
	return out
}

func oauthAPIGet(ctx context.Context, apiURL, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider api %s: status %d", apiURL, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
