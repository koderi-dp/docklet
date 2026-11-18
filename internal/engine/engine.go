// Package engine owns Docker Engine connectivity. Transport code (npipe, unix,
// tcp/tls, ssh, wsl) lives here and yields a standard Docker SDK client; resource
// services (internal/services) only ever consume the Provider interface and never
// care which transport is underneath.
package engine

import (
	"context"

	"github.com/moby/moby/client"

	"docklet/internal/model"
)

// Provider hands out the live SDK client for a connection ID. Implementations
// MUST return an error (never a client for a different connection) when the
// connection is unknown or not connected. Services call it per operation so
// that a connection switch can never leak a stale client.
type Provider interface {
	Client(ctx context.Context, connectionID string) (*client.Client, error)
}

// Emitter publishes events to the frontend (wraps Wails app.Event.Emit in main).
type Emitter interface {
	Emit(name string, data any)
}

// ProfileSource resolves the saved (or discovered) profile for a connection ID.
// Compose execution needs it to decide where the CLI runs.
type ProfileSource interface {
	Profile(connectionID string) (model.ConnectionProfile, error)
}
