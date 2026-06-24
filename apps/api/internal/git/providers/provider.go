// Package providers implements the git provider abstraction: connecting an
// account (GitHub App install or OAuth), minting short-lived clone tokens,
// listing repos/branches for pickers, and parsing/verifying push webhooks.
package providers

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// Integration is a connected account's decrypted state. For a GitHub App this
// is the installation id; for OAuth providers it is the access/refresh token.
type Integration struct {
	Provider       string    `json:"-"`
	BaseURL        string    `json:"-"`
	AccountLogin   string    `json:"-"`
	InstallationID int64     `json:"installation_id,omitempty"`
	AccessToken    string    `json:"access_token,omitempty"`
	RefreshToken   string    `json:"refresh_token,omitempty"`
	Expiry         time.Time `json:"expiry,omitempty"`
}

// AppConfig is the per-instance provider app/client config (decrypted).
type AppConfig struct {
	Provider     string
	BaseURL      string
	ClientID     string
	ClientSecret string
	AppID        string
	AppSlug      string
	PrivateKey   string // GitHub App PEM
}

// Repo is a repository shown in the picker.
type Repo struct {
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
}

// Branch is a branch shown in the picker.
type Branch struct {
	Name string `json:"name"`
}

// Connector connects an account and serves repos/branches/clone tokens for it.
type Connector interface {
	// AuthURL is the provider URL the user is redirected to in order to connect
	// (OAuth authorize URL, or the GitHub App installation URL).
	AuthURL(cfg AppConfig, redirectURI, state string) string

	// Connect completes the connect callback. params carries the provider's
	// callback query (code, installation_id, ...). It returns the integration
	// state to persist.
	Connect(ctx context.Context, cfg AppConfig, redirectURI string, params url.Values) (Integration, error)

	// CloneToken returns a usable token for cloning over HTTPS, minting or
	// refreshing as needed. When changed is true the caller must persist integ.
	CloneToken(ctx context.Context, cfg AppConfig, integ *Integration) (token string, changed bool, err error)

	// ListRepos / ListBranches back the application repo/branch pickers.
	ListRepos(ctx context.Context, cfg AppConfig, integ *Integration) ([]Repo, error)
	ListBranches(ctx context.Context, cfg AppConfig, integ *Integration, repoFullName string) ([]Branch, error)
}

// For returns the Connector for a provider name.
func For(provider string) (Connector, error) {
	switch provider {
	case "github":
		return githubApp{}, nil
	case "gitlab":
		return newOAuthProvider("gitlab"), nil
	case "bitbucket":
		return newOAuthProvider("bitbucket"), nil
	case "gitea":
		return newOAuthProvider("gitea"), nil
	default:
		return nil, fmt.Errorf("unknown git provider: %q", provider)
	}
}
