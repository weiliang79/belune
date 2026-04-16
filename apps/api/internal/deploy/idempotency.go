// Package deploy holds shared helpers for deployment creation. Today this is
// just the idempotency key derivation used to dedupe duplicate webhook
// deliveries and rapid manual triggers.
package deploy

import (
	"crypto/sha256"
	"encoding/hex"
)

// Windows per trigger source. Webhook providers often retry delivery within a
// few seconds; manual clicks at worst arrive within a double-click; rollback
// buttons sit behind a confirm dialog so the window can be small.
const (
	WindowPushSeconds     = 60
	WindowManualSeconds   = 10
	WindowRollbackSeconds = 30
)

// Key derives a stable idempotency key from an application ID, trigger source,
// and a discriminator identifying *what* is being deployed. Callers choose
// the discriminator per trigger:
//
//   - push     → commit SHA
//   - rollback → target image tag
//   - manual   → "" (the time window alone bounds dedup)
//
// The key is opaque to the DB — just a fingerprint used for lookup.
func Key(applicationID, trigger, discriminator string) string {
	h := sha256.Sum256([]byte(applicationID + "|" + trigger + "|" + discriminator))
	return hex.EncodeToString(h[:])
}
