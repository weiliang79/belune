package providers

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"strings"
)

type gitLabPushEvent struct {
	Ref     string         `json:"ref"`
	After   string         `json:"after"`
	Commits []commitObject `json:"commits"`
	Project struct {
		GitHTTPURL string `json:"git_http_url"`
	} `json:"project"`
}

// ParseGitLabWebhook parses a GitLab push webhook payload and verifies the token.
func ParseGitLabWebhook(body []byte, token string, secret string) (PushEvent, error) {
	// Verify the shared token in constant time. Fail closed: a missing secret
	// cannot verify.
	if secret == "" {
		return PushEvent{}, fmt.Errorf("no webhook secret configured")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
		return PushEvent{}, fmt.Errorf("token mismatch")
	}

	var event gitLabPushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return PushEvent{}, fmt.Errorf("parse gitlab payload: %w", err)
	}

	// GitLab has no head_commit; the tip is found by matching `after`.
	message, author := commitDetails(pickCommit(nil, event.Commits, event.After))
	return PushEvent{
		RepoURL: event.Project.GitHTTPURL,
		// Extract branch from ref (refs/heads/main -> main)
		Branch:        strings.TrimPrefix(event.Ref, "refs/heads/"),
		CommitSHA:     event.After,
		CommitMessage: message,
		CommitAuthor:  author,
	}, nil
}
