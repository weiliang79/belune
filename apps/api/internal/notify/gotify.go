package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// gotifyProvider posts a message to a self-hosted Gotify server. The app token
// authenticates via the X-Gotify-Key header. The click-through link is appended
// to the message body and marked up so Gotify clients render it.
type gotifyProvider struct {
	client *http.Client
}

type gotifyConfig struct {
	ServerURL string `json:"server_url"`
	AppToken  string `json:"app_token"`
}

func (p gotifyProvider) ValidateConfig(raw json.RawMessage) error {
	var c gotifyConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("gotify: invalid config: %w", err)
	}
	if strings.TrimSpace(c.ServerURL) == "" {
		return fmt.Errorf("gotify: server_url is required")
	}
	if strings.TrimSpace(c.AppToken) == "" {
		return fmt.Errorf("gotify: app_token is required")
	}
	return nil
}

func (p gotifyProvider) Send(ctx context.Context, raw json.RawMessage, ev Event) error {
	var c gotifyConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("gotify: invalid config: %w", err)
	}

	message := ev.Body
	extras := map[string]any{}
	if ev.Link != "" {
		message = fmt.Sprintf("%s\n\n[Open in Belune](%s)", message, ev.Link)
		extras["client::display"] = map[string]any{"contentType": "text/markdown"}
		extras["client::notification"] = map[string]any{"click": map[string]any{"url": ev.Link}}
	}

	payload, err := json.Marshal(map[string]any{
		"title":    ev.Title,
		"message":  message,
		"priority": gotifyPriority(ev.Severity()),
		"extras":   extras,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/message", strings.TrimRight(strings.TrimSpace(c.ServerURL), "/"))
	return postJSON(ctx, p.client, url, map[string]string{"X-Gotify-Key": c.AppToken}, payload)
}

// gotifyPriority maps severity onto Gotify's numeric priority (higher = louder).
func gotifyPriority(s Severity) int {
	switch s {
	case SeverityError:
		return 8
	case SeverityWarn:
		return 5
	default:
		return 2
	}
}
