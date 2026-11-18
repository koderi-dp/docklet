package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverSockets_OffersOnlySocketsThatExist(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	present := map[string]bool{
		filepath.Join(xdg, "docker.sock"):                        true,
		filepath.Join(home, ".colima", "default", "docker.sock"): true,
	}
	got := discoverSockets(home, xdg, func(p string) bool { return present[p] })
	if len(got) != 2 || got[0].ID != "local-rootless" || got[1].ID != "local-colima" {
		t.Fatalf("unexpected discovery: %+v", got)
	}
	for _, p := range got {
		if !p.Discovered || !strings.HasPrefix(p.Endpoint, "unix://") {
			t.Errorf("profile must be a discovered unix endpoint: %+v", p)
		}
	}
}

func TestDiscoverSockets_NothingAssumedWhenNothingExists(t *testing.T) {
	if got := discoverSockets(t.TempDir(), "", func(string) bool { return false }); len(got) != 0 {
		t.Fatalf("no sockets must yield no profiles: %+v", got)
	}
}

func TestIsSocket_RejectsRegularFilesAndMissingPaths(t *testing.T) {
	f := filepath.Join(t.TempDir(), "docker.sock")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if isSocket(f) || isSocket(f+".missing") {
		t.Fatal("only real sockets count")
	}
}
