package services

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"docklet/internal/model"
)

func TestFillPodmanHealth_ReadsHealthFromInspectAndRemembersContainersWithoutOne(t *testing.T) {
	var inspects atomic.Int32
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inspects.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/hc/json"):
			_, _ = w.Write([]byte(`{"Id":"hc","State":{"Status":"running","Running":true,"Health":{"Status":"unhealthy"}},"Config":{"Healthcheck":{"Test":["CMD-SHELL","false"]}}}`))
		default:
			_, _ = w.Write([]byte(`{"Id":"plain","State":{"Status":"running","Running":true},"Config":{}}`))
		}
	}), "1.44")
	list := func() []model.Container {
		return []model.Container{{ID: "hc", State: "running", Health: model.HealthNone}, {ID: "plain", State: "running", Health: model.HealthNone}, {ID: "off", State: "exited", Health: model.HealthNone}}
	}
	got := list()
	fillPodmanHealth(context.Background(), cli, got)
	if got[0].Health != model.HealthUnhealthy || got[1].Health != model.HealthNone || got[2].Health != model.HealthNone {
		t.Fatalf("health: %+v", got)
	}
	if inspects.Load() != 2 {
		t.Fatalf("only running containers are inspected, got %d calls", inspects.Load())
	}
	got = list()
	fillPodmanHealth(context.Background(), cli, got)
	if inspects.Load() != 3 {
		t.Fatalf("a container known to have no healthcheck must not be inspected again (calls %d)", inspects.Load())
	}
	if got[0].Health != model.HealthUnhealthy {
		t.Fatalf("a container with a healthcheck is re-read: %+v", got[0])
	}
}
