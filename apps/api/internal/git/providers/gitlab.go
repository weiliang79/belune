package providers

import "net/http"

// ParseGitLabWebhook parses a GitLab push webhook payload.
func ParseGitLabWebhook(r *http.Request) (repoURL, branch, commitSHA string, err error) {
	// TODO: Parse GitLab webhook JSON payload
	return "", "", "", nil
}
