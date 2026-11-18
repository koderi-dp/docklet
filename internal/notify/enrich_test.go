package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moby/moby/client"
)

type oneClient struct{ cli *client.Client }

func (o oneClient) Client(context.Context, string) (*client.Client, error) { return o.cli, nil }

func hubWithInspect(t *testing.T, state string) *Hub {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":"abc","Name":"/web","State":` + state + `}`))
	}))
	t.Cleanup(srv.Close)
	cli, err := client.New(client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")), client.WithAPIVersion("1.44"))
	if err != nil {
		t.Fatal(err)
	}
	return &Hub{d: Deps{Engines: oneClient{cli}}}
}

// Podman sends "health_status" without the verdict; the container's own health status supplies it.
func TestEnrich_PodmanHealthEventGetsItsVerdict(t *testing.T) {
	h := hubWithInspect(t, `{"Status":"running","Running":true,"Health":{"Status":"unhealthy","FailingStreak":3}}`)
	got := h.enrich(Event{ConnectionID: "c", ContainerID: "abc", Action: "health_status"})
	if len(got) != 1 || got[0].Action != "health_status: unhealthy" {
		t.Fatalf("got %+v", got)
	}
	// A container without a healthcheck yields nothing to report.
	h = hubWithInspect(t, `{"Status":"running","Running":true}`)
	if got := h.enrich(Event{Action: "health_status", ContainerID: "abc"}); len(got) != 0 {
		t.Fatalf("no health data, no event: %+v", got)
	}
}

// Podman reports an out-of-memory kill only as a die with exit code 137.
func TestEnrich_DieWithOOMKilledGetsAnOOMEventFirst(t *testing.T) {
	h := hubWithInspect(t, `{"Status":"exited","OOMKilled":true,"ExitCode":137}`)
	got := h.enrich(Event{Action: "die", ExitCode: "137", ContainerID: "abc"})
	if len(got) != 2 || got[0].Action != "oom" || got[1].Action != "die" {
		t.Fatalf("got %+v", got)
	}
	h = hubWithInspect(t, `{"Status":"exited","OOMKilled":false,"ExitCode":1}`)
	if got := h.enrich(Event{Action: "die", ExitCode: "1", ContainerID: "abc"}); len(got) != 1 || got[0].Action != "die" {
		t.Fatalf("an ordinary crash stays a crash: %+v", got)
	}
	if got := h.enrich(Event{Action: "die", ExitCode: "0", ContainerID: "abc"}); len(got) != 1 {
		t.Fatalf("a clean exit passes through: %+v", got)
	}
}

func TestEnrich_DockerStyleEventsPassThroughUntouched(t *testing.T) {
	h := &Hub{}
	for _, a := range []string{"health_status: unhealthy", "oom", "start", "die"} {
		if got := h.enrich(Event{Action: a, ExitCode: "1"}); len(got) != 1 || got[0].Action != a {
			t.Errorf("%s changed: %+v", a, got)
		}
	}
}
