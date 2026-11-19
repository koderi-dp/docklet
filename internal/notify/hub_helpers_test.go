package notify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"

	"docklet/internal/model"
)

// memAlertStore is an in-memory AlertStore that records calls and can fail on demand.
type memAlertStore struct {
	mu       sync.Mutex
	alerts   []model.Alert // newest first
	addErr   error
	markErr  error
	markIDs  [][]string
	markAll  int
	clears   int
	lastList int
}

func (s *memAlertStore) AddAlert(_ context.Context, a model.Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.addErr != nil {
		return s.addErr
	}
	s.alerts = append([]model.Alert{a}, s.alerts...)
	return nil
}

func (s *memAlertStore) ListAlerts(_ context.Context, limit int) ([]model.Alert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastList = limit
	return append([]model.Alert{}, s.alerts...), nil
}

func (s *memAlertStore) MarkAlertsRead(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markIDs = append(s.markIDs, ids)
	return s.markErr
}

func (s *memAlertStore) MarkAllAlertsRead(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markAll++
	return s.markErr
}

func (s *memAlertStore) ClearAlerts(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clears++
	return s.markErr
}

func (s *memAlertStore) stored() []model.Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.Alert{}, s.alerts...)
}

type recEmitter struct {
	mu    sync.Mutex
	names []string
	data  []any
}

func (e *recEmitter) Emit(name string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.names = append(e.names, name)
	e.data = append(e.data, data)
}

func (e *recEmitter) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.names)
}

type osCall struct{ title, body string }

// fakeOS stands in for the desktop notifier; calls arrive on a channel because osNotify is asynchronous.
type fakeOS struct {
	calls chan osCall
	err   error
	boom  bool
}

func newFakeOS() *fakeOS { return &fakeOS{calls: make(chan osCall, 16)} }

func (o *fakeOS) Notify(title, body string) error {
	if o.boom {
		panic("notifier exploded")
	}
	o.calls <- osCall{title, body}
	return o.err
}

func (o *fakeOS) next(t *testing.T) osCall {
	t.Helper()
	select {
	case c := <-o.calls:
		return c
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for an OS notification")
		return osCall{}
	}
}

type fakeProfiles map[string]string

func (p fakeProfiles) Profile(id string) (model.ConnectionProfile, error) {
	if n, ok := p[id]; ok {
		return model.ConnectionProfile{ID: id, Name: n}, nil
	}
	return model.ConnectionProfile{}, errors.New("unknown connection")
}

type failingProvider struct{}

func (failingProvider) Client(context.Context, string) (*client.Client, error) {
	return nil, errors.New("not connected")
}

// inspectProvider serves the given container inspect JSON for every container.
func inspectProvider(t *testing.T, inspectJSON string) oneClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(inspectJSON))
	}))
	t.Cleanup(srv.Close)
	cli, err := client.New(client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")), client.WithAPIVersion("1.44"))
	if err != nil {
		t.Fatal(err)
	}
	return oneClient{cli}
}

// flowHub is a hub whose worker the test controls: events are processed on a goroutine that ends when
// finish closes the queue, so "nothing was recorded" can be asserted without sleeping.
type flowHub struct {
	*Hub
	done chan struct{}
}

func newFlowHub(d Deps, s model.NotificationSettings) *flowHub {
	if d.Now == nil {
		fixed := time.Unix(1_700_000_000, 0)
		d.Now = func() time.Time { return fixed }
	}
	h := &Hub{d: d, rules: NewRules(d.Now), in: make(chan Event, queueSize), settings: normalize(s)}
	f := &flowHub{Hub: h, done: make(chan struct{})}
	go func() { defer close(f.done); h.run() }()
	return f
}

// finish stops accepting events and waits until every queued event has been fully processed.
func (f *flowHub) finish(t *testing.T) {
	t.Helper()
	close(f.in)
	select {
	case <-f.done:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not drain the queue")
	}
}

func containerEvent(action, name string, attrs map[string]string) events.Message {
	a := map[string]string{"name": name}
	for k, v := range attrs {
		a[k] = v
	}
	return events.Message{Type: events.ContainerEventType, Action: events.Action(action), Actor: events.Actor{ID: "id-" + name, Attributes: a}}
}

// inspectProviderStatus answers every request with the given HTTP error status.
func inspectProviderStatus(t *testing.T, status int) oneClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"no such container"}`, status)
	}))
	t.Cleanup(srv.Close)
	cli, err := client.New(client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")), client.WithAPIVersion("1.44"))
	if err != nil {
		t.Fatal(err)
	}
	return oneClient{cli}
}
