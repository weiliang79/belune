package providers

import (
	"crypto/subtle"
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
	// Verify the shared token in constant time. Fail closed: a missing secret
	// cannot verify.
	if secret == "" {
		return "", "", "", fmt.Errorf("no webhook secret configured")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
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
