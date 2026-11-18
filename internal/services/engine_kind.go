package services

import (
	"context"
	"sync"

	"github.com/moby/moby/client"

	"docklet/internal/engine"
	"docklet/internal/model"
)

// kindCache remembers what each live client answered: one /version call per connection, not per
// operation. A reconnect builds a new client, so a stale entry is never used for it.
var kindCache sync.Map // *client.Client -> model.EngineKind

// engineKindOf says whether the engine behind cli is Podman or Docker, from its own /version
// answer. Unknown or unreachable stays Docker.
func engineKindOf(ctx context.Context, cli *client.Client) model.EngineKind {
	if v, ok := kindCache.Load(cli); ok {
		return v.(model.EngineKind)
	}
	v, err := cli.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		return model.EngineDocker
	}
	k := engine.KindFromVersion(v.Platform.Name, v.Components)
	kindCache.Store(cli, k)
	return k
}

// engineLabel is the engine's product name for messages.
func engineLabel(k model.EngineKind) string {
	if k == model.EnginePodman {
		return "Podman"
	}
	return "Docker"
}
