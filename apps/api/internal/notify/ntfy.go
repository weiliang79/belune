package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ntfyDefaultServer is the public ntfy.sh instance used when server_url is blank.
const ntfyDefaultServer = "https://ntfy.sh"

// ntfyProvider publishes to an ntfy topic. The message is the request body;
// title, click-through link, priority and tags ride in headers. Self-hosted
// instances are supported via server_url and an optional bearer access token.
type ntfyProvider struct {
	client *http.Client
}

type ntfyConfig struct {
	ServerURL   string `json:"server_url"`
	Topic       string `json:"topic"`
	AccessToken string `json:"access_token"`
}

func (p ntfyProvider) ValidateConfig(raw json.RawMessage) error {
	var c ntfyConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("ntfy: invalid config: %w", err)
	}
	if strings.TrimSpace(c.Topic) == "" {
		return fmt.Errorf("ntfy: topic is required")
	}
	return nil
}

func (p ntfyProvider) Send(ctx context.Context, raw json.RawMessage, ev Event) error {
	var c ntfyConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("ntfy: invalid config: %w", err)
	}

	server := strings.TrimRight(strings.TrimSpace(c.ServerURL), "/")
	if server == "" {
		server = ntfyDefaultServer
	}
	url := fmt.Sprintf("%s/%s", server, c.Topic)

	headers := map[string]string{
		"Title":    sanitizeHeader(ev.Title),
		"Priority": ntfyPriority(ev.Severity()),
		"Tags":     ntfyTag(ev.Severity()),
	}
	if ev.Link != "" {
		headers["Click"] = ev.Link
	}
	if c.AccessToken != "" {
		headers["Authorization"] = "Bearer " + c.AccessToken
	}
	return postBytes(ctx, p.client, url, headers, []byte(ev.Body))
}

// ntfyPriority maps severity onto ntfy's 1..5 priority scale.
func ntfyPriority(s Severity) string {
	switch s {
	case SeverityError:
		return "high"
	case SeverityWarn:
		return "default"
	default:
		return "low"
	}
}

func ntfyTag(s Severity) string {
	switch s {
	case SeverityError:
		return "rotating_light"
	case SeverityWarn:
		return "warning"
	default:
		return "white_check_mark"
	}
}

// sanitizeHeader strips characters that are invalid in an HTTP header value so a
// title with a newline can't break the request.
func sanitizeHeader(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, v)
}
