package providers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type gitHubPushEvent struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Repository struct {
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
}

// ParseGitHubWebhook parses a GitHub push webhook payload and verifies the HMAC signature.
func ParseGitHubWebhook(body []byte, signature string, secret string) (repoURL, branch, commitSHA string, err error) {
	// Verify HMAC-SHA256 signature. Fail closed: a missing secret cannot verify.
	if secret == "" {
		return "", "", "", fmt.Errorf("no webhook secret configured")
	}
	if err := verifyGitHubSignature(body, signature, secret); err != nil {
		return "", "", "", err
	}

	var event gitHubPushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return "", "", "", fmt.Errorf("parse github payload: %w", err)
	}

	// Extract branch from ref (refs/heads/main -> main)
	branch = strings.TrimPrefix(event.Ref, "refs/heads/")

	return event.Repository.CloneURL, branch, event.After, nil
}

func verifyGitHubSignature(body []byte, signature string, secret string) error {
	if signature == "" {
		return fmt.Errorf("missing signature header")
	}

	// Signature format: sha256=<hex>
	sig, found := strings.CutPrefix(signature, "sha256=")
	if !found {
		return fmt.Errorf("invalid signature format")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
