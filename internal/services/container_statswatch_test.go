package services

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"docklet/internal/model"
)

func statsFake(t *testing.T) (*ContainerService, *recEmitter, *counter) {
	t.Helper()
	open := &counter{}
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/stats") {
			http.NotFound(w, r)
			return
		}
		open.add(1)
		defer open.add(-1)
		w.Header().Set("Content-Type", "application/json")
		for i := 1; ; i++ {
			fmt.Fprintf(w, `{"cpu_stats":{"cpu_usage":{"total_usage":%d},"system_cycles":0,"system_cpu_usage":%d,"online_cpus":2},"memory_stats":{"usage":1048576,"limit":4194304}}`+"\n", i*1000, i*10000)
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	}), "1.45")
	em := &recEmitter{}
	return &ContainerService{Engines: fakeProvider{cli}, Events: em}, em, open
}

func statBatches(em *recEmitter) (out []model.ContainerStatsData) {
	for _, ev := range em.snapshot() {
		if d, ok := ev.data.(model.ContainerStatsData); ok {
			out = append(out, d)
		}
	}
	return
}

func TestStatsWatch_BatchesAllContainersAndStopsCleanly(t *testing.T) {
	old := statsEmitInterval
	statsEmitInterval, statsTick = 150*time.Millisecond, 20*time.Millisecond
	defer func() { statsEmitInterval, statsTick = old, 250*time.Millisecond }()
	base := runtime.NumGoroutine()

	s, em, open := statsFake(t)
	id := s.StartStatsWatch(context.Background(), "c1", []string{"a", "b", "a"})
	eventually(t, "2 streams", func() bool { return open.get() == 2 })
	eventually(t, "3 batches", func() bool { return len(statBatches(em)) >= 3 })

	b := statBatches(em)
	if b[0].WatchID != id || b[0].ConnectionID != "c1" {
		t.Fatalf("batch header: %+v", b[0])
	}
	last := b[len(b)-1]
	if len(last.Stats) != 2 || last.Stats[0].ID != "a" || last.Stats[1].ID != "b" {
		t.Fatalf("one batched event must carry both containers: %+v", last.Stats)
	}
	if st := last.Stats[0]; st.CPUPercent == nil || *st.CPUPercent <= 0 || st.MemoryBytes == nil || *st.MemoryBytes != 1048576 || st.MemoryLimit == nil {
		t.Errorf("sample: %+v", st)
	}

	s.UpdateStatsWatch(context.Background(), id, []string{"b"})
	eventually(t, "1 stream after update", func() bool { return open.get() == 1 })

	s.StopStatsWatch(context.Background(), id)
	if s.watches.count() != 0 {
		t.Errorf("watch not removed")
	}
	eventually(t, "all streams closed", func() bool { return open.get() == 0 })
	n := len(statBatches(em))
	time.Sleep(300 * time.Millisecond)
	if len(statBatches(em)) != n {
		t.Errorf("events after stop")
	}
	s.StopStatsWatch(context.Background(), id) // idempotent
	http.DefaultClient.CloseIdleConnections()
	eventually(t, "no goroutine leak", func() bool { return runtime.NumGoroutine() <= base+8 })
}

func TestStatsWatch_RateLimitsEventsAndShutdownStopsStreams(t *testing.T) {
	old := statsEmitInterval
	statsEmitInterval, statsTick = 200*time.Millisecond, 20*time.Millisecond
	defer func() { statsEmitInterval, statsTick = old, 250*time.Millisecond }()
	s, em, open := statsFake(t)
	s.StartStatsWatch(context.Background(), "c1", []string{"a"})
	s.StartStatsWatch(context.Background(), "c1", []string{"b"})
	time.Sleep(700 * time.Millisecond)
	// 2 watches * (<= 700/200 + 1) batches
	if n := len(statBatches(em)); n > 8 {
		t.Errorf("rate limit exceeded: %d batches", n)
	}
	if err := s.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	if s.watches.count() != 0 {
		t.Errorf("watches left after shutdown")
	}
	eventually(t, "streams closed", func() bool { return open.get() == 0 })
}

func TestStatsWatch_CapsConcurrentStreams(t *testing.T) {
	old, oldMax := statsEmitInterval, maxStatsStreams
	statsEmitInterval, statsTick, maxStatsStreams = 50*time.Millisecond, 10*time.Millisecond, 1
	defer func() { statsEmitInterval, statsTick, maxStatsStreams = old, 250*time.Millisecond, oldMax }()
	s, _, open := statsFake(t)
	id := s.StartStatsWatch(context.Background(), "c1", []string{"a", "b", "c"})
	eventually(t, "exactly one stream", func() bool { return open.get() >= 1 })
	if open.get() > 1+maxConcurrency {
		t.Errorf("stream cap exceeded: %d", open.get())
	}
	s.StopStatsWatch(context.Background(), id)
}

func TestStatsWatch_ExpiresWithoutHeartbeat(t *testing.T) {
	oldTTL := statsWatchTTL
	statsEmitInterval, statsTick, statsWatchTTL = 50*time.Millisecond, 10*time.Millisecond, 150*time.Millisecond
	defer func() { statsEmitInterval, statsTick, statsWatchTTL = time.Second, 250*time.Millisecond, oldTTL }()
	s, _, open := statsFake(t)
	s.StartStatsWatch(context.Background(), "c1", []string{"a"})
	eventually(t, "expired", func() bool { return s.watches.count() == 0 && open.get() == 0 })
}
