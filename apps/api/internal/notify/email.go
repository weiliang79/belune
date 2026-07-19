package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// emailProvider fans an event out to one or more recipients. By default it uses
// the instance SMTP transport (Server → Email settings); a channel may instead
// supply its own SMTP server via the optional "smtp" config block (Option B), so
// alerts can route through a different provider than the app's transactional mail.
type emailProvider struct {
	mailer Mailer
}

type emailConfig struct {
	Recipients []string   `json:"recipients"`
	SMTP       *emailSMTP `json:"smtp,omitempty"`
}

// emailSMTP is the per-channel mail-server override. When Host is set it fully
// replaces the instance SMTP for this channel.
type emailSMTP struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	FromEmail string `json:"from_email"`
	FromName  string `json:"from_name"`
	TLSMode   string `json:"tls_mode"`
}

func (c emailConfig) hasOverride() bool {
	return c.SMTP != nil && strings.TrimSpace(c.SMTP.Host) != ""
}

func (p emailProvider) ValidateConfig(raw json.RawMessage) error {
	var c emailConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("email: invalid config: %w", err)
	}
	if len(nonEmpty(c.Recipients)) == 0 {
		return fmt.Errorf("email: at least one recipient is required")
	}
	if c.SMTP != nil {
		// A partial override (some fields but no host) is a mistake worth catching.
		anySet := strings.TrimSpace(c.SMTP.User) != "" ||
			strings.TrimSpace(c.SMTP.Password) != "" ||
			strings.TrimSpace(c.SMTP.FromEmail) != "" ||
			c.SMTP.Port != 0
		if strings.TrimSpace(c.SMTP.Host) == "" && anySet {
			return fmt.Errorf("email: a custom mail server requires a host")
		}
		if m := strings.TrimSpace(c.SMTP.TLSMode); m != "" && m != "none" && m != "starttls" && m != "tls" {
			return fmt.Errorf("email: invalid TLS mode %q (expected none, starttls, or tls)", c.SMTP.TLSMode)
		}
	}
	return nil
}

func (p emailProvider) Send(ctx context.Context, raw json.RawMessage, ev Event) error {
	var c emailConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("email: invalid config: %w", err)
	}
	if p.mailer == nil {
		return fmt.Errorf("email: no mailer available")
	}

	override := c.hasOverride()
	// Without a channel override we rely on the instance SMTP; if that is not
	// configured, fail loudly rather than dropping into silent log-only mode.
	if !override && !p.mailer.Configured(ctx) {
		return fmt.Errorf("email: SMTP is not configured — set it under Server → Email (SMTP), or give this channel its own mail server: %w", ErrPermanent)
	}

	body := ev.Body
	if ev.Link != "" {
		body = fmt.Sprintf("%s\n\n%s", body, ev.Link)
	}

	var firstErr error
	for _, to := range nonEmpty(c.Recipients) {
		msg := MailMessage{To: to, Subject: ev.Title, TextBody: body}
		var err error
		if override {
			err = p.mailer.SendWithConfig(ctx, c.SMTP.toMailSMTP(), msg)
		} else {
			err = p.mailer.Send(ctx, msg)
		}
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("email: send to %s: %w", to, err)
		}
	}
	return firstErr
}

func (o *emailSMTP) toMailSMTP() MailSMTP {
	return MailSMTP{
		Host:      strings.TrimSpace(o.Host),
		Port:      o.Port,
		User:      strings.TrimSpace(o.User),
		Password:  o.Password,
		FromEmail: strings.TrimSpace(o.FromEmail),
		FromName:  strings.TrimSpace(o.FromName),
		TLSMode:   strings.TrimSpace(o.TLSMode),
	}
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
