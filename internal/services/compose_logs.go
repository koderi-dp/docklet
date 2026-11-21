package services

import (
	"context"
	"docklet/internal/compose"
	"docklet/internal/model"
	"sort"
	"time"
)

// minPerContainerTail keeps quiet containers visible in a project with many containers.
const minPerContainerTail = 200

// perContainerTail is how many lines to ask each container for: about twice an even share of
// tail, never less than minPerContainerTail and never more than tail.
func perContainerTail(tail, containers int) int {
	if containers <= 1 {
		return tail
	}
	share := (2*tail + containers - 1) / containers
	if share < minPerContainerTail {
		share = minPerContainerTail
	}
	if share > tail {
		share = tail
	}
	return share
}

// ProjectLogs returns a snapshot of the last `tail` lines of the project, read as a share per
// container (see perContainerTail), merged by timestamp and trimmed to `tail` lines overall.
func (s *ComposeService) ProjectLogs(ctx context.Context, connectionID, name string, tail int) ([]model.ComposeLogLine, error) {
	cs, err := s.labelled(ctx, connectionID, name)
	if err != nil {
		return nil, err
	}
	if tail <= 0 {
		tail = defaultLogTail
	}
	if tail > maxLogTail {
		tail = maxLogTail
	}
	share := perContainerTail(tail, len(cs))
	cont := &ContainerService{Engines: s.Engines, Events: s.Events}
	type chunk struct {
		c     compose.Container
		lines []model.LogLine
		err   error
	}
	chunks := make([]chunk, len(cs))
	forEachBounded(ctx, len(cs), func(i int) {
		lines, err := cont.Logs(ctx, connectionID, cs[i].ID, share)
		chunks[i] = chunk{c: cs[i], lines: lines, err: err}
	}, func(i int) {
		chunks[i] = chunk{c: cs[i], err: ctx.Err()}
	})
	var in []projectLogChunk
	failed := 0
	var lastErr error
	for _, ch := range chunks {
		if ch.err != nil {
			failed++
			lastErr = ch.err
			continue
		}
		in = append(in, projectLogChunk{Service: ch.c.Labels[compose.LabelService], Container: ch.c.Name, Lines: ch.lines})
	}
	if len(cs) > 0 && failed == len(cs) {
		return nil, friendlyError(lastErr)
	}
	return mergeProjectLogs(in, tail), nil
}

type projectLogChunk struct {
	Service   string
	Container string
	Lines     []model.LogLine
}

// mergeProjectLogs merges per-container snapshots by timestamp (stable), keeping the last tail lines.
func mergeProjectLogs(in []projectLogChunk, tail int) []model.ComposeLogLine {
	type item struct {
		t time.Time
		l model.ComposeLogLine
	}
	var items []item
	for _, ch := range in {
		for _, ln := range ch.Lines {
			t, _ := time.Parse(time.RFC3339Nano, ln.Timestamp)
			items = append(items, item{t: t, l: model.ComposeLogLine{
				Service: ch.Service, Container: ch.Container, Timestamp: ln.Timestamp, Stream: ln.Stream, Text: ln.Text}})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].t.Before(items[j].t) })
	if tail > 0 && len(items) > tail {
		items = items[len(items)-tail:]
	}
	out := make([]model.ComposeLogLine, len(items))
	for i, it := range items {
		out[i] = it.l
	}
	return out
}
