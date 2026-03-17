package providers

import "net/http"

// ParseGitHubWebhook parses a GitHub push webhook payload.
func ParseGitHubWebhook(r *http.Request) (repoURL, branch, commitSHA string, err error) {
	// TODO: Parse GitHub webhook JSON payload
	return "", "", "", nil
}
