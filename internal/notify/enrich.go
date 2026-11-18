package notify

import (
	"context"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

// enrich completes events that Podman's compatibility layer reports more thinly than Docker:
//   - "health_status" carries no verdict (Docker sends "health_status: unhealthy"), so the container
//     is inspected for its health status;
//   - an out-of-memory kill arrives only as a "die" with exit code 137 (no "oom" event), so a failing
//     die is inspected for State.OOMKilled and an "oom" event is placed before it.
//
// On Docker these events already look right and pass through unchanged: a duplicate "oom" would be
// coalesced by the rules anyway.
func (h *Hub) enrich(ev Event) []Event {
	if h.d.Engines == nil {
		return []Event{ev}
	}
	switch {
	case ev.Action == "health_status":
		st := h.inspectState(ev)
		if st.health == "" || st.health == "none" {
			return nil
		}
		ev.Action = "health_status: " + st.health
	case ev.Action == "die" && ev.ExitCode != "" && ev.ExitCode != "0":
		if h.inspectState(ev).oomKilled {
			oom := ev
			oom.Action, oom.ExitCode = "oom", ""
			return []Event{oom, ev}
		}
	}
	return []Event{ev}
}

type inspected struct {
	health    string
	oomKilled bool
}

func (h *Hub) inspectState(ev Event) inspected {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cli, err := h.d.Engines.Client(ctx, ev.ConnectionID)
	if err != nil {
		return inspected{}
	}
	res, err := cli.ContainerInspect(ctx, ev.ContainerID, client.ContainerInspectOptions{})
	if err != nil || res.Container.State == nil {
		return inspected{}
	}
	out := inspected{oomKilled: res.Container.State.OOMKilled}
	if hl := res.Container.State.Health; hl != nil {
		out.health = strings.ToLower(string(hl.Status))
	}
	return out
}
