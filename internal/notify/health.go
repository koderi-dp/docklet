package notify

import (
	"context"
	"strings"

	"github.com/moby/moby/client"

	"docklet/internal/engine"
)

const detailMaxLen = 300

// lastHealthOutput returns the cleaned output of the newest health-check run, or "".
func lastHealthOutput(ctx context.Context, p engine.Provider, connectionID, containerID string) string {
	cli, err := p.Client(ctx, connectionID)
	if err != nil {
		return ""
	}
	res, err := cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return ""
	}
	st := res.Container.State
	if st == nil || st.Health == nil || len(st.Health.Log) == 0 {
		return ""
	}
	last := st.Health.Log[len(st.Health.Log)-1]
	if last == nil {
		return ""
	}
	return cleanDetail(last.Output)
}

// cleanDetail keeps the last non-empty line, redacts credentials and truncates it.
func cleanDetail(s string) string {
	line := ""
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			line = t
		}
	}
	line = engine.Redact(line)
	if r := []rune(line); len(r) > detailMaxLen {
		line = string(r[:detailMaxLen]) + "..."
	}
	return line
}
