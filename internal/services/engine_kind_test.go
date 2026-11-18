package services

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"docklet/internal/model"
)

// The /version answer of a Podman 6 machine (trimmed): the Engine names itself in Components.
const podmanVersionJSON = `{"Platform":{"Name":"linux/amd64/fedora-44"},"Version":"6.1.4","ApiVersion":"1.44","MinAPIVersion":"1.24","Os":"linux","Arch":"amd64",
"Components":[{"Name":"Podman Engine","Version":"6.1.4"},{"Name":"Conmon","Version":"conmon version 2.2.1"},{"Name":"Engine","Version":"6.1.4"}]}`

func versionHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/version") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	})
}

func TestEngineKindOf_ReadsPodmanFromVersion(t *testing.T) {
	cli := dockerClient(t, versionHandler(podmanVersionJSON), "1.44")
	if got := engineKindOf(context.Background(), cli); got != model.EnginePodman {
		t.Fatalf("want podman, got %q", got)
	}
	docker := dockerClient(t, versionHandler(`{"Version":"29.1.0","ApiVersion":"1.52","Components":[{"Name":"Engine","Version":"29.1.0"}]}`), "1.44")
	if got := engineKindOf(context.Background(), docker); got != model.EngineDocker {
		t.Fatalf("want docker, got %q", got)
	}
}

func TestBuildCapability_PodmanNeverOffersBuildKit(t *testing.T) {
	cli := dockerClient(t, versionHandler(podmanVersionJSON), "1.44")
	r := &cliRunner{outputs: map[string]string{"buildx version": "github.com/docker/buildx v0.19.2 abc\n"}}
	s := newBuildSvc(r)
	s.Engines = fakeProvider{cli: cli}
	s.Profiles = profiles{"c1": {ID: "c1", Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/podman-machine-default"}}
	c, err := s.BuildCapability(context.Background(), "c1")
	if err != nil || c.Available || !strings.Contains(c.Summary, "Classic builder") || !strings.Contains(c.Reason, "buildx") {
		t.Fatalf("%+v %v", c, err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("the docker CLI must not even be probed for Podman: %+v", r.calls)
	}
}
