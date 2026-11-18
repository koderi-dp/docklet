package services

import (
	"context"
	"strings"
	"sync"
	"testing"

	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/wsldocker"
)

type wslFakeHost struct {
	mu      sync.Mutex
	profile model.ConnectionProfile
	distros []model.WSLDistro
	events  []string
}

func (h *wslFakeHost) note(s string)                                   { h.mu.Lock(); h.events = append(h.events, s); h.mu.Unlock() }
func (h *wslFakeHost) Profile(string) (model.ConnectionProfile, error) { return h.profile, nil }
func (h *wslFakeHost) WSLDistros(context.Context) ([]model.WSLDistro, error) {
	return h.distros, nil
}
func (h *wslFakeHost) StartWSLDistro(_ context.Context, d string) error {
	h.note("start " + d)
	return nil
}
func (h *wslFakeHost) Connect(context.Context, string) (model.ConnectionStatus, error) {
	h.note("connect")
	return model.ConnectionStatus{State: model.StateConnected}, nil
}
func (h *wslFakeHost) Disconnect(context.Context, string) error { h.note("disconnect"); return nil }

// wslFakeExec answers the inspection script like a running Ubuntu with systemd and Docker.
type wslFakeExec struct {
	host *wslFakeHost
	mu   sync.Mutex
	cmds []string
}

func (e *wslFakeExec) Run(_ context.Context, _ string, c wsldocker.Cmd, onLine func(string)) error {
	d := wsldocker.Display(c)
	e.mu.Lock()
	e.cmds = append(e.cmds, d)
	e.mu.Unlock()
	e.host.note("exec " + d)
	switch {
	case strings.HasPrefix(d, "sh -c") && strings.Contains(d, "@@server"): // wsldocker.Detect
		for _, l := range strings.Split(detectOutput, "\n") {
			onLine(l)
		}
	case strings.HasPrefix(d, "sh -c") && strings.Contains(d, "@@docker"):
		for _, l := range strings.Split("@@init\nsystemd\n@@docker\nrunning\n@@end", "\n") {
			onLine(l)
		}
	case d == "cat /proc/meminfo":
		onLine("MemTotal: 1000 kB")
	case strings.Contains(d, "command -v dockerd"):
		onLine("running")
	}
	return nil
}

func newWSLTest(t *testing.T) (*WSLService, *wslFakeHost, *wslFakeExec) {
	t.Helper()
	h := &wslFakeHost{
		profile: model.ConnectionProfile{ID: "c1", Type: model.ConnWSL, WSLDistro: "Ubuntu-24.04"},
		distros: []model.WSLDistro{
			{Name: "Ubuntu-24.04", State: "Running", Version: 2, Default: true},
			{Name: "Debian", State: "Running", Version: 2},
		},
	}
	ex := &wslFakeExec{host: h}
	s := &WSLService{Ops: ops.New(nil), host: h, deps: wslDeps{exec: ex, fake: true}}
	return s, h, ex
}

func TestWSLService_RejectsNonWSLConnections(t *testing.T) {
	s, h, _ := newWSLTest(t)
	h.profile.Type = model.ConnUnix
	if _, err := s.Info(context.Background(), "c1"); err == nil {
		t.Fatal("a non-WSL connection must be refused")
	}
	if _, err := s.Run(context.Background(), "c1", model.WSLActionShutdown); err == nil {
		t.Fatal("actions must be refused too")
	}
}

func TestWSLService_ShutdownDisconnectsFirstAndRunsOnlyWslShutdown(t *testing.T) {
	s, h, ex := newWSLTest(t)
	op, err := s.Run(context.Background(), "c1", model.WSLActionShutdown)
	if err != nil {
		t.Fatal(err)
	}
	if got := waitOp(t, s.Ops, op.ID); got.Status != model.OpSucceeded {
		t.Fatalf("status %s: %s", got.Status, got.Error)
	}
	var ran []string
	for _, c := range ex.cmds {
		if !strings.HasPrefix(c, "sh -c") {
			ran = append(ran, c)
		}
	}
	if len(ran) != 1 || ran[0] != "wsl.exe --shutdown" {
		t.Fatalf("unexpected commands: %v", ran)
	}
	last := h.events[len(h.events)-2:]
	if last[0] != "disconnect" || last[1] != "exec wsl.exe --shutdown" {
		t.Fatalf("must disconnect before shutting down: %v", h.events)
	}
}

func TestWSLService_RestartDistroTerminatesOnlyThatDistroThenStartsAndReconnects(t *testing.T) {
	s, h, _ := newWSLTest(t)
	op, err := s.Run(context.Background(), "c1", model.WSLActionRestartDistro)
	if err != nil {
		t.Fatal(err)
	}
	if got := waitOp(t, s.Ops, op.ID); got.Status != model.OpSucceeded {
		t.Fatalf("status %s: %s", got.Status, got.Error)
	}
	var seq []string
	for _, e := range h.events {
		if e == "exec wsl.exe --terminate Ubuntu-24.04" || e == "start Ubuntu-24.04" || e == "connect" {
			seq = append(seq, e)
		}
	}
	if strings.Join(seq, ",") != "exec wsl.exe --terminate Ubuntu-24.04,start Ubuntu-24.04,connect" {
		t.Fatalf("wrong order: %v", h.events)
	}
}

func TestWSLService_RefusesActionThatIsDisabled(t *testing.T) {
	s, h, _ := newWSLTest(t)
	h.distros[0].State = "Stopped"
	if _, err := s.Run(context.Background(), "c1", model.WSLActionReclaimMemory); err == nil {
		t.Fatal("reclaim on a stopped distribution must be refused")
	}
}

const detectOutput = `@@init
systemd
@@pkgs
docker-ce
@@cli
/usr/bin/docker
@@dockerd
/usr/sbin/dockerd
@@server
27.0.1
@@end`
