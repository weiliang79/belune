package worker

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/weiling79/belune/internal/proxy"
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

func TestDeriveTLSStatus(t *testing.T) {
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
			sslMode:    proxy.SSLModeOff,
			dialErr:    dialRefused,
			wantStatus: TLSStatusDisabled,
		},
		{
			name:       "no certificate yet and no known reason is still pending",
			sslMode:    "automatic",
			dialErr:    dialRefused,
			wantStatus: TLSStatusPending,
		},
		{
			// The distinction the whole feature exists for: a stuck domain with a
			// known cause is failed, not an eternal "pending".
			name:        "no certificate with a known reason is failed",
			sslMode:     "automatic",
			dialErr:     dialRefused,
			recordedErr: "no A record for app.example.com",
			wantStatus:  TLSStatusFailed,
			wantErr:     "no A record for app.example.com",
		},
		{
			name:       "caddy internal CA means ACME has not succeeded yet",
			sslMode:    "automatic",
			leaf:       leafFor("Caddy Local Authority - ECC Intermediate", []string{host}, now.Add(90*24*time.Hour)),
			wantStatus: TLSStatusPending,
		},
		{
			name:        "caddy internal CA plus a known ACME error is failed",
			sslMode:     "automatic",
			leaf:        leafFor("Caddy Local Authority - ECC Intermediate", []string{host}, now.Add(90*24*time.Hour)),
			recordedErr: "acme: rate limit exceeded",
			wantStatus:  TLSStatusFailed,
			wantErr:     "acme: rate limit exceeded",
		},
		{
			name:       "valid certificate is active",
			sslMode:    "automatic",
			leaf:       leafFor("R3", []string{host}, now.Add(60*24*time.Hour)),
			wantStatus: TLSStatusActive,
		},
		{
			// A working certificate clears a stale complaint — otherwise the badge
			// would show an error for a domain that is demonstrably fine.
			name:        "a live certificate clears an old error",
			sslMode:     "automatic",
			leaf:        leafFor("R3", []string{host}, now.Add(60*24*time.Hour)),
			recordedErr: "acme: rate limit exceeded",
			wantStatus:  TLSStatusActive,
			wantErr:     "",
		},
		{
			name:       "within the warning window is expiring",
			sslMode:    "automatic",
			leaf:       leafFor("R3", []string{host}, now.Add(3*24*time.Hour)),
			wantStatus: TLSStatusExpiring,
		},
		{
			name:       "past not_after is expired",
			sslMode:    "automatic",
			leaf:       leafFor("R3", []string{host}, now.Add(-time.Hour)),
			wantStatus: TLSStatusExpired,
		},
		{
			// Caddy served some other SNI's certificate; a browser would reject it,
			// so reporting active would be a lie.
			name:       "certificate for a different host is failed",
			sslMode:    "custom",
			leaf:       leafFor("R3", []string{"other.example.com"}, now.Add(60*24*time.Hour)),
			wantStatus: TLSStatusFailed,
			wantErr:    "valid only for other.example.com",
		},
		{
			name:       "wildcard SAN covers the host",
			sslMode:    "custom",
			leaf:       leafFor("R3", []string{"*.example.com"}, now.Add(60*24*time.Hour)),
			wantStatus: TLSStatusActive,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveTLSStatus(tc.sslMode, tc.leaf, host, tc.dialErr, tc.recordedErr, now)
			assert.Equal(t, tc.wantStatus, got.Status)
			if tc.wantErr != "" {
				assert.Contains(t, got.Error, tc.wantErr)
			} else {
				assert.Empty(t, got.Error)
			}
		})
	}
}

func TestIsPublicIP(t *testing.T) {
	// A private or loopback autodetect must disable the DNS precheck rather than
	// flag every domain as misconfigured.
	assert.False(t, isPublicIP(net.ParseIP("192.168.1.10")))
	assert.False(t, isPublicIP(net.ParseIP("10.0.0.5")))
	assert.False(t, isPublicIP(net.ParseIP("127.0.0.1")))
	assert.True(t, isPublicIP(net.ParseIP("203.0.113.7")))
}
