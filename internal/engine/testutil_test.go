package engine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/model"
)

type fakeDaemon struct {
	srv         *httptest.Server
	eventConns  atomic.Int32 // open /events streams
	eventOpens  atomic.Int32 // total /events requests
	events      chan struct{}
	serverVer   string
	mu          sync.Mutex
	streamEnded chan struct{}
	done        chan struct{} // closed by close(): every /events stream, also one opened later, ends
}

func newFakeDaemon(ver string) *fakeDaemon {
	d := &fakeDaemon{events: make(chan struct{}, 16), serverVer: ver, streamEnded: make(chan struct{}, 16), done: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("API-Version", "1.45")
		w.Header().Set("OSType", "linux")
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/v1.45/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ServerVersion":"` + d.serverVer + `","OSType":"linux","Architecture":"x86_64","NCPU":4,"MemTotal":1024,"Containers":3,"Images":2}`))
	})
	mux.HandleFunc("/v1.45/volumes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Volumes":[{"Name":"v"}]}`))
	})
	mux.HandleFunc("/v1.45/networks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"Name":"a"},{"Name":"b"}]`))
	})
	mux.HandleFunc("/v1.45/events", func(w http.ResponseWriter, r *http.Request) {
		d.eventOpens.Add(1)
		d.eventConns.Add(1)
		defer func() { d.eventConns.Add(-1); d.streamEnded <- struct{}{} }()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-d.done:
				return
			case <-d.events:
				w.Write([]byte(`{"Type":"container","Action":"start","Actor":{"ID":"abc"}}` + "\n"))
				w.(http.Flusher).Flush()
			}
		}
	})
	d.srv = httptest.NewServer(mux)
	return d
}

type recEmitter struct {
	mu   sync.Mutex
	got  []string
	data []any
}

func (r *recEmitter) Emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, name)
	r.data = append(r.data, data)
}

func (r *recEmitter) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, g := range r.got {
		if g == name {
			n++
		}
	}
	return n
}

type memStore struct{ profiles []model.ConnectionProfile }

func (s *memStore) Load() ([]model.ConnectionProfile, error) { return s.profiles, nil }

func (s *memStore) Save(p []model.ConnectionProfile) error { s.profiles = p; return nil }

func testManager(t *testing.T, daemons map[string]*fakeDaemon) (*Manager, *recEmitter) {
	t.Helper()
	st := &memStore{}
	byEndpoint := map[string]*fakeDaemon{}
	for id, d := range daemons {
		st.profiles = append(st.profiles, model.ConnectionProfile{ID: id, Name: id, Type: model.ConnTCP, Endpoint: "tcp://" + strings.TrimPrefix(d.srv.URL, "http://")})
		byEndpoint["tcp://"+strings.TrimPrefix(d.srv.URL, "http://")] = d
	}
	em := &recEmitter{}
	m := NewManager(st, em)
	m.debounce = 30 * time.Millisecond
	m.backoffMin, m.backoffMax = 10*time.Millisecond, 20*time.Millisecond
	m.newClient = func(p model.ConnectionProfile) (*client.Client, error) {
		return client.New(client.WithHost(p.Endpoint), client.WithAPIVersionNegotiation())
	}
	t.Cleanup(m.Shutdown)
	return m, em
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

// close shuts the fake daemon down. Event streams end first: httptest's Close waits for active
// requests, and a stream held by a still-connected Manager would make it wait forever. Cutting the
// connections alone is not enough, because the Manager reconnects within milliseconds and the new
// stream would be active when Close starts waiting; done ends that one too.
func (d *fakeDaemon) close() {
	close(d.done)
	d.srv.CloseClientConnections()
	d.srv.Close()
}
