package runtime

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// Runtimes hands out the ContainerRuntime that speaks to a particular server.
//
// Everything that touches containers used to hold one injected
// ContainerRuntime — the control plane's own Docker socket — which quietly
// encoded the assumption that there is exactly one host. Resources already
// carry a placement (projects.server_id), so the resolver makes each operation
// say *which* host it means at the point it acts.
//
// The serverID is passed explicitly rather than carried on the context on
// purpose. A context-carried value that a call site forgets to set does not
// fail: it falls back to a default and silently acts on the wrong host, a bug
// neither code review nor a single-server test suite can see. An argument the
// compiler demands cannot be forgotten.
type Runtimes interface {
	// For returns the runtime for the server a resource is placed on.
	For(ctx context.Context, serverID pgtype.UUID) (ContainerRuntime, error)

	// Local returns the runtime for the control plane's own host. It is for
	// operations that are genuinely about this machine — host metrics, the
	// host shell, the platform's own containers — not for resources whose
	// placement simply is not to hand.
	Local(ctx context.Context) (ContainerRuntime, error)
}

// localRuntimes resolves every server to the control plane's own runtime.
//
// This is the whole single-server implementation, and it is deliberately not
// an optimisation of a more general one: until the agent exists there is one
// host, and every serverID that can reach here is the local server's. When
// multi-server lands, the remote transport is added here and nothing above
// this file changes.
type localRuntimes struct {
	local ContainerRuntime
}

// NewLocalRuntimes returns a resolver backed by a single local runtime.
func NewLocalRuntimes(local ContainerRuntime) Runtimes {
	return &localRuntimes{local: local}
}

func (r *localRuntimes) For(_ context.Context, _ pgtype.UUID) (ContainerRuntime, error) {
	return r.local, nil
}

func (r *localRuntimes) Local(_ context.Context) (ContainerRuntime, error) {
	return r.local, nil
}
