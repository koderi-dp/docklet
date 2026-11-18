package compose

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func fixtureLines(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func kinds(lines []string) []WatchKind {
	var out []WatchKind
	for _, l := range lines {
		if k := ParseWatchLine(l).Kind; k != WatchRaw {
			out = append(out, k)
		}
	}
	return out
}

// Real `docker compose watch` output (Compose 5.7.0): initial build + up, a sync, a sync of two
// changes and a rebuild with its BuildKit output. Only the meaningful lines may be classified.
func TestParseWatchLine_RealSyncAndRebuildOutput(t *testing.T) {
	lines := fixtureLines(t, "watch_sync_rebuild.txt")
	want := []WatchKind{WatchEnabled, WatchSync, WatchSync, WatchRebuilding, WatchRebuilt}
	if got := kinds(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	if p := ParseWatchLine(`Syncing service "web" after 2 changes were detected`); p.Count != 2 || !reflect.DeepEqual(p.Services, []string{"web"}) {
		t.Errorf("sync: %+v", p)
	}
	if p := ParseWatchLine(`Rebuilding service(s) ["api"] after changes were detected...`); !reflect.DeepEqual(p.Services, []string{"api"}) {
		t.Errorf("rebuilding: %+v", p)
	}
	if p := ParseWatchLine(`service(s) ["api"] successfully built`); p.Kind != WatchRebuilt || !reflect.DeepEqual(p.Services, []string{"api"}) {
		t.Errorf("built: %+v", p)
	}
}

func TestParseWatchLine_RealRestartOutput(t *testing.T) {
	lines := fixtureLines(t, "watch_restart.txt")
	want := []WatchKind{WatchEnabled, WatchSync, WatchRestarted, WatchSync, WatchRestarted}
	if got := kinds(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
}

func TestParseWatchLine_ErrorsAreErrorsAndStatusAndBuildNoiseIsNot(t *testing.T) {
	for _, l := range fixtureLines(t, "watch_errors.txt") {
		if k := ParseWatchLine(l).Kind; k != WatchError {
			t.Errorf("%q classified %s, want error", l, k)
		}
	}
	for _, l := range []string{
		" Container docklet-test-watch-web-1 Restarting",
		" Container my-error-handler-1 Started",
		" Image x Built",
		"#7 [2/2] COPY . /app",
		"#8 exporting layers 0.0s done",
		"#3 [internal] load metadata for docker.io/library/busybox:1.36",
		"",
	} {
		if k := ParseWatchLine(l).Kind; k != WatchRaw {
			t.Errorf("%q classified %s, want raw", l, k)
		}
	}
}

func TestParseWatchLine_ToleratesOlderWordingAndMultipleServices(t *testing.T) {
	if p := ParseWatchLine(`Rebuilding service "api" after changes were detected...`); p.Kind != WatchRebuilding || !reflect.DeepEqual(p.Services, []string{"api"}) {
		t.Errorf("single-service wording: %+v", p)
	}
	if p := ParseWatchLine(`service(s) ["api" "web"] successfully built`); !reflect.DeepEqual(p.Services, []string{"api", "web"}) {
		t.Errorf("two services: %+v", p)
	}
	if p := ParseWatchLine(`Syncing service "web" after 1 change was detected`); p.Kind != WatchSync || p.Count != 1 {
		t.Errorf("singular: %+v", p)
	}
	if p := ParseWatchLine("Watch enabled"); p.Kind != WatchEnabled {
		t.Errorf("enabled: %+v", p)
	}
}
