package services

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/moby/moby/api/types/container"
)

// Second frame of a streaming stats request against Podman 6.1.4: an idle busybox loop.
// Podman's system_cpu_usage advances by 0.2 s while 5 s passed, so the Docker formula alone
// would report about 175 percent.
const podmanStatsFrame = `{"read":"2026-11-19T00:16:08.969895036+01:00","preread":"2026-11-19T00:16:03.963154415+01:00",
"cpu_stats":{"cpu_usage":{"total_usage":120824000},"system_cpu_usage":30668784000,"online_cpus":20},
"precpu_stats":{"cpu_usage":{"total_usage":102060000},"system_cpu_usage":30454711000,"online_cpus":20},
"memory_stats":{"usage":2179072,"limit":16647041024}}`

func TestCPUPercent_PodmanUsesWallClockBetweenSamples(t *testing.T) {
	var sr container.StatsResponse
	if err := json.Unmarshal([]byte(podmanStatsFrame), &sr); err != nil {
		t.Fatal(err)
	}
	got := cpuPercent(sr)
	if got == nil || math.Abs(*got-0.3747) > 0.01 {
		t.Fatalf("want about 0.37 percent (what Podman itself reports), got %v", got)
	}
}

func TestCPUPercent_DockerFormulaIsKeptWhenTheClocksAgree(t *testing.T) {
	// 1 s apart, 4 CPUs: system delta 4 s. The container used 1 s of CPU: 100 percent of one core.
	const frame = `{"read":"2026-01-01T00:00:01Z","preread":"2026-01-01T00:00:00Z",
"cpu_stats":{"cpu_usage":{"total_usage":2000000000},"system_cpu_usage":14000000000,"online_cpus":4},
"precpu_stats":{"cpu_usage":{"total_usage":1000000000},"system_cpu_usage":10000000000,"online_cpus":4}}`
	var sr container.StatsResponse
	if err := json.Unmarshal([]byte(frame), &sr); err != nil {
		t.Fatal(err)
	}
	if got := cpuPercent(sr); got == nil || math.Abs(*got-100) > 0.01 {
		t.Fatalf("want 100, got %v", got)
	}
}
