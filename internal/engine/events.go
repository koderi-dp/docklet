package engine

import (
	"context"
	"sync"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"

	"docklet/internal/model"
)

// EventObserver receives every raw Engine event of every connected engine.
// Implementations must return quickly: they run on the event loop.
type EventObserver interface {
	ObserveEvent(connectionID string, ev events.Message)
}

// SetEventObserver installs (or, with nil, removes) the optional event observer.
func (m *Manager) SetEventObserver(o EventObserver) {
	if o == nil {
		m.observer.Store(nil)
		return
	}
	m.observer.Store(&o)
}

// maxBurstWait bounds how long continuous events may postpone a notification.
var maxBurstWait = 2 * time.Second

// debouncer coalesces bursts of Trigger calls into one fn call after d of quiet.
type debouncer struct {
	d  time.Duration
	fn func()
	mu sync.Mutex
	t  *time.Timer
	// first is when the current burst began; a burst longer than maxBurstWait fires anyway.
	first time.Time
	// stopped prevents any further fn calls.
	stopped bool
}

func newDebouncer(d time.Duration, fn func()) *debouncer { return &debouncer{d: d, fn: fn} }

func (b *debouncer) Trigger() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return
	}
	now := time.Now()
	if b.t != nil {
		b.t.Stop()
	} else {
		b.first = now
	}
	// Under a steady event stream pure debouncing never fires; cap how long a burst may defer it.
	delay := min(b.d, max(0, maxBurstWait-now.Sub(b.first)))
	b.t = time.AfterFunc(delay, func() {
		b.mu.Lock()
		stopped := b.stopped
		b.t = nil
		b.mu.Unlock()
		if !stopped {
			b.fn()
		}
	})
}

func (b *debouncer) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = true
	if b.t != nil {
		b.t.Stop()
	}
}

// runEvents streams Engine container events until ctx is cancelled, emitting a
// debounced containers:changed. On stream failure it re-probes the engine,
// updates/emits connection status, and reconnects with exponential backoff.
func (m *Manager) runEvents(ctx context.Context, id string, c *conn, cli *client.Client) {
	debs := map[events.Type]*debouncer{
		events.ContainerEventType: newDebouncer(m.debounce, func() {
			m.emitEvent(model.EventContainersChanged, model.ContainersChanged{ConnectionID: id})
		}),
		events.ImageEventType: newDebouncer(m.debounce, func() {
			m.emitEvent(model.EventImagesChanged, model.ImagesChanged{ConnectionID: id})
		}),
		events.VolumeEventType: newDebouncer(m.debounce, func() {
			m.emitEvent(model.EventVolumesChanged, model.VolumesChanged{ConnectionID: id})
		}),
		events.NetworkEventType: newDebouncer(m.debounce, func() {
			m.emitEvent(model.EventNetworksChanged, model.NetworksChanged{ConnectionID: id})
		}),
	}
	defer func() {
		for _, d := range debs {
			d.Stop()
		}
	}()
	backoff := m.backoffMin
	dropped := false
	for ctx.Err() == nil {
		if dropped {
			m.recover(ctx, id, c, cli)
		}
		res := cli.Events(ctx, client.EventsListOptions{})
		if dropped {
			// We may have missed events while the stream was down.
			for _, d := range debs {
				d.Trigger()
			}
		}
		err := m.consume(ctx, id, res, debs, func() { backoff = m.backoffMin })
		if ctx.Err() != nil {
			return
		}
		dropped = true
		m.markDegraded(ctx, id, c, cli, err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > m.backoffMax {
			backoff = m.backoffMax
		}
	}
}

// consume reads until the stream errors or ctx ends. onFirst runs on the first message.
func (m *Manager) consume(ctx context.Context, id string, res client.EventsResult, debs map[events.Type]*debouncer, onFirst func()) error {
	first := true
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-res.Messages:
			if !ok {
				return <-res.Err
			}
			if first {
				first = false
				onFirst()
			}
			if d := debs[ev.Type]; d != nil {
				d.Trigger()
			}
			if o := m.observer.Load(); o != nil {
				(*o).ObserveEvent(id, ev)
			}
		case err := <-res.Err:
			if err == nil {
				err = errNotConnected
			}
			return err
		}
	}
}

// markDegraded flips a connected connection to error when the engine is unreachable.
func (m *Manager) markDegraded(ctx context.Context, id string, c *conn, cli *client.Client, cause error) {
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(pctx, client.PingOptions{}); err == nil {
		return // engine is up; only the stream hiccuped
	}
	if ctx.Err() != nil {
		return
	}
	m.mu.RLock()
	already := c.status.State == model.StateError
	m.mu.RUnlock()
	if already {
		return
	}
	if cause == nil {
		cause = errNotConnected
	}
	m.setStatus(id, c, model.ConnectionStatus{State: model.StateError,
		Error:       "Lost connection to Docker Engine; retrying. " + friendlyError(cause, c.profile.Endpoint),
		Diagnostics: Redact(cause.Error())})
}

// recover refreshes status after the stream came back from an error state.
func (m *Manager) recover(ctx context.Context, id string, c *conn, cli *client.Client) {
	m.mu.RLock()
	was := c.status.State
	m.mu.RUnlock()
	if was == model.StateConnected {
		return
	}
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if st, err := m.probe(pctx, cli, c.profile); err == nil {
		m.setStatus(id, c, st)
	}
}
