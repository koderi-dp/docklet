package services

import (
	"context"
	"sync"

	"github.com/moby/moby/client"

	"docklet/internal/model"
)

// Podman's container list carries no health information (Docker puts it in the summary), so on a
// Podman engine the running containers are inspected. A container without a healthcheck is
// remembered and not inspected again; one with a healthcheck is re-read on every list, which the
// health_status events already trigger.
var (
	healthcheckMu    sync.Mutex
	healthcheckKnown = map[*client.Client]map[string]bool{} // client -> container ID -> has a healthcheck
)

func fillPodmanHealth(ctx context.Context, cli *client.Client, out []model.Container) {
	healthcheckMu.Lock()
	known := healthcheckKnown[cli]
	healthcheckMu.Unlock()

	var todo []int
	for i, c := range out {
		if c.State != "running" && c.State != "paused" && c.State != "restarting" {
			continue
		}
		if has, ok := known[c.ID]; ok && !has {
			continue
		}
		todo = append(todo, i)
	}
	found := make([]*bool, len(todo))
	forEachBounded(ctx, len(todo), func(n int) {
		res, err := cli.ContainerInspect(ctx, out[todo[n]].ID, client.ContainerInspectOptions{})
		if err != nil || res.Container.State == nil {
			return
		}
		cfg := res.Container.Config
		has := cfg != nil && cfg.Healthcheck != nil && len(cfg.Healthcheck.Test) > 0 && cfg.Healthcheck.Test[0] != "NONE"
		if has && res.Container.State.Health != nil {
			out[todo[n]].Health = mapHealthStatus(res.Container.State.Health.Status)
		}
		found[n] = &has
	}, nil)

	next := make(map[string]bool, len(out))
	for _, c := range out {
		if has, ok := known[c.ID]; ok {
			next[c.ID] = has
		}
	}
	for n, f := range found {
		if f != nil {
			next[out[todo[n]].ID] = *f
		}
	}
	healthcheckMu.Lock()
	healthcheckKnown[cli] = next
	healthcheckMu.Unlock()
}
