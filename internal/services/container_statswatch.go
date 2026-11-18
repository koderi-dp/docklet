package services

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/model"
)

// Tunables (variables so tests can shrink them).
var (
	statsEmitInterval = time.Second // minimum gap between "containerstats:data" batches
	statsTick         = 250 * time.Millisecond
	statsWatchTTL     = 45 * time.Second // a watch nobody touched for this long is torn down
	statsRetryAfter   = 5 * time.Second  // pause before re-opening a stream that ended
	maxStatsStreams   = 50               // concurrent streaming requests per watch
)

// statsWatch samples a set of containers of one connection with one streaming
// stats request per container and emits batched results.
type statsWatch struct {
	id, connectionID string
	svc              *ContainerService
	cancel           context.CancelFunc
	done             chan struct{}

	mu      sync.Mutex
	ids     []string // wanted container IDs (sorted, unique)
	touched time.Time
	dirty   map[string]model.ContainerStat // new samples since the last emit
	kick    chan struct{}                  // wakes the loop on update / first sample
}

// StartStatsWatch begins sampling the given running containers. Samples arrive as
// "containerstats:data" batches (at most one per second). The watch lives until
// StopStatsWatch, service shutdown, or ~45s without UpdateStatsWatch (callers
// heartbeat by re-sending their IDs), so a reloaded UI cannot leak streams.
func (s *ContainerService) StartStatsWatch(_ context.Context, connectionID string, ids []string) string {
	ctx, cancel := context.WithCancel(context.Background())
	w := &statsWatch{
		id: "sw-" + newStreamID()[3:], connectionID: connectionID, svc: s,
		cancel: cancel, done: make(chan struct{}),
		ids: normalizeIDs(ids), touched: time.Now(),
		dirty: map[string]model.ContainerStat{}, kick: make(chan struct{}, 1),
	}
	s.watches.add(w)
	go w.run(ctx)
	return w.id
}

// UpdateStatsWatch replaces the watched container set (also acts as a heartbeat).
func (s *ContainerService) UpdateStatsWatch(_ context.Context, watchID string, ids []string) {
	w := s.watches.get(watchID)
	if w == nil {
		return
	}
	w.mu.Lock()
	w.ids = normalizeIDs(ids)
	w.touched = time.Now()
	w.mu.Unlock()
	w.wake()
}

// StopStatsWatch tears the watch down and waits (bounded) for its goroutines.
// Unknown IDs are ignored.
func (s *ContainerService) StopStatsWatch(_ context.Context, watchID string) {
	w := s.watches.get(watchID)
	if w == nil {
		return
	}
	w.cancel()
	select {
	case <-w.done:
	case <-time.After(2 * time.Second):
	}
}

// ServiceShutdown stops every stats watch and log stream when the app quits.
func (s *ContainerService) ServiceShutdown() error {
	ws := s.watches.all()
	for _, w := range ws {
		w.cancel()
	}
	for _, w := range ws {
		select {
		case <-w.done:
		case <-time.After(2 * time.Second):
		}
	}
	s.streams.stopAll()
	return nil
}

func (w *statsWatch) wake() {
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

// push records a new sample; the first sample of a container wakes the loop so it
// does not wait for the next tick.
func (w *statsWatch) push(st model.ContainerStat) {
	w.mu.Lock()
	_, had := w.dirty[st.ID]
	w.dirty[st.ID] = st
	w.mu.Unlock()
	if !had {
		w.wake()
	}
}

type liveStream struct {
	cancel  context.CancelFunc
	done    chan struct{}
	retryAt time.Time // earliest time an ended stream may be re-opened
}

func (ls *liveStream) ended() bool {
	select {
	case <-ls.done:
		return true
	default:
		return false
	}
}

func (w *statsWatch) run(ctx context.Context) {
	defer close(w.done)
	defer w.svc.watches.remove(w.id)
	defer w.cancel()

	streams := map[string]*liveStream{}
	closeStream := func(id string) {
		ls := streams[id]
		ls.cancel()
		<-ls.done
		delete(streams, id)
	}
	defer func() {
		for id := range streams {
			closeStream(id)
		}
	}()

	var (
		curCli    *client.Client
		lastEmit  time.Time
		rr        int
		oneShotWG sync.WaitGroup
		oneShot   = make(chan struct{}, 1) // at most one round-robin sampling round in flight
	)
	defer func() { w.cancel(); oneShotWG.Wait() }()
	tick := time.NewTicker(statsTick)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-w.kick:
		}

		w.mu.Lock()
		expired := time.Since(w.touched) > statsWatchTTL
		want := w.ids
		w.mu.Unlock()
		if expired {
			return
		}

		cli, err := w.svc.client(ctx, w.connectionID)
		if err != nil || cli != curCli {
			// Connection lost or replaced: its streams are dead; drop them all.
			for id := range streams {
				closeStream(id)
			}
			curCli = cli
		}
		if err == nil {
			wantSet := make(map[string]struct{}, len(want))
			for _, id := range want {
				wantSet[id] = struct{}{}
			}
			for id, ls := range streams {
				if _, keep := wantSet[id]; !keep || (ls.ended() && time.Now().After(ls.retryAt)) {
					closeStream(id)
				}
			}
			var overflow []string
			for _, id := range want {
				if _, ok := streams[id]; ok {
					continue
				}
				if len(streams) >= maxStatsStreams {
					overflow = append(overflow, id)
					continue
				}
				sctx, cancel := context.WithCancel(ctx)
				ls := &liveStream{cancel: cancel, done: make(chan struct{}), retryAt: time.Now().Add(statsRetryAfter)}
				streams[id] = ls
				go func(id string) {
					defer close(ls.done)
					w.followStats(sctx, cli, id)
				}(id)
			}
			if len(overflow) > 0 {
				select {
				case oneShot <- struct{}{}:
					batch := roundRobin(overflow, &rr, maxConcurrency)
					oneShotWG.Add(1)
					go func() {
						defer oneShotWG.Done()
						defer func() { <-oneShot }()
						w.sampleBatch(ctx, cli, batch)
					}()
				default:
				}
			}
		}

		if time.Since(lastEmit) >= statsEmitInterval {
			w.mu.Lock()
			if len(w.dirty) == 0 {
				w.mu.Unlock()
				continue
			}
			batch := make([]model.ContainerStat, 0, len(w.dirty))
			for _, st := range w.dirty {
				batch = append(batch, st)
			}
			w.dirty = map[string]model.ContainerStat{}
			w.mu.Unlock()
			sort.Slice(batch, func(i, j int) bool { return batch[i].ID < batch[j].ID })
			lastEmit = time.Now()
			if w.svc.Events != nil {
				w.svc.Events.Emit(model.EventContainerStatsData, model.ContainerStatsData{
					WatchID: w.id, ConnectionID: w.connectionID, Stats: batch,
				})
			}
		}
	}
}
