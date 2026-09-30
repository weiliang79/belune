package ws

import "context"

// ChannelAuthorizer decides whether a connection may subscribe to a channel.
//
// ctx is the handshake request's context. It stays alive for the socket's whole
// lifetime and carries what Auth resolved at connect (user, role, and for a PAT
// its scopes and project pin), so an implementation reads the caller from it
// exactly as a REST handler does, and Client needs no identity fields of its own.
//
// Identity is resolved once, at handshake. REST re-validates a token on every
// request, so the socket is the one place a revoked PAT, a demoted role or a
// removed share keeps working until the connection drops; a reconnect
// re-authorizes from scratch.
//
// Implementations must fail closed: anything they do not positively recognise
// and approve — an unknown channel prefix, a malformed id, a lookup error — is
// false. The hub is a dumb pub/sub with no notion of who may hear what, so this
// is the only thing standing between a subscriber and a channel's frames.
type ChannelAuthorizer interface {
	AuthorizeChannel(ctx context.Context, channel string) bool
}
