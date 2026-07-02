package providers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// PushEvent is a normalized push webhook across providers.
type PushEvent struct {
	RepoURL   string
	Branch    string
	CommitSHA string
}

// VerifyAndParseWebhook verifies a push webhook's signature with the provider
// app's webhook secret and returns the normalized event.
func VerifyAndParseWebhook(provider string, header http.Header, body []byte, secret string) (PushEvent, error) {
	switch provider {
	case "github":
		repo, branch, sha, err := ParseGitHubWebhook(body, header.Get("X-Hub-Signature-256"), secret)
		return PushEvent{RepoURL: repo, Branch: branch, CommitSHA: sha}, err
	case "gitlab":
		repo, branch, sha, err := ParseGitLabWebhook(body, header.Get("X-Gitlab-Token"), secret)
		return PushEvent{RepoURL: repo, Branch: branch, CommitSHA: sha}, err
	case "gitea":
		return parseGiteaWebhook(body, header.Get("X-Gitea-Signature"), secret)
	case "bitbucket":
		return parseBitbucketWebhook(body, header.Get("X-Hub-Signature"), secret)
	default:
		return PushEvent{}, fmt.Errorf("unknown provider: %s", provider)
	}
}

// parseGiteaWebhook verifies the HMAC-SHA256 hex signature (X-Gitea-Signature,
// no prefix) and parses the GitHub-shaped push payload.
func parseGiteaWebhook(body []byte, signature, secret string) (PushEvent, error) {
	// Fail closed: a missing secret cannot verify the signature.
	if secret == "" {
		return PushEvent{}, fmt.Errorf("no webhook secret configured")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return PushEvent{}, fmt.Errorf("signature mismatch")
	}
	var ev struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Repository struct {
			CloneURL string `json:"clone_url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return PushEvent{}, fmt.Errorf("parse gitea payload: %w", err)
	}
	return PushEvent{
		RepoURL:   ev.Repository.CloneURL,
		Branch:    strings.TrimPrefix(ev.Ref, "refs/heads/"),
		CommitSHA: ev.After,
	}, nil
}

// parseBitbucketWebhook verifies the HMAC-SHA256 signature (X-Hub-Signature:
// sha256=<hex>) and parses the Bitbucket push payload.
func parseBitbucketWebhook(body []byte, signature, secret string) (PushEvent, error) {
	// Fail closed: a missing secret cannot verify the signature.
	if secret == "" {
		return PushEvent{}, fmt.Errorf("no webhook secret configured")
	}
	sig, found := strings.CutPrefix(signature, "sha256=")
	if !found {
		return PushEvent{}, fmt.Errorf("invalid signature format")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return PushEvent{}, fmt.Errorf("signature mismatch")
	}
	var ev struct {
		Push struct {
			Changes []struct {
				New struct {
					Name   string `json:"name"`
					Target struct {
						Hash string `json:"hash"`
					} `json:"target"`
				} `json:"new"`
			} `json:"changes"`
		} `json:"push"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return PushEvent{}, fmt.Errorf("parse bitbucket payload: %w", err)
	}
	if len(ev.Push.Changes) == 0 {
		return PushEvent{}, fmt.Errorf("no changes in push")
	}
	c := ev.Push.Changes[0]
	return PushEvent{
		RepoURL:   "https://bitbucket.org/" + ev.Repository.FullName + ".git",
		Branch:    c.New.Name,
		CommitSHA: c.New.Target.Hash,
	}, nil
}
