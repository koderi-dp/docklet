// Package ops is the central background-operation manager (spec section 11).
// Services start operations here; the manager owns lifecycle, cancellation (Go
// contexts), capped log buffers and frontend events. It is transport-agnostic.
package ops

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"docklet/internal/engine"
	"docklet/internal/model"
)

const (
	maxLogLines  = 5000
	maxFinished  = 50
	emitInterval = 150 * time.Millisecond
)

// Spec describes an operation to start.
type Spec struct {
	Kind         string
	Title        string
	ConnectionID string
	Resource     string
	// Cancellable=false hides the cancel control (the ctx is still cancelled on app shutdown).
	Cancellable bool
}

// Func is the operation body. Returning ctx.Err() after cancellation marks the
// operation cancelled; any other error marks it failed.
type Func func(ctx context.Context, h *Handle) error

type entry struct {
	mu       sync.Mutex
	op       model.Operation
	logs     []string
	cancel   context.CancelFunc
	lastEmit time.Time
	pending  []string
}

func (e *entry) append(line string) {
	e.logs = append(e.logs, line)
	if len(e.logs) > maxLogLines {
		e.logs = e.logs[len(e.logs)-maxLogLines:]
	}
	e.pending = append(e.pending, line)
}

// Handle lets the operation body report progress and logs.
type Handle struct {
	m *Manager
	e *entry
}

// ID returns the operation ID.
func (h *Handle) ID() string { return h.e.op.ID }

// Log appends a log line (it also becomes the Detail line).
func (h *Handle) Log(line string) {
	h.m.update(h.e, func(e *entry) { e.append(line); e.op.Detail = line }, false)
}

// Detail sets the latest status line without logging it.
func (h *Handle) Detail(s string) { h.m.update(h.e, func(e *entry) { e.op.Detail = s }, false) }

// Progress sets real progress in 0..1; pass a negative value to clear it (unknown).
func (h *Handle) Progress(p float64) {
	h.m.update(h.e, func(e *entry) {
		if p < 0 {
			e.op.Progress = nil
			return
		}
		if p > 1 {
			p = 1
		}
		e.op.Progress = &p
	}, false)
}

// Manager tracks operations. Safe for concurrent use.
type Manager struct {
	emit engine.Emitter
	// OnFinish, when set before any Start, is called once per operation after it
	// reaches a terminal status (e.g. to persist history). It must not block long.
	OnFinish func(model.Operation)
	seq      atomic.Uint64
	mu       sync.Mutex
	ops      map[string]*entry
	wg       sync.WaitGroup
}

// New creates a Manager. emit may be nil (tests).
func New(emit engine.Emitter) *Manager {
	return &Manager{emit: emit, ops: map[string]*entry{}}
}

// Start launches fn in a goroutine and returns the initial operation snapshot.
func (m *Manager) Start(spec Spec, fn Func) model.Operation {
	ctx, cancel := context.WithCancel(context.Background())
	id := fmt.Sprintf("op-%d-%d", time.Now().UnixMilli(), m.seq.Add(1))
	e := &entry{cancel: cancel, op: model.Operation{
		ID: id, Kind: spec.Kind, Title: spec.Title, ConnectionID: spec.ConnectionID,
		Resource: spec.Resource, Status: model.OpRunning, StartedAt: time.Now().UnixMilli(),
		Cancellable: spec.Cancellable,
	}}
	m.mu.Lock()
	m.ops[id] = e
	m.pruneLocked()
	m.mu.Unlock()
	snap := e.op
	m.send(model.EventOperationUpdated, snap)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer cancel()
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("internal error: %v", r)
				}
			}()
			err = fn(ctx, &Handle{m: m, e: e})
		}()
		m.update(e, func(e *entry) {
			e.op.FinishedAt = time.Now().UnixMilli()
			switch {
			case err == nil:
				e.op.Status = model.OpSucceeded
			case ctx.Err() != nil && errors.Is(err, context.Canceled):
				e.op.Status = model.OpCancelled
			default:
				e.op.Status = model.OpFailed
				e.op.Error = err.Error()
			}
		}, true)
		if m.OnFinish != nil {
			e.mu.Lock()
			final := e.op
			e.mu.Unlock()
			m.OnFinish(final)
		}
	}()
	return snap
}

// update mutates the entry and emits (throttled unless force).
func (m *Manager) update(e *entry, fn func(*entry), force bool) {
	e.mu.Lock()
	fn(e)
	now := time.Now()
	if !force && now.Sub(e.lastEmit) < emitInterval {
		e.mu.Unlock()
		return
	}
	e.lastEmit = now
	snap := e.op
	lines := e.pending
	e.pending = nil
	e.mu.Unlock()
	if len(lines) > 0 {
		m.send(model.EventOperationLog, model.OperationLog{ID: snap.ID, Lines: lines})
	}
	m.send(model.EventOperationUpdated, snap)
}

func (m *Manager) send(name string, data any) {
	if m.emit != nil {
		m.emit.Emit(name, data)
	}
}

// pruneLocked drops the oldest finished operations beyond maxFinished.
func (m *Manager) pruneLocked() {
	var done []*entry
	for _, e := range m.ops {
		e.mu.Lock()
		if e.op.Status != model.OpRunning {
			done = append(done, e)
		}
		e.mu.Unlock()
	}
	if len(done) <= maxFinished {
		return
	}
	sort.Slice(done, func(i, j int) bool { return done[i].op.FinishedAt < done[j].op.FinishedAt })
	for _, e := range done[:len(done)-maxFinished] {
		delete(m.ops, e.op.ID)
	}
}

// List returns all operations, newest first.
func (m *Manager) List() []model.Operation {
	m.mu.Lock()
	out := make([]model.Operation, 0, len(m.ops))
	for _, e := range m.ops {
		e.mu.Lock()
		out = append(out, e.op)
		e.mu.Unlock()
	}
	m.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	return out
}

// Logs returns the buffered log lines of one operation.
func (m *Manager) Logs(id string) ([]string, error) {
	m.mu.Lock()
	e, ok := m.ops[id]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("operation %q not found", id)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.logs...), nil
}

// Cancel cancels the context of the operation. It stops the client-side work;
// whether the server-side work stops depends on the Docker API involved.
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	e, ok := m.ops[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("operation %q not found", id)
	}
	e.cancel()
	return nil
}

// ClearFinished removes all finished operations.
func (m *Manager) ClearFinished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.ops {
		e.mu.Lock()
		done := e.op.Status != model.OpRunning
		e.mu.Unlock()
		if done {
			delete(m.ops, id)
		}
	}
}

// Shutdown cancels everything and waits briefly for bodies to return.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	for _, e := range m.ops {
		e.cancel()
	}
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}
