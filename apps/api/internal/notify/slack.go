package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// slackProvider posts to a Slack incoming webhook using a header block plus a
// coloured attachment so the severity reads at a glance.
type slackProvider struct {
	client *http.Client
}

type slackConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// Slack attachment colour bars (hex) keyed by severity.
const (
	slackColorOK    = "#2ECC71"
	slackColorWarn  = "#F1C40F"
	slackColorError = "#E74C3C"
)

func (p slackProvider) ValidateConfig(raw json.RawMessage) error {
	var c slackConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("slack: invalid config: %w", err)
	}
	if strings.TrimSpace(c.WebhookURL) == "" {
		return fmt.Errorf("slack: webhook_url is required")
	}
	return nil
}

func (p slackProvider) Send(ctx context.Context, raw json.RawMessage, ev Event) error {
	var c slackConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("slack: invalid config: %w", err)
	}

	body := ev.Body
	if ev.Link != "" {
		body = fmt.Sprintf("%s\n<%s|Open in Belune>", body, ev.Link)
	}

	payload, err := json.Marshal(map[string]any{
		"text": ev.Title,
		"attachments": []any{
			map[string]any{
				"color": slackColor(ev.Severity()),
				"blocks": []any{
					map[string]any{
						"type": "section",
						"text": map[string]any{"type": "mrkdwn", "text": body},
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	return postJSON(ctx, p.client, c.WebhookURL, nil, payload)
}

func slackColor(s Severity) string {
	switch s {
	case SeverityError:
		return slackColorError
	case SeverityWarn:
		return slackColorWarn
	default:
		return slackColorOK
	}
}
