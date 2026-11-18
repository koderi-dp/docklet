package services

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func roundRobin(ids []string, cursor *int, n int) []string {
	if n > len(ids) {
		n = len(ids)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ids[(*cursor+i)%len(ids)])
	}
	*cursor = (*cursor + n) % len(ids)
	return out
}

func (w *statsWatch) sampleBatch(ctx context.Context, cli *client.Client, ids []string) {
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			if st := sampleStats(sctx, cli, id); st != nil {
				w.push(*st)
			}
		}(id)
	}
	wg.Wait()
}

// followStats reads one streaming stats request, computing CPU% from consecutive
// frames. It returns when the stream ends (container stopped, connection lost,
// context cancelled); the reconcile loop re-opens it after statsRetryAfter.
func (w *statsWatch) followStats(ctx context.Context, cli *client.Client, id string) {
	res, err := cli.ContainerStats(ctx, id, client.ContainerStatsOptions{Stream: true})
	if err != nil {
		return
	}
	defer res.Body.Close()
	dec := json.NewDecoder(res.Body)
	var prev *container.StatsResponse
	for {
		sr := new(container.StatsResponse)
		if err := dec.Decode(sr); err != nil {
			return
		}
		st := statFromResponse(id, *sr)
		if st.CPUPercent == nil && prev != nil {
			st.CPUPercent = cpuPercentBetween(prev, sr)
		}
		prev = sr
		w.push(*st)
	}
}

// cpuPercentBetween computes CPU% from two consecutive frames (the daemon never
// fills PreCPUStats on a streaming request's first record).
func cpuPercentBetween(prev, cur *container.StatsResponse) *float64 {
	p := *cur
	p.PreCPUStats = prev.CPUStats
	return cpuPercent(p)
}
