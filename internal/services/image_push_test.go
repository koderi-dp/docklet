package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/jsonstream"

	"docklet/internal/registryauth"
)

func TestConsumePush_ReportsProgressUntilComplete(t *testing.T) {
	a, b := "aaaaaaaaaaaa", "bbbbbbbbbbbb"
	msgs := seq([]jsonstream.Message{
		dl(a, "Preparing", 0, 0), dl(b, "Preparing", 0, 0),
		dl(a, "Layer already exists", 0, 0),
		dl(b, "Pushing", 50, 100),
		dl(b, "Pushing", 100, 100),
		dl(b, "Pushed", 0, 0),
		{Status: "1: digest: sha256:abc size: 527"},
	})
	s := &sink{}
	if err := consumePush(context.Background(), msgs, s); err != nil {
		t.Fatal(err)
	}
	if len(s.progress) == 0 || s.progress[len(s.progress)-1] != 1 {
		t.Errorf("progress %v", s.progress)
	}
	if !strings.Contains(strings.Join(s.logs, "|"), "digest: sha256:abc") {
		t.Errorf("logs %v", s.logs)
	}
}

func TestRegistryAuthError_ExplainsMissingOrRejectedCredentials(t *testing.T) {
	denied := errors.New("denied: requested access to the resource is denied")
	none := registryauth.Credentials{Registry: "ghcr.io"}
	want := "No credentials for ghcr.io. Run `docker login ghcr.io` in a terminal, then retry."
	if got := registryAuthError(denied, none, "ghcr.io/o/a:1", false); got.Error() != want {
		t.Errorf("got %q", got)
	}
	have := registryauth.Credentials{Registry: "ghcr.io", Source: "config", Password: "hunter2"}
	if g := registryAuthError(denied, have, "ghcr.io/o/a:1", false); !strings.Contains(g.Error(), "rejected the saved credentials") || strings.Contains(g.Error(), "hunter2") {
		t.Errorf("got %q", g)
	}
	if registryAuthError(errors.New("boom"), none, "ghcr.io/a", false).Error() != "boom" {
		t.Error("non-auth errors pass through")
	}
}

func TestConsumePush_StopsOnRegistryError(t *testing.T) {
	err := consumePush(context.Background(), seq([]jsonstream.Message{{Error: &jsonstream.Error{Message: "denied: requested access"}}}), &sink{})
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("want denied error, got %v", err)
	}
}
