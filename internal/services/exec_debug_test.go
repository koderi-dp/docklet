package services

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"

	"docklet/internal/model"
)

func TestBuildDebugSpec_JoinsNamespacesWithoutExtraPrivileges(t *testing.T) {
	cfg, host := buildDebugSpec("abc123", debugSpecOptions{Image: "busybox:latest"})
	if string(host.PidMode) != "container:abc123" || string(host.NetworkMode) != "container:abc123" {
		t.Fatalf("pid=%q net=%q", host.PidMode, host.NetworkMode)
	}
	if host.IpcMode != "" {
		t.Fatalf("IPC must only be shared when asked, got %q", host.IpcMode)
	}
	if host.Privileged || len(host.CapAdd) != 0 || !host.AutoRemove {
		t.Fatalf("privileged=%v caps=%v autoremove=%v", host.Privileged, host.CapAdd, host.AutoRemove)
	}
	if cfg.Labels[debugLabel] != "true" || cfg.Labels[debugTargetLabel] != "abc123" || !cfg.Tty || !cfg.OpenStdin {
		t.Fatalf("cfg = %+v", cfg)
	}

	_, host = buildDebugSpec("abc123", debugSpecOptions{ShareIPC: true, SysPtrace: true})
	if string(host.IpcMode) != "container:abc123" || len(host.CapAdd) != 1 || host.CapAdd[0] != "SYS_PTRACE" || host.Privileged {
		t.Fatalf("host = %+v", host)
	}
}

func TestStaleDebugContainers_OnlyOldUnownedWithOurLabel(t *testing.T) {
	now := time.Unix(10_000, 0)
	old := now.Add(-time.Hour).Unix()
	mine := map[string]string{debugLabel: "true"}
	list := []debugInfo{
		{ID: "stale", Created: old, Labels: mine},
		{ID: "fresh", Created: now.Add(-time.Minute).Unix(), Labels: mine},
		{ID: "live", Created: old, Labels: mine},
		{ID: "foreign", Created: old, Labels: map[string]string{"docklet.helper": "1"}},
		{ID: "unlabelled", Created: old},
	}
	got := staleDebugContainers(list, func(id string) bool { return id == "live" }, now, debugStaleAge)
	if len(got) != 1 || got[0] != "stale" {
		t.Fatalf("got %v", got)
	}
}

type fakeDebugAPI struct {
	mu          sync.Mutex
	os          string
	target      debugTarget
	imageThere  bool
	createErrs  []error // consumed one per CreateSidecar call
	startErr    error
	created     []*container.HostConfig
	createNames []string
	removed     []string
	listed      []debugInfo
}

func (f *fakeDebugAPI) EngineOS(context.Context) (string, error) { return f.os, nil }
func (f *fakeDebugAPI) InspectTarget(context.Context, string) (debugTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.target, nil
}
func (f *fakeDebugAPI) ImageExists(context.Context, string) (bool, error) { return f.imageThere, nil }
func (f *fakeDebugAPI) CreateSidecar(_ context.Context, name string, _ *container.Config, h *container.HostConfig) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, h)
	f.createNames = append(f.createNames, name)
	if len(f.createErrs) > 0 {
		err := f.createErrs[0]
		f.createErrs = f.createErrs[1:]
		if err != nil {
			return "", err
		}
	}
	return "sidecar1", nil
}
func (f *fakeDebugAPI) StartContainer(context.Context, string) error { return f.startErr }
func (f *fakeDebugAPI) RemoveContainer(_ context.Context, id string) error {
	f.mu.Lock()
	f.removed = append(f.removed, id)
	f.mu.Unlock()
	return nil
}
func (f *fakeDebugAPI) ListDebug(context.Context) ([]debugInfo, error) { return f.listed, nil }

func (f *fakeDebugAPI) removedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.removed...)
}

func newDebugSvc(d *fakeDebugAPI, x *fakeExecAPI) (*ExecService, *recEmitter) {
	s, em := newExecSvc(x)
	s.newDebugAPI = func(context.Context, string) (debugAPI, error) { return d, nil }
	return s, em
}

func runningTarget() debugTarget {
	return debugTarget{ID: "0123456789abcdef0123", Name: "web", Running: true, IPCMode: "private"}
}

func TestDebugShellStart_RefusesWhereItCannotWork(t *testing.T) {
	ctx := context.Background()
	x := &fakeExecAPI{running: true, stream: newPipeStream()}

	win := &fakeDebugAPI{os: "windows", target: runningTarget(), imageThere: true}
	s, _ := newDebugSvc(win, x)
	if _, err := s.DebugShellStart(ctx, "c1", "web", model.DebugShellOptions{}); err == nil || !strings.Contains(err.Error(), "Windows") {
		t.Fatalf("windows: %v", err)
	}

	stopped := runningTarget()
	stopped.Running = false
	s, _ = newDebugSvc(&fakeDebugAPI{os: "linux", target: stopped, imageThere: true}, x)
	if _, err := s.DebugShellStart(ctx, "c1", "web", model.DebugShellOptions{}); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("stopped: %v", err)
	}
}

