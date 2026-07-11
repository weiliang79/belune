package git

import (
	"fmt"
	"net/http"

	"github.com/weiliang79/belune/internal/git/providers"
)

// WebhookPayload represents a parsed webhook event.
type WebhookPayload struct {
	Provider  string // "github" or "gitlab"
	RepoURL   string
	Branch    string
	CommitSHA string
}

// ParseWebhook parses an incoming webhook request from a git provider.
// It detects the provider from headers and verifies the signature using the provided secret.
func ParseWebhook(r *http.Request, body []byte, secret string) (*WebhookPayload, error) {
	if r.Header.Get("X-GitHub-Event") != "" {
		if r.Header.Get("X-GitHub-Event") != "push" {
			return nil, fmt.Errorf("ignoring non-push github event: %s", r.Header.Get("X-GitHub-Event"))
		}
		repoURL, branch, commitSHA, err := providers.ParseGitHubWebhook(
			body,
			r.Header.Get("X-Hub-Signature-256"),
			secret,
		)
		if err != nil {
			return nil, fmt.Errorf("github: %w", err)
		}
		return &WebhookPayload{
			Provider:  "github",
			RepoURL:   repoURL,
			Branch:    branch,
			CommitSHA: commitSHA,
		}, nil
	}

	if r.Header.Get("X-Gitlab-Event") != "" {
		if r.Header.Get("X-Gitlab-Event") != "Push Hook" {
			return nil, fmt.Errorf("ignoring non-push gitlab event: %s", r.Header.Get("X-Gitlab-Event"))
		}
		repoURL, branch, commitSHA, err := providers.ParseGitLabWebhook(
			body,
			r.Header.Get("X-Gitlab-Token"),
			secret,
		)
		if err != nil {
			return nil, fmt.Errorf("gitlab: %w", err)
		}
		return &WebhookPayload{
			Provider:  "gitlab",
			RepoURL:   repoURL,
			Branch:    branch,
			CommitSHA: commitSHA,
		}, nil
	}

	return nil, fmt.Errorf("unknown webhook provider")
}
