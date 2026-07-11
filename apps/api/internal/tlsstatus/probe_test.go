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
			got := Derive(tc.sslMode, tc.leaf, host, tc.dialErr, tc.recordedErr, now)
			assert.Equal(t, tc.wantStatus, got.Status)
			if tc.wantErr != "" {
				assert.Contains(t, got.Error, tc.wantErr)
			} else {
				assert.Empty(t, got.Error)
			}
		})
	}
}
