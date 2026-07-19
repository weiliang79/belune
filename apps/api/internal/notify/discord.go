package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// discordProvider posts a rich embed to a Discord channel webhook.
type discordProvider struct {
	client *http.Client
}

type discordConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// Discord embed colours (decimal RGB) keyed by severity.
const (
	discordColorOK    = 0x2ECC71 // green
	discordColorWarn  = 0xF1C40F // amber
	discordColorError = 0xE74C3C // red
)

func (p discordProvider) ValidateConfig(raw json.RawMessage) error {
	var c discordConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("discord: invalid config: %w", err)
	}
	if strings.TrimSpace(c.WebhookURL) == "" {
		return fmt.Errorf("discord: webhook_url is required")
	}
	return nil
}

func (p discordProvider) Send(ctx context.Context, raw json.RawMessage, ev Event) error {
	var c discordConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("discord: invalid config: %w", err)
	}

	embed := map[string]any{
		"title":       ev.Title,
		"description": ev.Body,
		"color":       discordColor(ev.Severity()),
	}
	if ev.Link != "" {
		embed["url"] = ev.Link
	}
	if !ev.OccurredAt.IsZero() {
		embed["timestamp"] = ev.OccurredAt.UTC().Format(time.RFC3339)
	}

	payload, err := json.Marshal(map[string]any{"embeds": []any{embed}})
	if err != nil {
		return err
	}
	return postJSON(ctx, p.client, c.WebhookURL, nil, payload)
}

func discordColor(s Severity) int {
	switch s {
	case SeverityError:
		return discordColorError
	case SeverityWarn:
		return discordColorWarn
	default:
		return discordColorOK
	}
}
