package engine

import (
	"context"
	"docklet/internal/model"
	"strings"
	"testing"
	"time"
)

func TestManager_ClientIsPerConnectionAndErrorsWhenNotConnected(t *testing.T) {
	a, b := newFakeDaemon("A-1.0"), newFakeDaemon("B-2.0")
	defer a.close()
	defer b.close()
	m, _ := testManager(t, map[string]*fakeDaemon{"a": a, "b": b})
	ctx := context.Background()

	if _, err := m.Client(ctx, "a"); err == nil {
		t.Fatal("Client before Connect must error")
	}
	if _, err := m.Client(ctx, "nope"); err == nil {
		t.Fatal("Client for unknown ID must error")
	}
	sa, _ := m.Connect(ctx, "a")
	sb, _ := m.Connect(ctx, "b")
	if sa.State != model.StateConnected || sa.ServerVersion != "A-1.0" || sa.Volumes != 1 || sa.Networks != 2 || sa.Containers != 3 || sa.APIVersion != "1.45" {
		t.Fatalf("bad status a: %+v", sa)
	}
	if sb.ServerVersion != "B-2.0" {
		t.Fatalf("bad status b: %+v", sb)
	}
	ca, _ := m.Client(ctx, "a")
	cb, _ := m.Client(ctx, "b")
	if ca == nil || cb == nil || ca == cb {
		t.Fatal("each connection must get its own client")
	}
	if err := m.Disconnect(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Client(ctx, "a"); err == nil {
		t.Fatal("Client after Disconnect must error")
	}
	if cb2, err := m.Client(ctx, "b"); err != nil || cb2 != cb {
		t.Fatal("disconnecting a must not affect b")
	}
}

func TestManager_DebouncesEventsAndClosesStreamOnDisconnect(t *testing.T) {
	d := newFakeDaemon("1")
	defer d.close()
	m, em := testManager(t, map[string]*fakeDaemon{"a": d})
	ctx := context.Background()

	// Reconnecting must not leave duplicate subscriptions behind.
	for i := 0; i < 2; i++ {
		if st, _ := m.Connect(ctx, "a"); st.State != model.StateConnected {
			t.Fatalf("connect: %+v", st)
		}
	}
	waitFor(t, func() bool { return d.eventConns.Load() == 1 }, "exactly one open event stream")

	for i := 0; i < 5; i++ {
		d.events <- struct{}{}
	}
	waitFor(t, func() bool { return em.count(model.EventContainersChanged) >= 1 }, "debounced emit")
	time.Sleep(120 * time.Millisecond)
	if n := em.count(model.EventContainersChanged); n != 1 {
		t.Fatalf("burst of 5 events produced %d emits, want 1", n)
	}

	if err := m.Disconnect(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return d.eventConns.Load() == 0 }, "event stream closed after Disconnect")
	if n := d.eventOpens.Load(); n > 2 {
		t.Fatalf("event streams opened = %d, want <=2 (no reconnect loop)", n)
	}
	if em.count(model.EventConnectionStatus) < 3 {
		t.Fatal("expected connection:status emits")
	}
}

func TestManager_ConnectFailureYieldsFriendlyErrorStatus(t *testing.T) {
	st := &memStore{profiles: []model.ConnectionProfile{{ID: "x", Name: "x", Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:1"}}}
	m := NewManager(st, nil)
	got, err := m.Connect(context.Background(), "x")
	if err != nil || got.State != model.StateError || got.Error == "" || got.Diagnostics == "" {
		t.Fatalf("want friendly error status, got %+v err=%v", got, err)
	}
	if _, err := m.Client(context.Background(), "x"); err == nil {
		t.Fatal("no client expected after failed connect")
	}
}

func TestManagerTest_NeverRegistersClientOrEmitsEvents(t *testing.T) {
	d := newFakeDaemon("T-9")
	defer d.close()
	m, em := testManager(t, map[string]*fakeDaemon{"a": d})
	st := m.store.(*memStore)
	before := len(st.profiles)
	ep := "tcp://" + strings.TrimPrefix(d.srv.URL, "http://")
	p := model.ConnectionProfile{ID: "temp", Name: "tmp", Type: model.ConnTCP, Endpoint: ep}

	got, err := m.Test(context.Background(), p)
	if err != nil || got.State != model.StateConnected || got.ServerVersion != "T-9" {
		t.Fatalf("test status: %+v %v", got, err)
	}
	if _, err := m.Client(context.Background(), "temp"); err == nil {
		t.Fatal("Test must not register a live client")
	}
	m.mu.RLock()
	n := len(m.conns)
	m.mu.RUnlock()
	if n != 0 || len(st.profiles) != before {
		t.Fatalf("Test leaked state: conns=%d profiles=%d", n, len(st.profiles))
	}
	if em.count(model.EventConnectionStatus) != 0 || d.eventOpens.Load() != 0 {
		t.Fatal("Test must not emit status events or open an event stream")
	}

	// Invalid and unreachable profiles report friendly errors, redacted.
	bad, _ := m.Test(context.Background(), model.ConnectionProfile{Name: "x", Type: model.ConnTCP, Endpoint: "tcp://10.0.0.5:2375"})
	if bad.State != model.StateError || bad.Error == "" {
		t.Fatalf("invalid profile: %+v", bad)
	}
	down, _ := m.Test(context.Background(), model.ConnectionProfile{Name: "x", Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:1"})
	if down.State != model.StateError || down.Error == "" || down.Diagnostics == "" {
		t.Fatalf("unreachable: %+v", down)
	}
}

func TestManagerList_HidesDiscoveredProfileOnlyWhenEndpointIsSaved(t *testing.T) {
	def := DefaultLocalProfile()
	st := &memStore{profiles: []model.ConnectionProfile{{ID: "w", Name: "w", Type: model.ConnWSL, WSLDistro: "U"}}}
	m := NewManager(st, nil)
	l, _ := m.List(context.Background())
	if len(l) < 2 || l[0].ID != "w" || !l[1].Discovered {
		t.Fatalf("discovered must stay listed after saved: %+v", l)
	}
	st.profiles = append(st.profiles, model.ConnectionProfile{ID: "s", Name: "mine", Type: def.Type, Endpoint: def.Endpoint})
	l, _ = m.List(context.Background())
	for _, p := range l {
		if p.Discovered && p.Endpoint == def.Endpoint {
			t.Fatalf("discovered must hide when a saved profile has the same endpoint: %+v", l)
		}
	}
}

func TestDuplicate_AppendsCopySuffixAndGeneratesNewID(t *testing.T) {
	st := &memStore{profiles: []model.ConnectionProfile{{ID: "a", Name: "Box", Type: model.ConnUnix, Endpoint: "unix:///x.sock"}}}
	m := NewManager(st, nil)
	d1, err := m.Duplicate(context.Background(), "a")
	if err != nil || d1.ID == "a" || d1.Name != "Box copy" {
		t.Fatalf("%+v %v", d1, err)
	}
	if d2, _ := m.Duplicate(context.Background(), "a"); d2.Name != "Box copy 2" {
		t.Fatalf("second copy name: %q", d2.Name)
	}
}
