package services

import (
	"context"
	"docklet/internal/model"
	"encoding/json"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// Stats takes one-shot samples for the given running containers only. Unavailable => nil fields.
func (s *ContainerService) Stats(ctx context.Context, connectionID string, ids []string) ([]model.ContainerStat, error) {
	if len(ids) == 0 {
		return []model.ContainerStat{}, nil
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	slots := make([]*model.ContainerStat, len(ids))
	forEachBounded(ctx, len(ids), func(i int) { slots[i] = sampleStats(ctx, cli, ids[i]) }, nil)
	out := make([]model.ContainerStat, 0, len(ids))
	for _, st := range slots {
		if st != nil {
			out = append(out, *st)
		}
	}
	return out, nil
}

func sampleStats(ctx context.Context, cli *client.Client, id string) *model.ContainerStat {
	insp, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil || insp.Container.State == nil || !insp.Container.State.Running {
		return nil
	}
	// IncludePreviousSample makes the daemon take two samples ~1s apart so the
	// pre-CPU delta is populated (a bare one-shot has no previous sample).
	res, err := cli.ContainerStats(ctx, id, client.ContainerStatsOptions{IncludePreviousSample: true})
	if err != nil {
		return &model.ContainerStat{ID: id}
	}
	defer res.Body.Close()
	var sr container.StatsResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return &model.ContainerStat{ID: id}
	}
	return statFromResponse(id, sr)
}

func statFromResponse(id string, sr container.StatsResponse) *model.ContainerStat {
	st := &model.ContainerStat{ID: id}
	st.CPUPercent = cpuPercent(sr)
	if sr.MemoryStats.Usage > 0 {
		used := sr.MemoryStats.Usage
		// Docker CLI convention: subtract page cache (cgroup v2 inactive_file, v1 total_inactive_file).
		var cache uint64
		if v, ok := sr.MemoryStats.Stats["inactive_file"]; ok {
			cache = v
		} else if v, ok := sr.MemoryStats.Stats["total_inactive_file"]; ok {
			cache = v
		}
		if cache < used {
			used -= cache
		}
		st.MemoryBytes = &used
	}
	if sr.MemoryStats.Limit > 0 {
		l := sr.MemoryStats.Limit
		st.MemoryLimit = &l
	}
	return st
}

func cpuPercent(sr container.StatsResponse) *float64 {
	cur, pre := sr.CPUStats, sr.PreCPUStats
	if pre.CPUUsage.TotalUsage == 0 || pre.SystemUsage == 0 || cur.SystemUsage <= pre.SystemUsage {
		return nil
	}
	if cur.CPUUsage.TotalUsage < pre.CPUUsage.TotalUsage {
		return nil
	}
	cpuDelta := float64(cur.CPUUsage.TotalUsage - pre.CPUUsage.TotalUsage)
	sysDelta := float64(cur.SystemUsage - pre.SystemUsage)
	cpus := float64(cur.OnlineCPUs)
	if cpus == 0 {
		cpus = float64(len(cur.CPUUsage.PercpuUsage))
	}
	if cpus == 0 {
		return nil
	}
	// Podman's system_cpu_usage does not advance with the host's CPU time the way Docker's does, so
	// the ratio would read several hundred percent for an idle container. When the Engine's own read
	// timestamps disagree with that clock by more than 2x, use the wall clock between the two samples.
	if elapsed := sr.Read.Sub(sr.PreRead); !sr.PreRead.IsZero() && sr.PreRead.Year() > 1 && elapsed > 0 {
		expected := float64(elapsed.Nanoseconds()) * cpus
		if sysDelta < expected/2 || sysDelta > expected*2 {
			v := cpuDelta / float64(elapsed.Nanoseconds()) * 100.0
			return &v
		}
	}
	v := cpuDelta / sysDelta * cpus * 100.0
	return &v
}
