package services

import (
	"math"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
)

func cdU64(v uint64) *uint64 { return &v }

func cdEq(t *testing.T, name string, got *uint64, want *uint64) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("%s = %v, want %v", name, got, want)
	case *got != *want:
		t.Errorf("%s = %d, want %d", name, *got, *want)
	}
}

func TestDetailFromResponse_EmptyResponseLeavesEverythingUnset(t *testing.T) {
	d := detailFromResponse("abc", container.StatsResponse{})
	if d.ID != "abc" || d.Timestamp != "" || d.CPUPercent != nil || d.MemoryBytes != nil || d.MemoryLimit != nil ||
		d.NetRxBytes != nil || d.NetTxBytes != nil || d.BlockReadBytes != nil || d.BlockWriteBytes != nil || d.Pids != nil {
		t.Errorf("missing values must stay nil, not 0: %+v", d)
	}
}

func TestDetailFromResponse_SumsNetworksAcrossInterfaces(t *testing.T) {
	sr := container.StatsResponse{Networks: map[string]container.NetworkStats{
		"eth0": {RxBytes: 100, TxBytes: 10},
		"eth1": {RxBytes: 5, TxBytes: 0},
	}}
	d := detailFromResponse("x", sr)
	cdEq(t, "rx", d.NetRxBytes, cdU64(105))
	cdEq(t, "tx", d.NetTxBytes, cdU64(10))

	// An interface that exists but is idle is a real zero, not "unknown".
	d = detailFromResponse("x", container.StatsResponse{Networks: map[string]container.NetworkStats{"eth0": {}}})
	cdEq(t, "idle rx", d.NetRxBytes, cdU64(0))
	cdEq(t, "idle tx", d.NetTxBytes, cdU64(0))
}

func TestDetailFromResponse_BlockIOSumsReadAndWriteCaseInsensitively(t *testing.T) {
	var sr container.StatsResponse
	sr.BlkioStats.IoServiceBytesRecursive = []container.BlkioStatEntry{
		{Op: "Read", Value: 10}, {Op: "read", Value: 5},
		{Op: "WRITE", Value: 7}, {Op: "Write", Value: 1},
		{Op: "Sync", Value: 999}, {Op: "Total", Value: 999},
	}
	d := detailFromResponse("x", sr)
	cdEq(t, "read", d.BlockReadBytes, cdU64(15))
	cdEq(t, "write", d.BlockWriteBytes, cdU64(8))
}

func TestDetailFromResponse_BlockIOFallsBackToStorageStats(t *testing.T) {
	var sr container.StatsResponse
	sr.StorageStats.ReadSizeBytes = 40
	sr.StorageStats.WriteSizeBytes = 2
	d := detailFromResponse("x", sr)
	cdEq(t, "read", d.BlockReadBytes, cdU64(40))
	cdEq(t, "write", d.BlockWriteBytes, cdU64(2))

	// Blkio entries win over the fallback, even when they only hold other ops.
	sr.BlkioStats.IoServiceBytesRecursive = []container.BlkioStatEntry{{Op: "read", Value: 9}}
	d = detailFromResponse("x", sr)
	cdEq(t, "blkio read", d.BlockReadBytes, cdU64(9))
	cdEq(t, "blkio write", d.BlockWriteBytes, cdU64(0))
}

func TestDetailFromResponse_PidsAreSetOnlyWhenPositive(t *testing.T) {
	var sr container.StatsResponse
	if d := detailFromResponse("x", sr); d.Pids != nil {
		t.Errorf("0 pids means not reported, got %d", *d.Pids)
	}
	sr.PidsStats.Current = 12
	cdEq(t, "pids", detailFromResponse("x", sr).Pids, cdU64(12))
}

func TestDetailFromResponse_TimestampIsRFC3339OrOmittedForZeroTime(t *testing.T) {
	sr := container.StatsResponse{Read: time.Date(2026, 1, 2, 3, 4, 5, 6000, time.UTC)}
	if got := detailFromResponse("x", sr).Timestamp; got != "2026-01-02T03:04:05.000006Z" {
		t.Errorf("timestamp = %q", got)
	}
	// Podman sends Go's zero time for unset fields; it must not reach the UI as year 0001.
	if got := detailFromResponse("x", container.StatsResponse{Read: time.Time{}}).Timestamp; got != "" {
		t.Errorf("zero time must be omitted, got %q", got)
	}
}

func TestDetailFromResponse_MemoryExcludesPageCacheAndCarriesLimitAndCPU(t *testing.T) {
	sr := container.StatsResponse{
		CPUStats:    container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 2_000_000_000}, SystemUsage: 14_000_000_000, OnlineCPUs: 4},
		PreCPUStats: container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 1_000_000_000}, SystemUsage: 10_000_000_000, OnlineCPUs: 4},
		MemoryStats: container.MemoryStats{Usage: 1000, Limit: 4096, Stats: map[string]uint64{"inactive_file": 300}},
	}
	d := detailFromResponse("x", sr)
	cdEq(t, "memory", d.MemoryBytes, cdU64(700))
	cdEq(t, "limit", d.MemoryLimit, cdU64(4096))
	if d.CPUPercent == nil || math.Abs(*d.CPUPercent-100) > 0.01 {
		t.Errorf("cpu = %v, want 100", d.CPUPercent)
	}

	// Cache larger than usage is ignored rather than underflowing the unsigned subtraction.
	sr.MemoryStats.Stats = map[string]uint64{"total_inactive_file": 5000}
	cdEq(t, "memory with oversized cache", detailFromResponse("x", sr).MemoryBytes, cdU64(1000))

	// Unlimited container: limit 0 is "no limit", not a zero limit.
	sr.MemoryStats.Limit = 0
	if detailFromResponse("x", sr).MemoryLimit != nil {
		t.Error("limit 0 must stay nil")
	}
}

func TestDetailFromResponse_CPUStaysUnsetWhenThereIsNoUsableBaseline(t *testing.T) {
	cases := map[string]container.StatsResponse{
		"first sample (no previous)": {CPUStats: container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 5}, SystemUsage: 100, OnlineCPUs: 2}},
		"system clock did not advance": {
			CPUStats:    container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 5}, SystemUsage: 100, OnlineCPUs: 2},
			PreCPUStats: container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 3}, SystemUsage: 100, OnlineCPUs: 2},
		},
		"no CPU count": {
			CPUStats:    container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 5}, SystemUsage: 200},
			PreCPUStats: container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 3}, SystemUsage: 100},
		},
		"counter went backwards": {
			CPUStats:    container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 3}, SystemUsage: 200, OnlineCPUs: 2},
			PreCPUStats: container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 5}, SystemUsage: 100, OnlineCPUs: 2},
		},
	}
	for name, sr := range cases {
		if d := detailFromResponse("x", sr); d.CPUPercent != nil {
			t.Errorf("%s: cpu = %v, want nil (no division by zero or negative delta)", name, *d.CPUPercent)
		}
	}
}
