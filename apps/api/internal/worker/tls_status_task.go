package worker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/proxy"
	"github.com/weiling79/belune/internal/store/generated"
)

// TLS status values, mirroring the CHECK on domains.tls_status.
const (
	TLSStatusUnknown  = "unknown"
	TLSStatusDisabled = "disabled"
	TLSStatusPending  = "pending"
	TLSStatusActive   = "active"
	TLSStatusExpiring = "expiring"
	TLSStatusExpired  = "expired"
	TLSStatusFailed   = "failed"
)

// tlsExpiryWarning is how far ahead of expiry a certificate is reported as
// `expiring`. Let's Encrypt certificates are 90 days and renew at 30 days left,
// so 14 days means renewal has already failed several times — this is a real
// problem, not routine churn.
const tlsExpiryWarning = 14 * 24 * time.Hour

// caddyInternalIssuer appears in the issuer CN of certificates Caddy mints from
// its own CA. Serving one means automatic HTTPS has not obtained a real
// certificate yet, so the domain is still pending rather than active.
const caddyInternalIssuer = "Caddy Local Authority"

// TLSProbeResult is what a single probe observed on the wire. It is derived
// purely from the handshake, so the status the user sees is what a browser
// would actually get — not what our config says should happen.
type TLSProbeResult struct {
	Status   string
	Issuer   string
	NotAfter time.Time
	Error    string
}

// deriveTLSStatus turns a probe observation into the status stored on the
// domain. Split out from the dialling so the state machine is testable without
// a live proxy.
//
// recordedErr is any error already known for the domain (a failed SetupTLS, an
// ACME failure lifted from Caddy's logs, a DNS mismatch). It is what separates
// "no certificate yet, still working on it" from "no certificate, and here is
// why" — the distinction the whole feature exists to make.
func deriveTLSStatus(sslMode string, leaf *x509.Certificate, hostname string, dialErr error, recordedErr string, now time.Time) TLSProbeResult {
	if sslMode == proxy.SSLModeOff {
		return TLSProbeResult{Status: TLSStatusDisabled}
	}

	// No certificate served: either issuance is still in flight, or it failed and
	// something already told us why.
	if dialErr != nil || leaf == nil {
		res := TLSProbeResult{Status: TLSStatusPending, Error: recordedErr}
		if recordedErr != "" {
			res.Status = TLSStatusFailed
		}
		return res
	}

	// A certificate is served, but not for this hostname — Caddy fell back to
	// another SNI's certificate, which a browser would reject.
	if !certMatchesHost(leaf, hostname) {
		return TLSProbeResult{
			Status: TLSStatusFailed,
			Error:  fmt.Sprintf("the certificate served for %s is valid only for %s", hostname, strings.Join(leaf.DNSNames, ", ")),
		}
	}

	res := TLSProbeResult{
		Issuer:   leaf.Issuer.CommonName,
		NotAfter: leaf.NotAfter,
		Error:    recordedErr,
	}

	// Caddy's own CA is a placeholder while ACME is still working (or failing).
	// Reporting it as active would tell the user HTTPS is fine when no browser
	// would trust it.
	if strings.Contains(leaf.Issuer.CommonName, caddyInternalIssuer) {
		res.Status = TLSStatusPending
		if recordedErr != "" {
			res.Status = TLSStatusFailed
		}
		return res
	}

	switch {
	case now.After(leaf.NotAfter):
		res.Status = TLSStatusExpired
	case leaf.NotAfter.Sub(now) < tlsExpiryWarning:
		res.Status = TLSStatusExpiring
	default:
		res.Status = TLSStatusActive
		// A live, valid certificate settles any older complaint.
		res.Error = ""
	}
	return res
}

// certMatchesHost reports whether the leaf covers the hostname, honouring
// wildcard SANs the same way a browser does.
func certMatchesHost(leaf *x509.Certificate, hostname string) bool {
	return leaf.VerifyHostname(hostname) == nil
}

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

	leaf, dialErr := h.probeTLS(ctx, d.Hostname)
	res := deriveTLSStatus(d.SslMode, leaf, d.Hostname, dialErr, recordedErr, time.Now())

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

// probeTLS completes a TLS handshake against the local proxy with the domain's
// hostname as SNI, and returns the leaf it presented. The certificate is
// inspected, never trusted: it may legitimately be self-signed or expired, which
// is exactly what we are trying to detect.
func (h *TaskHandler) probeTLS(ctx context.Context, hostname string) (*x509.Certificate, error) {
	addr := h.Config.CaddyTLSProbeAddr
	if addr == "" {
		return nil, errors.New("no TLS probe address configured")
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName:         hostname,
		InsecureSkipVerify: true, //nolint:gosec // we inspect the leaf; trusting it would defeat the check
	})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, errors.New("no certificate presented")
	}
	return state.PeerCertificates[0], nil
}

// notifyTLSTransition tells admins when a domain enters a state that needs
// human action. Only transitions notify, so a domain that stays broken does not
// re-notify every minute.
func (h *TaskHandler) notifyTLSTransition(ctx context.Context, hostname string, res TLSProbeResult) {
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
