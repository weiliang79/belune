package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A host split across paths appears once per path in the reconciler's expected
// set. The auto-HTTPS lists are about certificates, and a certificate belongs to
// a name — Caddy selects it by SNI, which knows nothing about paths — so the
// hostname must reach Caddy exactly once however many paths it serves.
func TestHostSet_DeduplicatesPathsOfOneHost(t *testing.T) {
	s := newHostSet()
	s.add("shop.com") // "/"
	s.add("shop.com") // "/api"
	s.add("shop.com") // "/api/v2"
	s.add("other.com")

	assert.Equal(t, []string{"other.com", "shop.com"}, s.sorted())
}

// These lists are written into Caddy's config and then compared against what is
// there. An order that varied between passes would read as drift for ever and
// rewrite the server config every minute.
func TestHostSet_OrderIsStable(t *testing.T) {
	build := func() []string {
		s := newHostSet()
		for _, h := range []string{"z.com", "a.com", "m.com", "a.com"} {
			s.add(h)
		}
		return s.sorted()
	}

	want := []string{"a.com", "m.com", "z.com"}
	for i := 0; i < 10; i++ {
		assert.Equal(t, want, build(), "map iteration must not leak into the output")
	}
}

// nil, not an empty slice: the callers marshal this straight into Caddy's config,
// where an empty list and an absent one are not the same statement.
func TestHostSet_EmptyIsNil(t *testing.T) {
	assert.Nil(t, newHostSet().sorted())
}
