// Package notify delivers domain events to third-party notification providers
// (Discord, Telegram, Slack, generic webhook, ntfy, Gotify, email). It is a pure
// delivery layer: the events it carries are the ones that already fire through
// the in-app notification pipeline — this package only routes them outward.
//
// Each provider is a small stateless adapter implementing Provider. A Registry
// maps a channel type to its adapter. Provider configuration is passed in as
// decrypted JSON (json.RawMessage) on every call; the encryption, storage and
// masking of that config live in the service layer.
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrPermanent marks a delivery failure that cannot succeed on retry — a
// misconfigured or corrupt channel (unknown type, undecryptable config, no SMTP)
// rather than a transient network error. The worker wraps these with
// asynq.SkipRetry so the error is stamped immediately instead of after the full
// backoff schedule.
var ErrPermanent = errors.New("notify: permanent delivery failure")

// Severity is derived from an event type and drives per-provider styling
// (Discord embed colour, ntfy priority, emoji prefixes...).
type Severity string

const (
	SeverityOK    Severity = "ok"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

// Event is a single notification to deliver. It mirrors the in-app notification
// row plus an absolute Link and the time the underlying event occurred.
type Event struct {
	// Type is the canonical event type, e.g. "deployment.failed". See events.go.
	Type string `json:"type"`
	// Title and Body are the human-readable summary already used by the in-app bell.
	Title string `json:"title"`
	Body  string `json:"body"`
	// Link is an absolute URL into the dashboard, or "" when PUBLIC_BASE_URL is unset.
	Link string `json:"link"`
	// OccurredAt is when the underlying event happened (not when it was delivered).
	OccurredAt time.Time `json:"occurred_at"`
}

// Severity classifies an event from the action half of its type. The suffix
// convention is fixed by the event registry: "...failed" / "...expired" are
// errors, "...expiring" is a warning, everything else is informational.
func (e Event) Severity() Severity {
	_, action, found := strings.Cut(e.Type, ".")
	if !found {
		action = e.Type
	}
	switch {
	case strings.Contains(action, "failed"), strings.HasSuffix(action, "expired"):
		return SeverityError
	case strings.HasSuffix(action, "expiring"):
		return SeverityWarn
	default:
		return SeverityOK
	}
}

// Provider is a notification transport. Implementations are stateless with
// respect to a single channel: the decrypted config JSON is supplied on every
// call, so one Provider value serves every channel of its type.
type Provider interface {
	// ValidateConfig reports whether raw carries the fields this provider needs.
	// Called before persisting a channel and before a test send.
	ValidateConfig(raw json.RawMessage) error
	// Send delivers ev using the decrypted config JSON. A non-nil error is
	// surfaced verbatim to the operator (test send) or stamped as last_error.
	Send(ctx context.Context, raw json.RawMessage, ev Event) error
}

