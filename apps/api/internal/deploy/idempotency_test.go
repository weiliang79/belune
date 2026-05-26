package deploy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/deploy"
)

func TestKey_Deterministic(t *testing.T) {
	a := deploy.Key("app-1", "push", "abc123")
	b := deploy.Key("app-1", "push", "abc123")
	assert.Equal(t, a, b, "same inputs must produce the same key")
}

func TestKey_DifferentApplicationIDs(t *testing.T) {
	a := deploy.Key("app-1", "push", "abc123")
	b := deploy.Key("app-2", "push", "abc123")
	assert.NotEqual(t, a, b, "different application IDs must produce different keys")
}

func TestKey_DifferentTriggers(t *testing.T) {
	a := deploy.Key("app-1", "push", "abc123")
	b := deploy.Key("app-1", "manual", "abc123")
	assert.NotEqual(t, a, b, "different triggers must produce different keys")
}

func TestKey_DifferentDiscriminators(t *testing.T) {
	a := deploy.Key("app-1", "push", "abc123")
	b := deploy.Key("app-1", "push", "def456")
	assert.NotEqual(t, a, b, "different discriminators must produce different keys")
}

func TestKey_EmptyDiscriminator(t *testing.T) {
	// manual trigger uses an empty discriminator — key must still be stable
	a := deploy.Key("app-1", "manual", "")
	b := deploy.Key("app-1", "manual", "")
	assert.Equal(t, a, b, "empty discriminator must produce a stable key")
}

func TestKey_Format(t *testing.T) {
	k := deploy.Key("app-1", "push", "abc123")
	// SHA-256 hex digest → always 64 lowercase hex characters.
	require.Len(t, k, 64, "key must be 64 hex characters")
	for _, ch := range k {
		assert.True(t, (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f'),
			"key must contain only lowercase hex characters, got %c", ch)
	}
}
