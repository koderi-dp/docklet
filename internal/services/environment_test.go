package services

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"docklet/internal/model"
)

type fakeEnv struct {
	distros  []model.WSLDistro
	infos    map[string]model.WSLDockerInfo
	plans    map[string]model.WSLInstallPlan
	local    []model.ConnectionProfile
	tests    map[string]model.ConnectionStatus
	contexts []model.DockerContextInfo
	saved    []model.ConnectionProfile
	hang     map[string]bool // distro names whose probe never answers

	mu       sync.Mutex
	probed   []string
	inFlight atomic.Int32
	maxSeen  atomic.Int32
}

func (f *fakeEnv) WSLDistros(context.Context) ([]model.WSLDistro, error) { return f.distros, nil }
func (f *fakeEnv) WSLDockerInfo(ctx context.Context, d string) (model.WSLDockerInfo, error) {
	n := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for {
		m := f.maxSeen.Load()
		if n <= m || f.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}
	f.mu.Lock()
	f.probed = append(f.probed, d)
	f.mu.Unlock()
	if f.hang[d] {
		<-ctx.Done()
		return model.WSLDockerInfo{}, ctx.Err()
	}
	time.Sleep(5 * time.Millisecond)
	return f.infos[d], nil
}
func (f *fakeEnv) WSLDockerInstallPlan(_ context.Context, d string) (model.WSLInstallPlan, error) {
	return f.plans[d], nil
}
func (f *fakeEnv) Test(_ context.Context, p model.ConnectionProfile) (model.ConnectionStatus, error) {
	return f.tests[p.ID], nil
}
func (f *fakeEnv) DockerContexts(context.Context) ([]model.DockerContextInfo, error) {
	return f.contexts, nil
}
func (f *fakeEnv) List(context.Context) ([]model.ConnectionProfile, error) {
	return append(append([]model.ConnectionProfile{}, f.saved...), f.local...), nil
}
func (f *fakeEnv) Discovered() []model.ConnectionProfile { return f.local }

func TestDetect_StoppedDistroStaysStoppedAndIsNotBooted(t *testing.T) {
	f := &fakeEnv{
		distros: []model.WSLDistro{{Name: "Ubuntu", State: "Stopped", Version: 2, Default: true}},
		infos:   map[string]model.WSLDockerInfo{"Ubuntu": {Distro: "Ubuntu", Running: false}},
		plans:   map[string]model.WSLInstallPlan{"Ubuntu": {State: model.WSLStateRunning}},
	}
	d := detectEnvironment(context.Background(), "windows", f).WSL.Distros[0]
	if d.Docker.State != model.WSLStateStopped || d.Docker.Reachable {
		t.Fatalf("a stopped distribution must stay stopped, got %+v", d.Docker)
	}
}

func TestDetect_DockerDesktopProxyIsNotReportedAsOwnEngine(t *testing.T) {
	f := &fakeEnv{
		distros: []model.WSLDistro{{Name: "Ubuntu", State: "Running", Version: 2}},
		infos:   map[string]model.WSLDockerInfo{"Ubuntu": {Running: true, CLIFound: true, DaemonReachable: true, ServerVersion: "29.1.0"}},
		plans:   map[string]model.WSLInstallPlan{"Ubuntu": {State: model.WSLStateRunning, DockerDesktop: true}},
	}
	d := detectEnvironment(context.Background(), "windows", f).WSL.Distros[0]
	if d.Docker.State != model.WSLStateDockerDesktop || !d.Docker.Reachable || d.Docker.Version != "29.1.0" {
		t.Fatalf("want a reachable Docker Desktop proxy, got %+v", d.Docker)
	}
}

func TestDetect_OwnEngineKeepsItsVersionAndInstallableDistroIsMarked(t *testing.T) {
	f := &fakeEnv{
		distros: []model.WSLDistro{{Name: "A", State: "Running", Version: 2}, {Name: "B", State: "Running", Version: 2}},
		infos: map[string]model.WSLDockerInfo{
			"A": {Running: true, DaemonReachable: true, ServerVersion: "28.0.1"},
			"B": {Running: true},
		},
		plans: map[string]model.WSLInstallPlan{
			"A": {State: model.WSLStateRunning},
			"B": {State: model.WSLStateNotInstalled, Supported: true},
		},
	}
	ds := detectEnvironment(context.Background(), "windows", f).WSL.Distros
	if ds[0].Docker.State != model.WSLStateRunning || ds[0].Docker.Version != "28.0.1" {
		t.Fatalf("A: %+v", ds[0].Docker)
	}
	if ds[1].Docker.State != model.WSLStateNotInstalled || !ds[1].Docker.Supported || ds[1].Docker.Reachable {
		t.Fatalf("B: %+v", ds[1].Docker)
	}
}

