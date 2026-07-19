package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// webhookProvider POSTs a stable JSON envelope to an arbitrary endpoint, the
// escape hatch for integrations without a first-class adapter. When a secret is
// configured the body is signed with HMAC-SHA256 in the X-Belune-Signature
// header (sha256=<hex>), matching the git webhook signing convention.
type webhookProvider struct {
	client *http.Client
}

type webhookConfig struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

// webhookPayload is the documented, stable shape delivered to the endpoint.
type webhookPayload struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Link       string `json:"link"`
	Severity   string `json:"severity"`
	OccurredAt string `json:"occurred_at"`
}

func (p webhookProvider) ValidateConfig(raw json.RawMessage) error {
	var c webhookConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("webhook: invalid config: %w", err)
	}
	if strings.TrimSpace(c.URL) == "" {
		return fmt.Errorf("webhook: url is required")
	}
	return nil
}

func (p webhookProvider) Send(ctx context.Context, raw json.RawMessage, ev Event) error {
	var c webhookConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("webhook: invalid config: %w", err)
	}

	occurred := ""
	if !ev.OccurredAt.IsZero() {
		occurred = ev.OccurredAt.UTC().Format(time.RFC3339)
	}
	body, err := json.Marshal(webhookPayload{
		Type:       ev.Type,
		Title:      ev.Title,
		Body:       ev.Body,
		Link:       ev.Link,
		Severity:   string(ev.Severity()),
		OccurredAt: occurred,
	})
	if err != nil {
		return err
	}

	var headers map[string]string
	if c.Secret != "" {
		mac := hmac.New(sha256.New, []byte(c.Secret))
		mac.Write(body)
		headers = map[string]string{"X-Belune-Signature": "sha256=" + hex.EncodeToString(mac.Sum(nil))}
	}
	return postJSON(ctx, p.client, c.URL, headers, body)
}
