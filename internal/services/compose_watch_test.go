package services

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
	"docklet/internal/ops"
)

const watchConfigJSON = `{"services":{
  "web":{"develop":{"watch":[{"path":"/p/site","action":"sync","target":"/html"}]}},
  "api":{"develop":{"watch":[{"path":"/p/api","action":"rebuild"}]}},
  "cache":{"image":"busybox"}}}`

// watchRunner scripts the CLI: config/version answers and a long-running watch that prints lines
// and then blocks until it is cancelled (or exits with err when exitWith is set).
type watchRunner struct {
	fakeRunner
	config, version string
	lines           []string
	exitWith        error
	started         chan struct{}
}

func (w *watchRunner) Output(ctx context.Context, c compose.Command) (string, error) {
	_, _ = w.fakeRunner.Output(ctx, c)
	if slices.Contains(c.Args, "--short") {
		return w.version, nil
	}
	if slices.Contains(c.Args, "config") {
		return w.config, nil
	}
	return "", nil
}

func (w *watchRunner) Run(ctx context.Context, c compose.Command, onLine func(string)) error {
	_ = w.fakeRunner.Run(ctx, c, onLine)
	for _, l := range w.lines {
		onLine(l)
	}
	if w.started != nil {
		close(w.started)
	}
	if w.exitWith != nil {
		return w.exitWith
	}
	<-ctx.Done()
	return ctx.Err()
}

func newWatchSvc(t *testing.T, version string) (*ComposeService, *watchRunner, *ops.Manager) {
	t.Helper()
	dir := t.TempDir()
	cfg := writeFile(t, dir, "compose.yml")
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}), "1.45")
	runner := &watchRunner{config: watchConfigJSON, version: version, started: make(chan struct{})}
	m := ops.New(nil)
	s := &ComposeService{
		Engines: fakeProvider{cli}, Events: &recEmitter{}, Ops: m, runner: runner,
		Projects: &fakeStore{recs: []appdb.ComposeProjectRecord{{ConnectionID: "c1", Name: "shop", WorkingDir: dir, ConfigFiles: []string{cfg}}}},
		Profiles: fakeProfiles{"c1": {ID: "c1", Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:2375"}},
	}
	return s, runner, m
}

func TestWatchInfo_ListsRulesAndExplainsWhyNot(t *testing.T) {
	s, _, _ := newWatchSvc(t, "5.7.0\n")
	info, err := s.WatchInfo(bg, "c1", "shop")
	if err != nil || !info.Available || info.RuleCount != 2 || len(info.Services) != 2 || info.CLIVersion != "5.7.0" {
		t.Fatalf("%+v %v", info, err)
	}

	s, r, _ := newWatchSvc(t, "5.7.0")
	r.config = `{"services":{"a":{"image":"x"}}}`
	if info, _ := s.WatchInfo(bg, "c1", "shop"); info.Available || info.Reason != noWatchRulesReason {
		t.Errorf("no rules: %+v", info)
	}

	s, _, _ = newWatchSvc(t, "2.21.0")
	info, _ = s.WatchInfo(bg, "c1", "shop")
	if info.Available || !strings.Contains(info.Reason, "2.21.0") || !strings.Contains(info.Reason, "too old") {
		t.Errorf("old CLI: %+v", info)
	}
}

func TestStartWatch_RunsCLIWithOptionsParsesOutputAndStops(t *testing.T) {
	s, r, m := newWatchSvc(t, "5.7.0")
	r.lines = []string{
		"Watch enabled",
		`Syncing service "web" after 2 changes were detected`,
		`Rebuilding service(s) ["api"] after changes were detected...`,
		"#5 [internal] load build context",
		`service(s) ["api"] successfully built`,
	}
	op, err := s.StartWatch(bg, "c1", "shop", model.ComposeWatchOptions{Services: []string{"web"}, NoUp: true})
	if err != nil || op.Kind != "compose.watch" || !op.Cancellable {
		t.Fatalf("%+v %v", op, err)
	}
	<-r.started
	args := composeArgs(t, &r.fakeRunner, "watch")
	if n := len(args); args[n-1] != "web" || args[n-2] != "--no-up" {
		t.Errorf("argv: %v", args)
	}
	if _, err := s.StartWatch(bg, "c1", "shop", model.ComposeWatchOptions{}); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("second watch for the same project: %v", err)
	}

	st, _ := s.WatchState(bg, "c1", "shop")
	if !st.Running || !st.Ready || st.OperationID != op.ID || !st.NoUp {
		t.Errorf("state: %+v", st)
	}
	if len(st.Events) != 4 || st.Events[0].Kind != model.WatchEventRebuilt || !strings.HasPrefix(st.Events[0].Text, "Rebuilt") {
		t.Errorf("events (newest first): %+v", st.Events)
	}
	if len(st.Raw) != 1 || st.Raw[0] != "#5 [internal] load build context" {
		t.Errorf("raw: %+v", st.Raw)
	}
	counts := map[string]model.ComposeWatchStat{}
	for _, c := range st.Stats {
		counts[c.Service] = c
	}
	if counts["web"].Syncs != 1 || counts["api"].Rebuilds != 1 || counts["api"].Syncs != 0 {
		t.Errorf("stats: %+v", st.Stats)
	}

	if err := s.StopWatch(bg, "c1", "shop"); err != nil {
		t.Fatal(err)
	}
	if done := waitOp(t, m, op.ID); done.Status != model.OpCancelled {
		t.Errorf("operation: %+v", done)
	}
	if st, _ := s.WatchState(bg, "c1", "shop"); st.Active {
		t.Errorf("a stopped watch must disappear: %+v", st)
	}
}

