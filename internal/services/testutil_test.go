package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

// fakeProvider hands out one live client for connection "c1"; any other ID is unknown.
type fakeProvider struct{ cli *client.Client }

func (f fakeProvider) Client(_ context.Context, id string) (*client.Client, error) {
	if id != "c1" {
		return nil, errors.New("unknown connection " + id)
	}
	return f.cli, nil
}

// dockerClient returns an SDK client wired to a throwaway httptest server running h.
func dockerClient(t *testing.T, h http.Handler, apiVersion string) *client.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cli, err := client.New(client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")), client.WithAPIVersion(apiVersion))
	if err != nil {
		t.Fatal(err)
	}
	return cli
}

type emitted struct {
	name string
	data any
}

// recEmitter records every event a service emits.
type recEmitter struct {
	mu  sync.Mutex
	evs []emitted
}

func (e *recEmitter) Emit(name string, data any) {
	e.mu.Lock()
	e.evs = append(e.evs, emitted{name, data})
	e.mu.Unlock()
}

func (e *recEmitter) snapshot() []emitted {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]emitted(nil), e.evs...)
}

func (e *recEmitter) names() []string {
	var out []string
	for _, ev := range e.snapshot() {
		out = append(out, ev.name)
	}
	return out
}

func (e *recEmitter) find(name string) []any {
	var out []any
	for _, ev := range e.snapshot() {
		if ev.name == name {
			out = append(out, ev.data)
		}
	}
	return out
}

// counter tracks how many requests a fake server currently has open.
type counter struct {
	mu sync.Mutex
	n  int
}

func (c *counter) add(d int) { c.mu.Lock(); c.n += d; c.mu.Unlock() }
func (c *counter) get() int  { c.mu.Lock(); defer c.mu.Unlock(); return c.n }

// eventually polls cond until it holds, failing the test after a deadline.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
