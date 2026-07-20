package providers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ParseGitHubWebhook parses a GitHub push webhook payload and verifies the HMAC signature.
func ParseGitHubWebhook(body []byte, signature string, secret string) (PushEvent, error) {
	// Verify HMAC-SHA256 signature. Fail closed: a missing secret cannot verify.
	if secret == "" {
		return PushEvent{}, fmt.Errorf("no webhook secret configured")
	}
	if err := verifyGitHubSignature(body, signature, secret); err != nil {
		return PushEvent{}, err
	}

	var event githubShapedPush
	if err := json.Unmarshal(body, &event); err != nil {
		return PushEvent{}, fmt.Errorf("parse github payload: %w", err)
	}

	message, author := commitDetails(pickCommit(event.HeadCommit, event.Commits, event.After))
	return PushEvent{
		RepoURL: event.Repository.CloneURL,
		// Extract branch from ref (refs/heads/main -> main)
		Branch:        strings.TrimPrefix(event.Ref, "refs/heads/"),
		CommitSHA:     event.After,
		CommitMessage: message,
		CommitAuthor:  author,
	}, nil
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
