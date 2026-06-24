package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const githubAPIBase = "https://api.github.com"

// githubApp connects a GitHub account via a GitHub App installation and mints
// short-lived installation access tokens for cloning.
type githubApp struct{}

func (githubApp) AuthURL(cfg AppConfig, _redirectURI, state string) string {
	// The App's redirect_url (configured in the manifest) receives the callback;
	// we only need the install entrypoint plus our state nonce here.
	return fmt.Sprintf("https://github.com/apps/%s/installations/new?state=%s",
		cfg.AppSlug, url.QueryEscape(state))
}

func (githubApp) Connect(ctx context.Context, cfg AppConfig, _redirectURI string, params url.Values) (Integration, error) {
	idStr := params.Get("installation_id")
	if idStr == "" {
		return Integration{}, fmt.Errorf("missing installation_id")
	}
	installationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return Integration{}, fmt.Errorf("invalid installation_id: %w", err)
	}

	jwtTok, err := appJWT(cfg)
	if err != nil {
		return Integration{}, err
	}

	// Resolve the account login for display.
	var inst struct {
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := githubGet(ctx, fmt.Sprintf("%s/app/installations/%d", githubAPIBase, installationID), jwtTok, &inst); err != nil {
		return Integration{}, fmt.Errorf("fetch installation: %w", err)
	}

	return Integration{
		Provider:       "github",
		AccountLogin:   inst.Account.Login,
		InstallationID: installationID,
	}, nil
}

func (githubApp) CloneToken(ctx context.Context, cfg AppConfig, integ *Integration) (string, bool, error) {
	jwtTok, err := appJWT(cfg)
	if err != nil {
		return "", false, err
	}
	var resp struct {
		Token string `json:"token"`
	}
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", githubAPIBase, integ.InstallationID)
	if err := githubPost(ctx, url, jwtTok, &resp); err != nil {
		return "", false, fmt.Errorf("mint installation token: %w", err)
	}
	// Installation tokens are short-lived and not persisted; minted per use.
	return resp.Token, false, nil
}

func (g githubApp) ListRepos(ctx context.Context, cfg AppConfig, integ *Integration) ([]Repo, error) {
	token, _, err := g.CloneToken(ctx, cfg, integ)
	if err != nil {
		return nil, err
	}
	var out []Repo
	page := 1
	for {
		var resp struct {
			Repositories []struct {
				FullName      string `json:"full_name"`
				CloneURL      string `json:"clone_url"`
				Private       bool   `json:"private"`
				DefaultBranch string `json:"default_branch"`
			} `json:"repositories"`
		}
		u := fmt.Sprintf("%s/installation/repositories?per_page=100&page=%d", githubAPIBase, page)
		if err := githubGet(ctx, u, token, &resp); err != nil {
			return nil, err
		}
		for _, r := range resp.Repositories {
			out = append(out, Repo{FullName: r.FullName, CloneURL: r.CloneURL, Private: r.Private, DefaultBranch: r.DefaultBranch})
		}
		if len(resp.Repositories) < 100 {
			break
		}
		page++
		if page > 20 {
			break
		}
	}
	return out, nil
}

func (g githubApp) ListBranches(ctx context.Context, cfg AppConfig, integ *Integration, repoFullName string) ([]Branch, error) {
	token, _, err := g.CloneToken(ctx, cfg, integ)
	if err != nil {
		return nil, err
	}
	var branches []struct {
		Name string `json:"name"`
	}
	u := fmt.Sprintf("%s/repos/%s/branches?per_page=100", githubAPIBase, repoFullName)
	if err := githubGet(ctx, u, token, &branches); err != nil {
		return nil, err
	}
	out := make([]Branch, 0, len(branches))
	for _, b := range branches {
		out = append(out, Branch{Name: b.Name})
	}
	return out, nil
}

// appJWT builds a short-lived RS256 JWT signed with the App private key.
func appJWT(cfg AppConfig) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("parse app private key: %w", err)
	}
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    cfg.AppID,
		IssuedAt:  jwt.NewNumericDate(now.Add(-30 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	})
	return tok.SignedString(key)
}

func githubGet(ctx context.Context, url, bearer string, out any) error {
	return githubDo(ctx, http.MethodGet, url, bearer, out)
}

func githubPost(ctx context.Context, url, bearer string, out any) error {
	return githubDo(ctx, http.MethodPost, url, bearer, out)
}

func githubDo(ctx context.Context, method, url, bearer string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github api %s: status %d", url, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
