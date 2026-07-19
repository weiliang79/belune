package notify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpTimeout bounds a single provider delivery attempt. Retries are handled by
// the asynq worker, so a per-attempt timeout is enough here.
const httpTimeout = 15 * time.Second

// Mailer is the subset of the email service the email provider needs. Declaring
// it here keeps notify decoupled from the concrete service and trivially fakeable
// in tests.
type Mailer interface {
	// Send delivers via the instance-default SMTP transport.
	Send(ctx context.Context, msg MailMessage) error
	// SendWithConfig delivers via a channel's own SMTP server (Option B override).
	SendWithConfig(ctx context.Context, smtp MailSMTP, msg MailMessage) error
	// Configured reports whether the instance-default SMTP has a host set, so the
	// email provider can fail loudly instead of silently dropping into log-only
	// mode when a channel relies on the default but none is configured.
	Configured(ctx context.Context) bool
}

// MailMessage is the provider-level view of an outbound email, mapped onto the
// email service's Message by the wiring layer.
type MailMessage struct {
	To       string
	Subject  string
	TextBody string
}

// MailSMTP is a channel-specific mail-server override, mapped onto the email
// service's SMTPConfig by the wiring layer.
type MailSMTP struct {
	Host      string
	Port      int
	User      string
	Password  string
	FromEmail string
	FromName  string
	TLSMode   string
}

// Registry resolves a channel type to its Provider. Construct it once at startup
// and share it across the dispatcher, the test-send handler and the worker.
type Registry struct {
	providers map[string]Provider
	client    *http.Client
}

// NewRegistry builds the standard provider set. mailer may be nil, in which case
// the email provider validates config but returns an error on Send.
func NewRegistry(mailer Mailer) *Registry {
	client := &http.Client{Timeout: httpTimeout}
	return &Registry{
		client: client,
		providers: map[string]Provider{
			"discord":  discordProvider{client: client},
			"slack":    slackProvider{client: client},
			"telegram": telegramProvider{client: client, apiBase: telegramAPIBase},
			"webhook":  webhookProvider{client: client},
			"ntfy":     ntfyProvider{client: client},
			"gotify":   gotifyProvider{client: client},
			"email":    emailProvider{mailer: mailer},
		},
	}
}

// Provider returns the adapter for a channel type, or (nil, false) if unknown.
func (r *Registry) Provider(channelType string) (Provider, bool) {
	p, ok := r.providers[channelType]
	return p, ok
}

// postJSON POSTs body to url with the given headers and treats any non-2xx
// status as an error, including a short response snippet for diagnosis.
func postJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return do(client, req)
}

// postBytes POSTs a raw (non-JSON) body, used by ntfy where the message is the
// request body and metadata rides in headers.
func postBytes(ctx context.Context, client *http.Client, url string, headers map[string]string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return do(client, req)
}

func do(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	msg := string(bytes.TrimSpace(snippet))
	if msg == "" {
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	return fmt.Errorf("provider returned status %d: %s", resp.StatusCode, msg)
}
