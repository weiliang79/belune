package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// emailProvider fans an event out to one or more recipients through the existing
// SMTP mailer, so an admin can receive provider notifications by email without a
// second delivery path. It intentionally reuses the same Mailer the rest of the
// app dials; when no mailer is wired (SMTP unconfigured) Send reports an error.
type emailProvider struct {
	mailer Mailer
}

type emailConfig struct {
	Recipients []string `json:"recipients"`
}

func (p emailProvider) ValidateConfig(raw json.RawMessage) error {
	var c emailConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("email: invalid config: %w", err)
	}
	if len(nonEmpty(c.Recipients)) == 0 {
		return fmt.Errorf("email: at least one recipient is required")
	}
	return nil
}

func (p emailProvider) Send(ctx context.Context, raw json.RawMessage, ev Event) error {
	var c emailConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("email: invalid config: %w", err)
	}
	if p.mailer == nil {
		return fmt.Errorf("email: no mailer configured (set SMTP_HOST)")
	}

	body := ev.Body
	if ev.Link != "" {
		body = fmt.Sprintf("%s\n\n%s", body, ev.Link)
	}

	var firstErr error
	for _, to := range nonEmpty(c.Recipients) {
		msg := MailMessage{To: to, Subject: ev.Title, TextBody: body}
		if err := p.mailer.Send(ctx, msg); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("email: send to %s: %w", to, err)
		}
	}
	return firstErr
}

func nonEmpty(in []string) []string {
	out := in[:0:0]
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
