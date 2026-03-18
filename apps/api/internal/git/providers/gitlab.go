package providers

import (
	"encoding/json"
	"fmt"
	"strings"
)

type gitLabPushEvent struct {
	Ref     string `json:"ref"`
	After   string `json:"after"`
	Project struct {
		GitHTTPURL string `json:"git_http_url"`
	} `json:"project"`
}

// ParseGitLabWebhook parses a GitLab push webhook payload and verifies the token.
func ParseGitLabWebhook(body []byte, token string, secret string) (repoURL, branch, commitSHA string, err error) {
	// Verify token (simple string comparison)
	if secret != "" && token != secret {
		return "", "", "", fmt.Errorf("token mismatch")
	}

	var event gitLabPushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return "", "", "", fmt.Errorf("parse gitlab payload: %w", err)
	}

	// Extract branch from ref (refs/heads/main -> main)
	branch = strings.TrimPrefix(event.Ref, "refs/heads/")

	return event.Project.GitHTTPURL, branch, event.After, nil
}