func TestDebugShellStart_MissingImageCreatesNothing(t *testing.T) {
	d := &fakeDebugAPI{os: "linux", target: runningTarget(), imageThere: false}
	s, _ := newDebugSvc(d, &fakeExecAPI{running: true, stream: newPipeStream()})
	res, err := s.DebugShellStart(context.Background(), "c1", "web", model.DebugShellOptions{Image: "alpine:latest"})
	if err != nil || !res.ImageMissing || res.Image != "alpine:latest" || res.SessionID != "" || len(d.created) != 0 {
		t.Fatalf("res=%+v err=%v created=%d", res, err, len(d.created))
	}
}

func TestDebugShellStart_FallsBackWithoutIPCAndCloseRemovesSidecar(t *testing.T) {
	ctx := context.Background()
	tg := runningTarget()
	tg.IPCMode = "shareable"
	d := &fakeDebugAPI{os: "linux", target: tg, imageThere: true, createErrs: []error{errors.New("cannot join IPC of a non-shareable container")}}
	x := &fakeExecAPI{running: true, stream: newPipeStream()}
	s, _ := newDebugSvc(d, x)

	res, err := s.DebugShellStart(ctx, "c1", "web", model.DebugShellOptions{SysPtrace: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.created) != 2 || d.created[0].IpcMode == "" || d.created[1].IpcMode != "" {
		t.Fatalf("create attempts: %+v", d.created)
	}
	if res.SharesIPC || len(res.Notices) == 0 || res.SessionID == "" || !strings.HasPrefix(res.SidecarName, "docklet-debug-0123456789ab-") {
		t.Fatalf("res = %+v", res)
	}
	if !s.debug.isLive("c1", "sidecar1") {
		t.Fatal("sidecar must be tracked as live")
	}
	if err := s.Close(ctx, res.SessionID); err != nil {
		t.Fatal(err)
	}
	if got := d.removedIDs(); len(got) != 1 || got[0] != "sidecar1" || s.debug.isLive("c1", "sidecar1") {
		t.Fatalf("removed=%v", got)
	}
}

func TestDebugShell_TargetStoppingEndsSessionRemovesSidecarAndExplains(t *testing.T) {
	ctx := context.Background()
	d := &fakeDebugAPI{os: "linux", target: runningTarget(), imageThere: true}
	x := &fakeExecAPI{running: true, stream: newPipeStream()}
	s, em := newDebugSvc(d, x)
	if _, err := s.DebugShellStart(ctx, "c1", "web", model.DebugShellOptions{}); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.target.Running = false // the target stops: the kernel ends the shared PID namespace
	d.mu.Unlock()
	_ = x.stream.w.Close()
	eventually(t, "exit event", func() bool { return len(em.find("exec:exit")) == 1 })
	ev := em.find("exec:exit")[0].(model.ExecExitEvent)
	if !strings.Contains(ev.Notice, "web stopped") {
		t.Fatalf("notice = %q", ev.Notice)
	}
	if got := d.removedIDs(); len(got) != 1 || got[0] != "sidecar1" {
		t.Fatalf("removed=%v", got)
	}
}

func TestDebugShellStart_FailedStartRemovesSidecar(t *testing.T) {
	d := &fakeDebugAPI{os: "linux", target: runningTarget(), imageThere: true, startErr: errors.New("boom")}
	s, _ := newDebugSvc(d, &fakeExecAPI{running: true, stream: newPipeStream()})
	if _, err := s.DebugShellStart(context.Background(), "c1", "web", model.DebugShellOptions{}); err == nil {
		t.Fatal("want error")
	}
	if got := d.removedIDs(); len(got) != 1 {
		t.Fatalf("removed=%v", got)
	}
}

func TestSweepDebug_RemovesOnlyStaleUnownedAndIsThrottled(t *testing.T) {
	mine := map[string]string{debugLabel: "true"}
	old := time.Now().Add(-time.Hour).Unix()
	d := &fakeDebugAPI{listed: []debugInfo{
		{ID: "left", Created: old, Labels: mine},
		{ID: "owned", Created: old, Labels: mine},
		{ID: "other", Created: old, Labels: map[string]string{"x": "y"}},
	}}
	s, _ := newDebugSvc(d, &fakeExecAPI{})
	s.debug.add("c1", "owned")
	if n, err := s.SweepDebugContainers(context.Background(), "c1"); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got := d.removedIDs(); len(got) != 1 || got[0] != "left" {
		t.Fatalf("removed=%v", got)
	}
	if n, _ := s.SweepDebugContainers(context.Background(), "c1"); n != 0 {
		t.Fatal("second sweep within the throttle window must be a no-op")
	}
}

func TestExecStart_AutoReportsNoShellWhenEveryCandidateExits127(t *testing.T) {
	s, _ := newExecSvc(&fakeExecAPI{running: true, stream: newPipeStream(), exitCode: 127})
	_, err := s.Start(context.Background(), "c1", "ctr", model.ExecOptions{})
	if err == nil || !strings.Contains(err.Error(), "No usable shell found") {
		t.Fatalf("got %v", err)
	}
	if s.count() != 0 {
		t.Fatal("no session may be registered")
	}
}
