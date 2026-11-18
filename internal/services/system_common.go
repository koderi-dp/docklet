package services

import (
	"context"
	"fmt"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
)

// engineClient resolves the live SDK client for a connection.
func engineClient(ctx context.Context, p engine.Provider, connectionID string) (*client.Client, error) {
	if p == nil {
		return nil, fmt.Errorf("no engine provider configured")
	}
	cli, err := p.Client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if cli == nil {
		return nil, fmt.Errorf("connection %q is not available", connectionID)
	}
	return cli, nil
}

func emit(e engine.Emitter, name string, payload any) {
	if e != nil {
		e.Emit(name, payload)
	}
}

// formatBytesIEC renders a byte count for logs ("1.5 GB").
func formatBytesIEC(n uint64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n) / 1024
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// pruneOutcome is what one Engine prune call reported.
type pruneOutcome struct {
	Noun      string   // plural, e.g. "containers"
	Items     []string // names/ids the Engine says were deleted
	Reclaimed uint64
	HasSpace  bool // false for kinds whose report carries no byte figure (networks)
}

type pruneStep func(ctx context.Context, cli *client.Client) (pruneOutcome, error)

type pruneSpec struct {
	Kind, Title, Resource string
	Changed               []string // events to emit once finished (success or not)
}

// startPrune runs steps sequentially as one background operation. Each step's real
// Engine report is logged; a failing step aborts the rest (earlier results stay logged).
// The client is resolved up front so an unknown connection fails synchronously.
func startPrune(ctx context.Context, p engine.Provider, m *ops.Manager, em engine.Emitter, connectionID string, sp pruneSpec, steps ...pruneStep) (model.Operation, error) {
	if m == nil {
		return model.Operation{}, fmt.Errorf("operation manager is not available")
	}
	cli, err := engineClient(ctx, p, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	op := m.Start(ops.Spec{Kind: sp.Kind, Title: sp.Title, ConnectionID: connectionID, Resource: sp.Resource},
		func(opCtx context.Context, h *ops.Handle) error {
			defer emitChanged(em, connectionID, sp.Changed)
			var totalItems int
			var totalBytes uint64
			var anySpace bool
			for _, step := range steps {
				out, err := step(opCtx, cli)
				if err != nil {
					return friendlyError(err)
				}
				for _, it := range out.Items {
					h.Log("Removed " + singular(out.Noun) + ": " + it)
				}
				line := "No unused " + out.Noun + " found"
				if len(out.Items) > 0 {
					line = "Removed " + plural(len(out.Items), singular(out.Noun), out.Noun)
				}
				if out.HasSpace {
					line += ", reclaimed " + formatBytesIEC(out.Reclaimed)
					anySpace = true
					totalBytes += out.Reclaimed
				}
				h.Log(line)
				totalItems += len(out.Items)
			}
			sum := "Removed " + plural(totalItems, "item", "items")
			if anySpace {
				sum += ", reclaimed " + formatBytesIEC(totalBytes)
			}
			h.Detail(sum)
			return nil
		})
	return op, nil
}

func singular(noun string) string {
	switch {
	case strings.HasSuffix(noun, "ies"):
		return strings.TrimSuffix(noun, "ies") + "y"
	case strings.HasSuffix(noun, "s"):
		return strings.TrimSuffix(noun, "s")
	}
	return noun
}

func emitChanged(em engine.Emitter, connectionID string, names []string) {
	for _, n := range names {
		switch n {
		case model.EventVolumesChanged:
			emit(em, n, model.VolumesChanged{ConnectionID: connectionID})
		case model.EventNetworksChanged:
			emit(em, n, model.NetworksChanged{ConnectionID: connectionID})
		case model.EventContainersChanged:
			emit(em, n, model.ContainersChanged{ConnectionID: connectionID})
		}
	}
}

func isNotFound(err error) bool {
	return cerrdefs.IsNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "no such")
}

func isConflict(err error) bool {
	low := strings.ToLower(err.Error())
	return cerrdefs.IsConflict(err) || strings.Contains(low, "in use") || strings.Contains(low, "active endpoints")
}

func userNames(users []model.ResourceUser) string {
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.Name)
	}
	return strings.Join(names, ", ")
}

// containerName strips the leading slash of the first Engine name.
func containerName(names []string, id string) string {
	if len(names) > 0 {
		return strings.TrimPrefix(names[0], "/")
	}
	return shortID(id)
}
