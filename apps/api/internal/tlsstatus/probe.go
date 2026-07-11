package tlsstatus

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// TLS status values, mirroring the CHECK on domains.tls_status.
const (
	StatusUnknown  = "unknown"
	StatusDisabled = "disabled"
	StatusPending  = "pending"
	StatusActive   = "active"
	StatusExpiring = "expiring"
	StatusExpired  = "expired"
	StatusFailed   = "failed"
)

// SSLModeOff mirrors proxy.SSLModeOff. Duplicated rather than imported to keep
// this package free of a dependency on the proxy.
const SSLModeOff = "off"

// ExpiryWarning is how far ahead of expiry a certificate is reported as
// `expiring`. Let's Encrypt certificates are 90 days and renew at 30 days left,
// so 14 days means renewal has already failed several times — a real problem,
// not routine churn.
const ExpiryWarning = 14 * 24 * time.Hour

// caddyInternalIssuer appears in the issuer CN of certificates Caddy mints from
// its own CA. Serving one means automatic HTTPS has not obtained a real
// certificate yet, so the domain is still pending rather than active.
const caddyInternalIssuer = "Caddy Local Authority"

// ProbeResult is what a single probe observed on the wire. It is derived purely
// from the handshake, so the status the user sees is what a browser would
// actually get — not what our configuration says should happen.
type ProbeResult struct {
	Status   string
	Issuer   string
	NotAfter time.Time
	Error    string
}

// Probe completes a TLS handshake against addr with hostname as SNI and returns
// the leaf certificate presented. The certificate is inspected, never trusted: it
// may legitimately be self-signed, expired, or for the wrong host — which is
// exactly what we are trying to detect.
func Probe(ctx context.Context, addr, hostname string) (*x509.Certificate, error) {
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

// Derive turns a probe observation into the status stored on the domain. Split
// out from the dialling so the state machine is testable without a live proxy.
//
// recordedErr is any error already known for the domain (a failed SetupTLS, an
// ACME failure lifted from Caddy's logs, a DNS mismatch). It is what separates
// "no certificate yet, still working on it" from "no certificate, and here is
// why" — the distinction the whole feature exists to make.
func Derive(sslMode string, leaf *x509.Certificate, hostname string, dialErr error, recordedErr string, now time.Time) ProbeResult {
	if sslMode == SSLModeOff {
		return ProbeResult{Status: StatusDisabled}
	}

	// No certificate served: either issuance is still in flight, or it failed and
	// something already told us why.
	if dialErr != nil || leaf == nil {
		res := ProbeResult{Status: StatusPending, Error: recordedErr}
		if recordedErr != "" {
			res.Status = StatusFailed
		}
		return res
	}

	// A certificate is served, but not for this hostname — the proxy fell back to
	// another SNI's certificate, which a browser would reject.
	if leaf.VerifyHostname(hostname) != nil {
		return ProbeResult{
			Status: StatusFailed,
			Error:  fmt.Sprintf("the certificate served for %s is valid only for %s", hostname, strings.Join(leaf.DNSNames, ", ")),
		}
	}

	res := ProbeResult{
		Issuer:   leaf.Issuer.CommonName,
		NotAfter: leaf.NotAfter,
		Error:    recordedErr,
	}

	// Caddy's own CA is a placeholder while ACME is still working (or failing).
	// Reporting it as active would tell the user HTTPS is fine when no browser
	// would trust it.
	if strings.Contains(leaf.Issuer.CommonName, caddyInternalIssuer) {
		res.Status = StatusPending
		if recordedErr != "" {
			res.Status = StatusFailed
		}
		return res
	}

	switch {
	case now.After(leaf.NotAfter):
		res.Status = StatusExpired
	case leaf.NotAfter.Sub(now) < ExpiryWarning:
		res.Status = StatusExpiring
	default:
		res.Status = StatusActive
		// A live, valid certificate settles any older complaint.
		res.Error = ""
	}
	return res
}
