package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
)

// telegramAPIBase is the Bot API root. It is a field on the provider so tests can
// point it at an httptest server.
const telegramAPIBase = "https://api.telegram.org"

// telegramProvider sends an HTML message via the Bot API sendMessage method.
type telegramProvider struct {
	client  *http.Client
	apiBase string
}

type telegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func (p telegramProvider) ValidateConfig(raw json.RawMessage) error {
	var c telegramConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("telegram: invalid config: %w", err)
	}
	if strings.TrimSpace(c.BotToken) == "" {
		return fmt.Errorf("telegram: bot_token is required")
	}
	if strings.TrimSpace(c.ChatID) == "" {
		return fmt.Errorf("telegram: chat_id is required")
	}
	return nil
}

func (p telegramProvider) Send(ctx context.Context, raw json.RawMessage, ev Event) error {
	var c telegramConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("telegram: invalid config: %w", err)
	}

	var b strings.Builder
	b.WriteString(severityEmoji(ev.Severity()))
	b.WriteString(" <b>")
	b.WriteString(html.EscapeString(ev.Title))
	b.WriteString("</b>")
	if ev.Body != "" {
		b.WriteString("\n")
		b.WriteString(html.EscapeString(ev.Body))
	}
	if ev.Link != "" {
		b.WriteString("\n<a href=\"")
		b.WriteString(html.EscapeString(ev.Link))
		b.WriteString("\">Open in Belune</a>")
	}

	payload, err := json.Marshal(map[string]any{
		"chat_id":    c.ChatID,
		"text":       b.String(),
		"parse_mode": "HTML",
	})
	if err != nil {
		return err
	}

	base := p.apiBase
	if base == "" {
		base = telegramAPIBase
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(base, "/"), c.BotToken)
	return postJSON(ctx, p.client, url, nil, payload)
}

func severityEmoji(s Severity) string {
	switch s {
	case SeverityError:
		return "🔴"
	case SeverityWarn:
		return "🟡"
	default:
		return "🟢"
	}
}