func TestStartWatch_ProcessExitingOnItsOwnKeepsTheReason(t *testing.T) {
	s, r, m := newWatchSvc(t, "5.7.0")
	r.lines = []string{"Watch enabled", "Error response from daemon: connection lost"}
	r.exitWith = context.DeadlineExceeded
	op, err := s.StartWatch(bg, "c1", "shop", model.ComposeWatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitOp(t, m, op.ID); done.Status != model.OpFailed || !strings.Contains(done.Error, "connection lost") {
		t.Fatalf("operation: %+v", done)
	}
	st, _ := s.WatchState(bg, "c1", "shop")
	if !st.Active || st.Running || !strings.Contains(st.Exit, "connection lost") || st.EndedAt == 0 {
		t.Errorf("state: %+v", st)
	}
	// "Start again" works: the ended watch does not block a new one.
	r.exitWith, r.started = nil, make(chan struct{})
	if _, err := s.StartWatch(bg, "c1", "shop", model.ComposeWatchOptions{}); err != nil {
		t.Fatal(err)
	}
	<-r.started
	_ = s.StopWatch(bg, "c1", "shop")
}

func TestStartWatch_RejectsWhatCannotWatchWithoutRunningIt(t *testing.T) {
	s, r, _ := newWatchSvc(t, "5.7.0")
	if _, err := s.StartWatch(bg, "c1", "shop", model.ComposeWatchOptions{Services: []string{"cache"}}); err == nil || !strings.Contains(err.Error(), "no develop.watch rules") {
		t.Errorf("service without rules: %v", err)
	}
	if _, err := s.StartWatch(bg, "c1", "shop", model.ComposeWatchOptions{Services: []string{"x; rm -rf /"}}); err == nil {
		t.Error("injection-like service accepted")
	}
	r.config = `{"services":{}}`
	if _, err := s.StartWatch(bg, "c1", "shop", model.ComposeWatchOptions{}); err == nil {
		t.Error("watch started without rules")
	}
	if err := s.StopWatch(bg, "c1", "shop"); err == nil {
		t.Error("stop without a watch must report an error")
	}
	time.Sleep(10 * time.Millisecond)
	for _, c := range r.cmds {
		if slices.Contains(c.Args, "watch") {
			t.Errorf("watch CLI ran for a rejected request: %v", c.Args)
		}
	}
}

func TestStartWatch_ExitWithoutErrorLinesDoesNotPassOffLastOutputAsTheReason(t *testing.T) {
	s, r, m := newWatchSvc(t, "5.7.0")
	r.lines = []string{"Watch enabled"}
	r.exitWith = context.DeadlineExceeded
	op, err := s.StartWatch(bg, "c1", "shop", model.ComposeWatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	waitOp(t, m, op.ID)
	st, _ := s.WatchState(bg, "c1", "shop")
	if !strings.Contains(st.Exit, "ended unexpectedly") || !strings.Contains(st.Exit, "Last output: Watch enabled") {
		t.Errorf("exit: %q", st.Exit)
	}
}
