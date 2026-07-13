package tlsstatus

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// leafFor builds the parts of a certificate the status derivation actually
// reads, without the cost of minting a real one.
func leafFor(issuerCN string, dnsNames []string, notAfter time.Time) *x509.Certificate {
	return &x509.Certificate{
		Issuer:   pkix.Name{CommonName: issuerCN},
		DNSNames: dnsNames,
		NotAfter: notAfter,
	}
}

func TestDerive(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	host := "app.example.com"
	dialRefused := errors.New("connection refused")

	tests := []struct {
		name        string
		sslMode     string
		leaf        *x509.Certificate
		dialErr     error
		recordedErr string
		wantStatus  string
		wantErr     string
	}{
		{
			name:       "off means disabled, not broken",
			sslMode:    SSLModeOff,
			dialErr:    dialRefused,
			wantStatus: StatusDisabled,
		},
		{
			name:       "no certificate yet and no known reason is still pending",
			sslMode:    "automatic",
			dialErr:    dialRefused,
			wantStatus: StatusPending,
		},
		{
			// The distinction the whole feature exists for: a stuck domain with a
			// known cause is failed, not an eternal "pending".
			name:        "no certificate with a known reason is failed",
			sslMode:     "automatic",
			dialErr:     dialRefused,
			recordedErr: "no A record for app.example.com",
			wantStatus:  StatusFailed,
			wantErr:     "no A record for app.example.com",
		},
		{
			name:       "caddy internal CA means ACME has not succeeded yet",
			sslMode:    "automatic",
			leaf:       leafFor("Caddy Local Authority - ECC Intermediate", []string{host}, now.Add(90*24*time.Hour)),
			wantStatus: StatusPending,
		},
		{
			name:        "caddy internal CA plus a known ACME error is failed",
			sslMode:     "automatic",
			leaf:        leafFor("Caddy Local Authority - ECC Intermediate", []string{host}, now.Add(90*24*time.Hour)),
			recordedErr: "acme: rate limit exceeded",
			wantStatus:  StatusFailed,
			wantErr:     "acme: rate limit exceeded",
		},
		{
			name:       "valid certificate is active",
			sslMode:    "automatic",
			leaf:       leafFor("R3", []string{host}, now.Add(60*24*time.Hour)),
			wantStatus: StatusActive,
		},
		{
			// A working certificate clears a stale complaint — otherwise the badge
			// would show an error for a domain that is demonstrably fine.
			name:        "a live certificate clears an old error",
			sslMode:     "automatic",
			leaf:        leafFor("R3", []string{host}, now.Add(60*24*time.Hour)),
			recordedErr: "acme: rate limit exceeded",
			wantStatus:  StatusActive,
			wantErr:     "",
		},
		{
			name:       "within the warning window is expiring",
			sslMode:    "automatic",
			leaf:       leafFor("R3", []string{host}, now.Add(3*24*time.Hour)),
			wantStatus: StatusExpiring,
		},
		{
			name:       "past not_after is expired",
			sslMode:    "automatic",
			leaf:       leafFor("R3", []string{host}, now.Add(-time.Hour)),
			wantStatus: StatusExpired,
		},
		{
			// Caddy served some other SNI's certificate; a browser would reject it,
			// so reporting active would be a lie.
			name:       "certificate for a different host is failed",
			sslMode:    "custom",
			leaf:       leafFor("R3", []string{"other.example.com"}, now.Add(60*24*time.Hour)),
			wantStatus: StatusFailed,
			wantErr:    "valid only for other.example.com",
		},
		{
			name:       "wildcard SAN covers the host",
			sslMode:    "custom",
			leaf:       leafFor("R3", []string{"*.example.com"}, now.Add(60*24*time.Hour)),
			wantStatus: StatusActive,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Derive(tc.sslMode, tc.leaf, host, tc.dialErr, tc.recordedErr, now, false)
			assert.Equal(t, tc.wantStatus, got.Status)
			if tc.wantErr != "" {
				assert.Contains(t, got.Error, tc.wantErr)
			} else {
				assert.Empty(t, got.Error)
			}
		})
	}
}

// A CommonName is optional in a distinguished name, and Cloudflare's Origin CA
// omits it — the issuer DN carries only C/O/OU/L/ST. Reading CommonName alone
// left custom-certificate domains with a blank issuer in the UI while ACME ones
// (Let's Encrypt sets a CN) were populated. The DNs here are the real ones, off
// a live Origin CA certificate and a live Let's Encrypt certificate.
func TestDeriveIssuerName(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	host := "whoami-custom.brandoncaryn.com"
	expiry := now.Add(365 * 24 * time.Hour)

	tests := []struct {
		name       string
		issuer     pkix.Name
		wantIssuer string
	}{
		{
			name: "cloudflare origin CA has no issuer CN, falls back to the organization",
			issuer: pkix.Name{
				Country:            []string{"US"},
				Organization:       []string{"CloudFlare, Inc."},
				OrganizationalUnit: []string{"CloudFlare Origin SSL Certificate Authority"},
				Locality:           []string{"San Francisco"},
				Province:           []string{"California"},
			},
			wantIssuer: "CloudFlare, Inc.",
		},
		{
			name: "a CN is preferred when present",
			issuer: pkix.Name{
				Country:      []string{"US"},
				Organization: []string{"Let's Encrypt"},
				CommonName:   "YE2",
			},
			wantIssuer: "YE2",
		},
		{
			name:       "neither CN nor organization yields an empty issuer, not a panic",
			issuer:     pkix.Name{Country: []string{"US"}},
			wantIssuer: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leaf := &x509.Certificate{
				Issuer:   tt.issuer,
				DNSNames: []string{host},
				NotAfter: expiry,
			}
			res := Derive("custom", leaf, host, nil, "", now, false)
			assert.Equal(t, StatusActive, res.Status)
			assert.Equal(t, tt.wantIssuer, res.Issuer)
		})
	}
}

// The same certificate means two opposite things depending on how the proxy is
// configured, and the certificate itself cannot tell them apart. This is the
// whole reason "local" is decided from Caddy's configured issuer rather than
// from the issuer name on the wire — a production Caddy whose ACME issuance has
// failed serves exactly this certificate, and reporting that as a healthy local
// setup would be the lie the pipeline exists to prevent.
func TestDeriveInternalCertificate(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	host := "app.belune.local"
	internal := leafFor("Caddy Local Authority - ECC Intermediate", []string{host}, now.Add(12*time.Hour))

	t.Run("proxy issues internally: this is the finished state", func(t *testing.T) {
		res := Derive("automatic", internal, host, nil, "", now, true)
		assert.Equal(t, StatusLocal, res.Status)
	})

	t.Run("a public CA was expected: still pending, not local", func(t *testing.T) {
		res := Derive("automatic", internal, host, nil, "", now, false)
		assert.Equal(t, StatusPending, res.Status)
	})

	t.Run("a public CA was expected and failed: failed, not local", func(t *testing.T) {
		res := Derive("automatic", internal, host, nil, "DNS problem: NXDOMAIN", now, false)
		assert.Equal(t, StatusFailed, res.Status)
		assert.Equal(t, "DNS problem: NXDOMAIN", res.Error)
	})

	t.Run("a real certificate is active regardless of the proxy's issuer setting", func(t *testing.T) {
		real := leafFor("YE2", []string{host}, now.Add(80*24*time.Hour))
		assert.Equal(t, StatusActive, Derive("automatic", real, host, nil, "", now, true).Status)
		assert.Equal(t, StatusActive, Derive("automatic", real, host, nil, "", now, false).Status)
	})
}
