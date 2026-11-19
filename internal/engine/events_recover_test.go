package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"docklet/internal/model"
)

// stateRig is a Manager plus one connection whose status the test sets directly, so markDegraded and
// recover can be driven without a running event stream.
type stateRig struct {
	m  *Manager
	em *recEmitter
	c  *conn
	d  *fakeDaemon
}

func newStateRig(t *testing.T, daemonUp bool, state model.ConnectionState) *stateRig {
	t.Helper()
	d := newFakeDaemon("9.9")
	m, em := testManager(t, map[string]*fakeDaemon{"a": d})
	p, err := m.Profile("a")
	if err != nil {
		t.Fatal(err)
	}
	cli, err := m.newClient(p)
	if err != nil {
		t.Fatal(err)
	}
	if !daemonUp {
		d.close()
	} else {
		t.Cleanup(d.close)
	}
	return &stateRig{m: m, em: em, d: d, c: &conn{profile: p, cli: cli, status: model.ConnectionStatus{ConnectionID: "a", State: state}}}
}

func (r *stateRig) state() model.ConnectionState {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	return r.c.status.State
}

func (r *stateRig) lastStatus(t *testing.T) model.ConnectionStatus {
	t.Helper()
	r.em.mu.Lock()
	defer r.em.mu.Unlock()
	for i := len(r.em.got) - 1; i >= 0; i-- {
		if r.em.got[i] == model.EventConnectionStatus {
			return r.em.data[i].(model.ConnectionStatus)
		}
	}
	t.Fatal("no status event emitted")
	return model.ConnectionStatus{}
}

func TestMarkDegraded_EngineGoneFlipsConnectedToErrorOnce(t *testing.T) {
	r := newStateRig(t, false, model.StateConnected)
	cause := errors.New("read tcp: connection reset password=hunter2")

	r.m.markDegraded(context.Background(), "a", r.c, r.c.cli, cause)

	if r.state() != model.StateError {
		t.Fatalf("state = %s, want error", r.state())
	}
	if n := r.em.count(model.EventConnectionStatus); n != 1 {
		t.Fatalf("emitted %d status events, want 1", n)
	}
	st := r.lastStatus(t)
	if st.ConnectionID != "a" || st.State != model.StateError || !strings.HasPrefix(st.Error, "Lost connection to Docker Engine; retrying.") {
		t.Fatalf("status %+v", st)
	}
	if strings.Contains(st.Diagnostics, "hunter2") || !strings.Contains(st.Diagnostics, "connection reset") {
		t.Fatalf("diagnostics must keep the cause but redact secrets: %q", st.Diagnostics)
	}

	// The next failed reconnect attempt must not re-announce the same error.
	r.m.markDegraded(context.Background(), "a", r.c, r.c.cli, cause)
	if n := r.em.count(model.EventConnectionStatus); n != 1 {
		t.Fatalf("double transition: %d status events", n)
	}
}

func TestMarkDegraded_NilCauseStillExplainsTheLoss(t *testing.T) {
	r := newStateRig(t, false, model.StateConnected)
	r.m.markDegraded(context.Background(), "a", r.c, r.c.cli, nil)
	st := r.lastStatus(t)
	if st.State != model.StateError || st.Diagnostics != errNotConnected.Error() {
		t.Fatalf("status %+v", st)
	}
}

func TestMarkDegraded_StreamHiccupWithEngineUpChangesNothing(t *testing.T) {
	r := newStateRig(t, true, model.StateConnected)
	r.m.markDegraded(context.Background(), "a", r.c, r.c.cli, errors.New("EOF"))
	if r.state() != model.StateConnected || r.em.count(model.EventConnectionStatus) != 0 {
		t.Fatalf("state=%s emitted=%d", r.state(), r.em.count(model.EventConnectionStatus))
	}
}

func TestMarkDegraded_CancelledContextIsShutdownNotAnOutage(t *testing.T) {
	r := newStateRig(t, false, model.StateConnected)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.m.markDegraded(ctx, "a", r.c, r.c.cli, errors.New("context canceled"))
	if r.state() != model.StateConnected || r.em.count(model.EventConnectionStatus) != 0 {
		t.Fatalf("a manager shutting down must not report an error: state=%s", r.state())
	}
}

func TestRecover_ReturnsToConnectedWhenEngineIsBack(t *testing.T) {
	r := newStateRig(t, true, model.StateError)
	r.m.recover(context.Background(), "a", r.c, r.c.cli)
	if r.state() != model.StateConnected {
		t.Fatalf("state = %s", r.state())
	}
	st := r.lastStatus(t)
	if st.State != model.StateConnected || st.ServerVersion != "9.9" || st.ConnectionID != "a" || st.Error != "" {
		t.Fatalf("status %+v", st)
	}
	if n := r.em.count(model.EventConnectionStatus); n != 1 {
		t.Fatalf("emitted %d status events, want 1", n)
	}
}

func TestRecover_StaysInErrorWhileEngineIsStillDown(t *testing.T) {
	r := newStateRig(t, false, model.StateError)
	r.m.recover(context.Background(), "a", r.c, r.c.cli)
	if r.state() != model.StateError || r.em.count(model.EventConnectionStatus) != 0 {
		t.Fatalf("state=%s emitted=%d", r.state(), r.em.count(model.EventConnectionStatus))
	}
}

func TestRecover_AlreadyConnectedDoesNotProbeOrEmit(t *testing.T) {
	r := newStateRig(t, true, model.StateConnected)
	r.m.recover(context.Background(), "a", r.c, r.c.cli)
	if r.em.count(model.EventConnectionStatus) != 0 {
		t.Fatal("a healthy connection must not be re-announced when the stream restarts")
	}
}

// Lost, lost again, recover attempt while still down: one announcement in total.
func TestDegradedThenRecoverWhileStillDownEmitsOnce(t *testing.T) {
	r := newStateRig(t, true, model.StateConnected)
	r.d.srv.CloseClientConnections()
	r.d.srv.Close() // engine goes away

	r.m.markDegraded(context.Background(), "a", r.c, r.c.cli, errors.New("EOF"))
	r.m.markDegraded(context.Background(), "a", r.c, r.c.cli, errors.New("EOF"))
	r.m.recover(context.Background(), "a", r.c, r.c.cli) // still down
	if r.state() != model.StateError || r.em.count(model.EventConnectionStatus) != 1 {
		t.Fatalf("state=%s emitted=%d", r.state(), r.em.count(model.EventConnectionStatus))
	}
}
