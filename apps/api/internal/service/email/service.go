// Package email provides an async-safe email sending service with template support.
// When SMTP_HOST is empty, messages are written to slog instead of dialed.
package email

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	mail "github.com/wneessen/go-mail"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/weiliang79/belune/internal/config"
	"github.com/weiliang79/belune/internal/pkg/metrics"
	"github.com/weiliang79/belune/internal/pkg/tracing"
)

// Message represents a single outbound email.
type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
	Headers  map[string]string
}

// Service sends emails via SMTP or logs them when no SMTP host is configured.
type Service struct {
	cfg      *config.Config
	baseURL  *url.URL // parsed PUBLIC_BASE_URL; nil if absent/unparseable
	registry map[string]*templateDef
}

// New constructs an email Service from cfg. Returns an error if the embedded
// templates cannot be parsed — this indicates a broken binary and the caller
// should log and exit.
func New(cfg *config.Config) (*Service, error) {
	reg, err := loadTemplates()
	if err != nil {
		return nil, err
	}

	svc := &Service{cfg: cfg, registry: reg}

	if cfg.SMTPHost != "" {
		if cfg.PublicBaseURL == "" {
			slog.Warn("SMTP_HOST is set but PUBLIC_BASE_URL is empty — email sending disabled until PUBLIC_BASE_URL is configured")
		} else if u, err := url.Parse(cfg.PublicBaseURL); err != nil || u.Host == "" {
			slog.Warn("SMTP_HOST is set but PUBLIC_BASE_URL is not a valid absolute URL — email sending disabled",
				"public_base_url", cfg.PublicBaseURL)
		} else {
			svc.baseURL = u
		}
	}

	return svc, nil
}

// PublicURL returns the configured PUBLIC_BASE_URL. Returns "" if not set.
func (s *Service) PublicURL() string {
	return s.cfg.PublicBaseURL
}

// Render executes a named template with vars and returns (subject, textBody, htmlBody).
// Useful for tests and future debugging surfaces.
func (s *Service) Render(templateID string, vars any) (subject, textBody, htmlBody string, err error) {
	return renderTemplate(s.registry, templateID, vars)
}

// SendTemplate renders the named template with vars and sends it to addr.
// Always called from the async email task — never from a hot path.
func (s *Service) SendTemplate(ctx context.Context, templateID, addr string, vars any) error {
	ctx, span := tracing.Tracer().Start(ctx, "email.send",
		trace.WithAttributes(
			attribute.String("email.template", templateID),
			attribute.String("email.recipient_hash", hashRecipient(addr)),
		),
	)
	defer span.End()

	subject, textBody, htmlBody, err := renderTemplate(s.registry, templateID, vars)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	msg := Message{
		To:       addr,
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}

	// Only instrument when SMTP is actually configured; log-only sends are
	// cheap no-ops that would skew duration histograms with near-zero values.
	if s.cfg.SMTPHost == "" {
		return s.Send(ctx, msg)
	}

	start := time.Now()
	sendErr := s.Send(ctx, msg)
	metrics.RecordSMTPSend(templateID, sendErr, time.Since(start))
	if sendErr != nil {
		span.RecordError(sendErr)
		span.SetStatus(codes.Error, sendErr.Error())
	}
	return sendErr
}

// Send delivers msg via SMTP. If no SMTP host is configured, the rendered
// message is written to slog at INFO level (dev/log-only fallback).
func (s *Service) Send(ctx context.Context, msg Message) error {
	if s.cfg.SMTPHost == "" {
		slog.InfoContext(ctx, "email (log-only mode)",
			"to", msg.To,
			"subject", msg.Subject,
			"text_body", msg.TextBody,
		)
		return nil
	}

	// Refuse to send if PUBLIC_BASE_URL was missing/invalid at startup.
	if s.baseURL == nil {
		slog.WarnContext(ctx, "email send skipped: PUBLIC_BASE_URL not configured",
			"to", msg.To,
			"subject", msg.Subject,
		)
		return nil
	}

	m := mail.NewMsg()
	if err := m.FromFormat(s.cfg.SMTPFromName, s.cfg.SMTPFromEmail); err != nil {
		return fmt.Errorf("email: set from: %w", err)
	}
	if err := m.To(msg.To); err != nil {
		return fmt.Errorf("email: set to: %w", err)
	}
	m.Subject(msg.Subject)
	if msg.TextBody != "" {
		m.SetBodyString(mail.TypeTextPlain, msg.TextBody)
	}
	if msg.HTMLBody != "" {
		m.AddAlternativeString(mail.TypeTextHTML, msg.HTMLBody)
	}
	for k, v := range msg.Headers {
		m.SetGenHeader(mail.Header(k), v)
	}

	client, err := s.newClient()
	if err != nil {
		return fmt.Errorf("email: create client: %w", err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, smtpSpan := tracing.Tracer().Start(ctx, "email.smtp.send")
	defer smtpSpan.End()

	if err := client.DialAndSendWithContext(dialCtx, m); err != nil {
		smtpSpan.RecordError(err)
		smtpSpan.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("email: send: %w", err)
	}

	slog.InfoContext(ctx, "email sent", "to", msg.To, "subject", msg.Subject)
	return nil
}

// hashRecipient returns a short SHA-256 hex digest of addr for trace attributes.
// The full address is never embedded in traces so PII stays out of telemetry.
func hashRecipient(addr string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(addr))))
	return hex.EncodeToString(sum[:8])
}

func (s *Service) newClient() (*mail.Client, error) {
	opts := []mail.Option{
		mail.WithPort(s.cfg.SMTPPort),
		mail.WithTimeout(30 * time.Second),
	}

	if s.cfg.SMTPUser != "" {
		opts = append(opts, mail.WithUsername(s.cfg.SMTPUser))
	}
	if s.cfg.SMTPPassword != "" {
		opts = append(opts, mail.WithPassword(s.cfg.SMTPPassword))
	}

	switch s.cfg.SMTPTLSMode {
	case "tls":
		opts = append(opts, mail.WithSSL())
	case "none":
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	default: // "starttls" or unset
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	}

	return mail.NewClient(s.cfg.SMTPHost, opts...)
}