func TestDetect_InternalDockerDesktopDistroIsNeverProbed(t *testing.T) {
	f := &fakeEnv{distros: []model.WSLDistro{{Name: "docker-desktop", State: "Running", Version: 2}}}
	rep := detectEnvironment(context.Background(), "windows", f)
	if len(f.probed) != 0 || !rep.WSL.Distros[0].Internal {
		t.Fatalf("probed=%v internal=%v", f.probed, rep.WSL.Distros[0].Internal)
	}
}

func TestDetect_ProbesRespectTheConcurrencyCap(t *testing.T) {
	f := &fakeEnv{infos: map[string]model.WSLDockerInfo{}, plans: map[string]model.WSLInstallPlan{}}
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		f.distros = append(f.distros, model.WSLDistro{Name: n, State: "Running", Version: 2})
		f.infos[n] = model.WSLDockerInfo{Running: true}
	}
	detectEnvironment(context.Background(), "windows", f)
	if got := f.maxSeen.Load(); got > envConcurrency || got < 2 {
		t.Fatalf("peak concurrency %d, want between 2 and %d", got, envConcurrency)
	}
}

func TestDetect_OneHungProbeDoesNotBlockTheOthers(t *testing.T) {
	// The per-probe timeout is a constant; a short parent deadline stands in for it.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	f := &fakeEnv{
		distros: []model.WSLDistro{{Name: "slow", State: "Running", Version: 2}, {Name: "fast", State: "Running", Version: 2}},
		infos:   map[string]model.WSLDockerInfo{"fast": {Running: true, DaemonReachable: true, ServerVersion: "29.0.0"}},
		plans:   map[string]model.WSLInstallPlan{"fast": {State: model.WSLStateRunning}},
		hang:    map[string]bool{"slow": true},
	}
	start := time.Now()
	ds := detectEnvironment(ctx, "windows", f).WSL.Distros
	if time.Since(start) > 2*time.Second {
		t.Fatal("detection waited for the hung probe")
	}
	if ds[0].Docker.State != model.WSLStateUnknown || ds[0].Docker.Detail != "Did not answer in time." {
		t.Fatalf("slow: %+v", ds[0].Docker)
	}
	if !ds[1].Docker.Reachable {
		t.Fatalf("fast: %+v", ds[1].Docker)
	}
}

func TestDetect_LocalEngineReportsTestOutcomeAndSavedExcludesDiscovered(t *testing.T) {
	up := model.ConnectionProfile{ID: "up", Name: "Up", Discovered: true}
	down := model.ConnectionProfile{ID: "down", Name: "Down", Discovered: true}
	f := &fakeEnv{
		local: []model.ConnectionProfile{up, down},
		tests: map[string]model.ConnectionStatus{
			"up":   {State: model.StateConnected, ServerVersion: "29.1.0"},
			"down": {State: model.StateError, Error: "Docker is not running."},
		},
		saved:    []model.ConnectionProfile{{ID: "mine", Name: "Mine"}},
		contexts: []model.DockerContextInfo{{Name: "default"}},
	}
	rep := detectEnvironment(context.Background(), "linux", f)
	if !rep.Local[0].Reachable || rep.Local[0].ServerVersion != "29.1.0" {
		t.Fatalf("up: %+v", rep.Local[0])
	}
	if rep.Local[1].Reachable || rep.Local[1].Error != "Docker is not running." {
		t.Fatalf("down: %+v", rep.Local[1])
	}
	if len(rep.Saved) != 1 || rep.Saved[0].ID != "mine" || len(rep.Contexts) != 1 {
		t.Fatalf("saved=%v contexts=%v", rep.Saved, rep.Contexts)
	}
}

func TestDetect_NonWindowsNeverLooksForWSL(t *testing.T) {
	f := &fakeEnv{distros: []model.WSLDistro{{Name: "x"}}}
	rep := detectEnvironment(context.Background(), "linux", f)
	if rep.WSL.Available || len(f.probed) != 0 {
		t.Fatalf("wsl=%+v probed=%v", rep.WSL, f.probed)
	}
}

type failingDistros struct{ fakeEnv }

func (*failingDistros) WSLDistros(context.Context) ([]model.WSLDistro, error) {
	return nil, errors.New("boom")
}

func TestDetect_WSLListFailureIsReportedNotHidden(t *testing.T) {
	rep := detectEnvironment(context.Background(), "windows", &failingDistros{})
	if rep.WSL.Available || rep.WSL.Detail == "" {
		t.Fatalf("a failed listing needs an explanation: %+v", rep.WSL)
	}
}
