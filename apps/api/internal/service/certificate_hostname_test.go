package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCertificateHostnameCandidates pins RFC 6125 wildcard matching, which is
// the whole reason candidates are generated in Go rather than pattern-matched in
// SQL. A wildcard matches exactly ONE label and only the leftmost one, and the
// cases that get this wrong in the wild are the multi-label host and the bare
// parent — both of which a LIKE '%.example.com' would happily accept.
func TestCertificateHostnameCandidates(t *testing.T) {
	cases := []struct {
		name     string
		hostname string
		want     []string
	}{
		{
			name:     "one label deep gets its wildcard parent",
			hostname: "app.example.com",
			want:     []string{"app.example.com", "*.example.com"},
		},
		{
			// The trap. "*.example.com" must NOT be offered here: it covers one
			// label, and this host is two deep. Only "*.eu.example.com" would.
			name:     "two labels deep wildcards only its immediate parent",
			hostname: "a.b.example.com",
			want:     []string{"a.b.example.com", "*.b.example.com"},
		},
		{
			// The other trap: a wildcard never covers the bare domain, so
			// "example.com" must not pick up "*.example.com".
			name:     "apex does not match its own wildcard",
			hostname: "example.com",
			want:     []string{"example.com", "*.com"},
		},
		{
			name:     "single label has no parent to wildcard",
			hostname: "localhost",
			want:     []string{"localhost"},
		},
		{
			name:     "case and the root dot are normalised away",
			hostname: "  APP.Example.COM.  ",
			want:     []string{"app.example.com", "*.example.com"},
		},
		{
			name:     "empty yields nothing rather than a match-all",
			hostname: "   ",
			want:     nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, certificateHostnameCandidates(tc.hostname))
		})
	}
}

// TestCertificateHostnameCandidates_NeverMatchesAcrossLabels is the property the
// table above encodes, stated once as an invariant: the wildcard a hostname
// generates must have the same label COUNT as the hostname, or it would match
// hosts at a different depth than the certificate actually covers.
func TestCertificateHostnameCandidates_NeverMatchesAcrossLabels(t *testing.T) {
	for _, host := range []string{"a.example.com", "a.b.example.com", "a.b.c.example.com"} {
		got := certificateHostnameCandidates(host)
		if assert.Len(t, got, 2, "host %q should yield exact + wildcard", host) {
			assert.Equal(t, labelCount(host), labelCount(got[1]),
				"wildcard %q must sit at the same depth as %q", got[1], host)
		}
	}
}

func labelCount(s string) int {
	n := 1
	for _, c := range s {
		if c == '.' {
			n++
		}
	}
	return n
}
