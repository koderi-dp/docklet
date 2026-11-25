package services

import (
	"net/http"
	"slices"
	"testing"

	"docklet/internal/appdb"
	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/registryauth"
)

// newVerbSvc returns a ComposeService with one managed project "shop" on connection c1
// and a fake CLI runner, so verbs can be driven end to end without Docker.
func newVerbSvc(t *testing.T) (*ComposeService, *fakeRunner, *ops.Manager) {
	t.Helper()
	dir := t.TempDir()
	cfg := writeFile(t, dir, "compose.yml")
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}), "1.45")
	runner := &fakeRunner{}
	m := ops.New(nil)
	s := &ComposeService{
		Engines: fakeProvider{cli}, Events: &recEmitter{}, Ops: m, runner: runner,
		Projects: &fakeStore{recs: []appdb.ComposeProjectRecord{{ConnectionID: "c1", Name: "shop", WorkingDir: dir, ConfigFiles: []string{cfg}}}},
		Profiles: fakeProfiles{"c1": {ID: "c1", Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:2375"}},
		// Never the sign-ins of the machine the tests run on.
		Registry: &registryauth.Resolver{ConfigDir: dir},
	}
	return s, runner, m
}

// composeArgs returns the argv of the CLI invocation that ran the given verb.
func composeArgs(t *testing.T, r *fakeRunner, verb string) []string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.cmds {
		if slices.Contains(c.Args, verb) {
			return c.Args
		}
	}
	t.Fatalf("no %q invocation among %+v", verb, r.cmds)
	return nil
}

func TestComposeDown_DoesNotRemoveVolumesUnlessExplicitlyRequested(t *testing.T) {
	for _, tc := range []struct {
		name        string
		opts        model.ComposeDownOptions
		wantVolumes bool
	}{
		{"default", model.ComposeDownOptions{}, false},
		{"remove orphans only", model.ComposeDownOptions{RemoveOrphans: true}, false},
		{"explicit volumes", model.ComposeDownOptions{Volumes: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, runner, m := newVerbSvc(t)
			op, err := s.Down(bg, "c1", "shop", tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded {
				t.Fatalf("operation: %+v", done)
			}
			if got := slices.Contains(composeArgs(t, runner, "down"), "--volumes"); got != tc.wantVolumes {
				t.Errorf("--volumes present = %v, want %v", got, tc.wantVolumes)
			}
		})
	}
}

func TestComposeStop_UsesStopNotDown(t *testing.T) {
	s, runner, m := newVerbSvc(t)
	op, err := s.Stop(bg, "c1", "shop")
	if err != nil {
		t.Fatal(err)
	}
	waitOp(t, m, op.ID)
	if args := composeArgs(t, runner, "stop"); slices.Contains(args, "down") {
		t.Errorf("stop must never run down: %v", args)
	}
}

func TestComposeVerb_RejectsInvalidProjectNameWithoutRunningCLI(t *testing.T) {
	s, runner, _ := newVerbSvc(t)
	if _, err := s.Up(bg, "c1", "x; rm -rf /", model.ComposeUpOptions{}); err == nil {
		t.Fatal("injection-like project name accepted")
	}
	if runner.calls != 0 {
		t.Fatalf("CLI ran %d times for an invalid project", runner.calls)
	}
}
