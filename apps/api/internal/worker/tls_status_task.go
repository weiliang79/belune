package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/proxy"
	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/tlsstatus"
)

// TLS status values. The state machine and the probe itself live in the
// tlsstatus package, shared with the handler that reports the dashboard's own
// certificate — there must be exactly one definition of what "active" means.
const (
	TLSStatusUnknown  = tlsstatus.StatusUnknown
	TLSStatusDisabled = tlsstatus.StatusDisabled
	TLSStatusPending  = tlsstatus.StatusPending
	TLSStatusActive   = tlsstatus.StatusActive
	TLSStatusExpiring = tlsstatus.StatusExpiring
	TLSStatusExpired  = tlsstatus.StatusExpired
	TLSStatusFailed   = tlsstatus.StatusFailed
)

// HandleTLSStatusSweep probes every domain and records what the proxy actually
// serves for it. Registered @every 1m; domain create/update also enqueues a
// one-shot so the badge moves within seconds rather than up to a minute.
func (h *TaskHandler) HandleTLSStatusSweep(ctx context.Context) {
	domains, err := h.Queries.ListDomainsForTLSProbe(ctx)
	if err != nil {
		slog.Warn("tls probe: failed to list domains", "error", err)
		return
	}

	for _, d := range domains {
		h.probeDomain(ctx, d)
	}
}

// probeDomain probes one domain and persists the result, notifying on a
// transition into a state the operator needs to act on.
func (h *TaskHandler) probeDomain(ctx context.Context, d generated.ListDomainsForTLSProbeRow) {
	recordedErr := d.TlsError.String

	// Check DNS before the handshake: a hostname pointing somewhere else can
	// never get a certificate, and saying so is far more useful than a generic
	// "pending" that never resolves.
	if d.SslMode != proxy.SSLModeOff {
		if mismatch := h.checkDNS(ctx, d.Hostname); mismatch != "" {
			recordedErr = mismatch
		}
	}

	leaf, dialErr := tlsstatus.Probe(ctx, h.Config.CaddyTLSProbeAddr, d.Hostname)
	res := tlsstatus.Derive(d.SslMode, leaf, d.Hostname, dialErr, recordedErr, time.Now())

	var notAfter pgtype.Timestamptz
	if !res.NotAfter.IsZero() {
		notAfter = pgtype.Timestamptz{Time: res.NotAfter, Valid: true}
	}
	if err := h.Queries.UpdateDomainTLSStatus(ctx, generated.UpdateDomainTLSStatusParams{
		ID:          d.ID,
		TlsStatus:   res.Status,
		TlsIssuer:   pgtype.Text{String: res.Issuer, Valid: res.Issuer != ""},
		TlsNotAfter: notAfter,
		TlsError:    pgtype.Text{String: res.Error, Valid: res.Error != ""},
	}); err != nil {
		slog.Warn("tls probe: failed to record status", "hostname", d.Hostname, "error", err)
		return
	}

	if res.Status != d.TlsStatus {
		slog.Info("tls status changed", "hostname", d.Hostname, "from", d.TlsStatus, "to", res.Status, "error", res.Error)
		h.notifyTLSTransition(ctx, d.Hostname, res)
	}
}

// notifyTLSTransition tells admins when a domain enters a state that needs
// human action. Only transitions notify, so a domain that stays broken does not
// re-notify every minute.
func (h *TaskHandler) notifyTLSTransition(ctx context.Context, hostname string, res tlsstatus.ProbeResult) {
	if h.Notifier == nil {
		return
	}
	var title, body string
	switch res.Status {
	case TLSStatusFailed:
		title = "TLS certificate failed"
		body = fmt.Sprintf("%s could not get a certificate: %s", hostname, res.Error)
	case TLSStatusExpired:
		title = "TLS certificate expired"
		body = fmt.Sprintf("The certificate for %s has expired. HTTPS is broken for this domain.", hostname)
	case TLSStatusExpiring:
		title = "TLS certificate expiring"
		body = fmt.Sprintf("The certificate for %s expires %s.", hostname, res.NotAfter.Format("2 Jan 2006"))
	default:
		return
	}

	admins, err := h.Queries.ListAdminUserIDs(ctx)
	if err != nil {
		slog.Warn("tls probe: failed to list admins for notification", "error", err)
		return
	}
	for _, id := range admins {
		h.Notifier.Notify(formatUUID(id), "tls."+res.Status, title, body, "/certificates")
	}
}

// TLSProbePayload targets a single domain for an immediate re-probe.
type TLSProbePayload struct {
	DomainID string `json:"domain_id"`
}

// HandleTLSProbeTask re-probes one domain now: enqueued when a domain is created
// or updated, and by the UI's "Recheck" action.
func (h *TaskHandler) HandleTLSProbeTask(ctx context.Context, payload []byte) error {
	var p TLSProbePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal tls probe payload: %w", err)
	}
	var id pgtype.UUID
	if err := id.Scan(p.DomainID); err != nil {
		return fmt.Errorf("invalid domain id: %w", err)
	}

	domains, err := h.Queries.ListDomainsForTLSProbe(ctx)
	if err != nil {
		return err
	}
	for _, d := range domains {
		if d.ID == id {
			h.probeDomain(ctx, d)
			return nil
		}
	}
	return nil
}
