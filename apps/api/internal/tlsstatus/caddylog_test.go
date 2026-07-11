package tlsstatus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The first two samples are real lines captured from a Caddy instance failing to
// issue against Let's Encrypt staging — not invented shapes. The rest are the
// failure modes users actually hit (rate limit, CAA, DNS) in the same format.
func TestParseCaddyTLSError(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantHost string
		wantIn   string
		wantOK   bool
	}{
		{
			name:     "captured: could not get certificate from issuer",
			line:     `{"level":"error","ts":1783771336.8427126,"logger":"tls.obtain","msg":"could not get certificate from issuer","identifier":"belune-phase3-nxdomain.example.com","issuer":"acme-staging-v02.api.letsencrypt.org-directory","error":"HTTP 400 urn:ietf:params:acme:error:invalidContact - Error validating contact(s) :: contact email has forbidden domain \"example.com\""}`,
			wantHost: "belune-phase3-nxdomain.example.com",
			wantIn:   "urn:ietf:params:acme:error:invalidContact",
			wantOK:   true,
		},
		{
			// No `identifier` field — the hostname is only bracketed in the error.
			name:     "captured: will retry",
			line:     `{"level":"error","ts":1783771336.8429382,"logger":"tls.obtain","msg":"will retry","error":"[belune-phase3-nxdomain.example.com] Obtain: registering account [mailto:test@example.com] with server: attempt 1: HTTP 400 urn:ietf:params:acme:error:invalidContact","attempt":1,"retrying_in":60}`,
			wantHost: "belune-phase3-nxdomain.example.com",
			wantIn:   "Obtain: registering account",
			wantOK:   true,
		},
		{
			name:     "rate limit",
			line:     `{"level":"error","logger":"tls.obtain","msg":"could not get certificate from issuer","identifier":"app.example.com","error":"HTTP 429 urn:ietf:params:acme:error:rateLimited - too many certificates already issued for exact set of domains"}`,
			wantHost: "app.example.com",
			wantIn:   "rateLimited",
			wantOK:   true,
		},
		{
			name:     "CAA forbids the issuer",
			line:     `{"level":"error","logger":"tls.obtain","msg":"could not get certificate from issuer","identifier":"app.example.com","error":"HTTP 403 urn:ietf:params:acme:error:caa - CAA record for app.example.com prevents issuance"}`,
			wantHost: "app.example.com",
			wantIn:   "CAA record",
			wantOK:   true,
		},
		{
			name:     "DNS / connection failure during the challenge",
			line:     `{"level":"error","logger":"tls.obtain","msg":"will retry","error":"[app.example.com] Obtain: [app.example.com] solving challenge: app.example.com: [app.example.com] error checking authorization: DNS problem: NXDOMAIN looking up A for app.example.com","attempt":2}`,
			wantHost: "app.example.com",
			wantIn:   "NXDOMAIN",
			wantOK:   true,
		},
		// Everything below is the overwhelmingly common case: not a failure.
		{
			name:   "info-level issuance progress is not an error",
			line:   `{"level":"info","logger":"tls.obtain","msg":"obtaining certificate","identifier":"app.example.com"}`,
			wantOK: false,
		},
		{
			name:   "an error from another subsystem is not ours",
			line:   `{"level":"error","logger":"http.log.access","msg":"handled request","status":500}`,
			wantOK: false,
		},
		{
			name:   "an application's plain-text log line",
			line:   `2026-07-11 12:00:00 ERROR something failed in the user's app`,
			wantOK: false,
		},
		{
			name:   "malformed JSON",
			line:   `{"level":"error","logger":"tls.obtain"`,
			wantOK: false,
		},
		{
			name:   "empty line",
			line:   ``,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, reason, ok := ParseCaddyTLSError(tc.line)
			assert.Equal(t, tc.wantOK, ok)
			if !tc.wantOK {
				return
			}
			assert.Equal(t, tc.wantHost, host)
			assert.Contains(t, reason, tc.wantIn)
		})
	}
}
