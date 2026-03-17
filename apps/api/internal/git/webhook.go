package git

import "net/http"

// WebhookPayload represents a parsed webhook event.
type WebhookPayload struct {
	Provider  string // "github" or "gitlab"
	RepoURL   string
	Branch    string
	CommitSHA string
}

// ParseWebhook parses an incoming webhook request from a git provider.
func ParseWebhook(r *http.Request) (*WebhookPayload, error) {
	// TODO: Detect provider and delegate to provider-specific parser
	return nil, nil
}
