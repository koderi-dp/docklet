package services

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	"docklet/internal/engine"
	"docklet/internal/model"
)

const (
	// envConcurrency caps simultaneous probes so a machine with many distributions is not flooded.
	envConcurrency = 3
	// envProbeTimeout bounds each probe: one hung probe must not block the whole screen.
	envProbeTimeout = 15 * time.Second
)

// envSource is everything detection needs. ConnectionService implements it with the real
// functions; tests supply fakes.
type envSource interface {
	WSLDistros(ctx context.Context) ([]model.WSLDistro, error)
	WSLDockerInfo(ctx context.Context, distro string) (model.WSLDockerInfo, error)
	WSLDockerInstallPlan(ctx context.Context, distro string) (model.WSLInstallPlan, error)
	Test(ctx context.Context, p model.ConnectionProfile) (model.ConnectionStatus, error)
	DockerContexts(ctx context.Context) ([]model.DockerContextInfo, error)
	List(ctx context.Context) ([]model.ConnectionProfile, error)
	Discovered() []model.ConnectionProfile
	PodmanMachines(ctx context.Context) ([]model.PodmanMachineInfo, error)
}

// DetectEnvironment looks at what this machine already has: WSL distributions and their Docker
// state, local engines, Docker CLI contexts and saved connections. It is read-only: a stopped
// distribution is reported as stopped, never booted, and nothing is saved or connected.
func (s *ConnectionService) DetectEnvironment(ctx context.Context) (model.EnvironmentReport, error) {
	if _, err := s.mgr(); err != nil {
		return model.EnvironmentReport{}, err
	}
	rep := detectEnvironment(ctx, runtime.GOOS, liveEnv{s})
	if s.EnvironmentHook != nil {
		rep = s.EnvironmentHook(rep)
	}
	return rep, nil
}

// liveEnv is the real envSource: the service's own methods plus the well-known local endpoints.
type liveEnv struct{ *ConnectionService }

// Discovered returns the named pipe or sockets Docker usually listens on; they may not answer.
func (liveEnv) Discovered() []model.ConnectionProfile { return engine.DiscoveredProfiles() }

// PodmanMachines reads `podman machine list` (never starts anything).
func (l liveEnv) PodmanMachines(ctx context.Context) ([]model.PodmanMachineInfo, error) {
	return l.ConnectionService.PodmanMachines(ctx)
}

// detectEnvironment composes the existing detection calls. The sections run side by side and
// their probes share one concurrency cap.
func detectEnvironment(ctx context.Context, goos string, src envSource) model.EnvironmentReport {
	rep := model.EnvironmentReport{OS: goos, Local: []model.LocalEngineReport{}, Contexts: []model.DockerContextInfo{},
		Saved: []model.ConnectionProfile{}, WSL: model.WSLReport{Distros: []model.WSLDistroReport{}}}
	sem := make(chan struct{}, envConcurrency)
	probe := func(wg *sync.WaitGroup, fn func(ctx context.Context)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			pctx, cancel := context.WithTimeout(ctx, envProbeTimeout)
			defer cancel()
			fn(pctx)
		}()
	}

	var wg sync.WaitGroup
	if goos == "windows" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rep.WSL = detectWSL(ctx, src, probe)
		}()
	}
	found := src.Discovered()
	rep.Local = make([]model.LocalEngineReport, len(found))
	for i, p := range found {
		probe(&wg, func(pctx context.Context) { rep.Local[i] = probeLocal(pctx, src, p) })
	}
	var stopped []model.LocalEngineReport
	probe(&wg, func(pctx context.Context) { stopped = stoppedMachines(pctx, src) })
	probe(&wg, func(pctx context.Context) {
		if c, err := src.DockerContexts(pctx); err == nil && c != nil {
			rep.Contexts = c
		}
	})
	probe(&wg, func(pctx context.Context) {
		list, _ := src.List(pctx)
		for _, p := range list {
			if !p.Discovered {
				rep.Saved = append(rep.Saved, p)
			}
		}
	})
	wg.Wait()
	rep.Local = append(rep.Local, stopped...)
	return rep
}

// stoppedMachines lists Podman machines that exist but do not run. They are reported with the hint
// to start them; Docklet never starts one. Running machines are already found as discovered pipes.
func stoppedMachines(ctx context.Context, src envSource) []model.LocalEngineReport {
	machines, _ := src.PodmanMachines(ctx)
	var out []model.LocalEngineReport
	for _, m := range machines {
		if !m.Running {
			p := m.Profile
			p.Name = "Podman (" + m.Name + ")"
			out = append(out, model.LocalEngineReport{Profile: p, Kind: model.EnginePodman, Stopped: true, Error: m.Hint})
		}
	}
	return out
}

type probeFunc func(wg *sync.WaitGroup, fn func(ctx context.Context))

func detectWSL(ctx context.Context, src envSource, probe probeFunc) model.WSLReport {
	out := model.WSLReport{Distros: []model.WSLDistroReport{}}
	lctx, cancel := context.WithTimeout(ctx, envProbeTimeout)
	defer cancel()
	distros, err := src.WSLDistros(lctx)
	switch {
	case err != nil:
		out.Detail = "Could not list WSL distributions: " + err.Error()
		return out
	case len(distros) == 0:
		out.Detail = "No WSL distributions are installed (or WSL itself is not installed)."
		return out
	}
	out.Available = true
	for _, d := range distros {
		out.Distros = append(out.Distros, model.WSLDistroReport{Name: d.Name, Version: d.Version, State: d.State, Default: d.Default,
			Internal: isInternalDistro(d.Name)})
	}
	var wg sync.WaitGroup
	for i := range out.Distros {
		d := &out.Distros[i]
		if d.Internal {
			d.Docker = model.WSLDockerReport{State: model.WSLStateUnknown, Detail: "Internal distribution (Docker Desktop or a Podman machine)."}
			continue
		}
		probe(&wg, func(pctx context.Context) { d.Docker = probeDistro(pctx, src, d.Name) })
	}
	wg.Wait()
	return out
}

// probeDistro reports one distribution's Docker state without starting it.
func probeDistro(ctx context.Context, src envSource, name string) model.WSLDockerReport {
	info, err := src.WSLDockerInfo(ctx, name)
	if err != nil {
		return model.WSLDockerReport{State: model.WSLStateUnknown, Detail: probeError(ctx, err)}
	}
	if !info.Running {
		return model.WSLDockerReport{State: model.WSLStateStopped, Detail: "Stopped. Start it to look for Docker."}
	}
	rep := model.WSLDockerReport{Reachable: info.DaemonReachable, Version: info.ServerVersion, Detail: info.Detail}
	plan, err := src.WSLDockerInstallPlan(ctx, name)
	switch {
	case err != nil && info.DaemonReachable:
		rep.State = model.WSLStateRunning
	case err != nil:
		rep.State, rep.Detail = model.WSLStateUnknown, probeError(ctx, err)
	default:
		rep.State, rep.Supported, rep.Reason, rep.DocsURL = plan.State, plan.Supported, plan.Reason, plan.DocsURL
		if plan.DockerDesktop && plan.State == model.WSLStateRunning {
			rep.State = model.WSLStateDockerDesktop // reachable, but through Docker Desktop's proxy
		}
	}
	return rep
}

func probeLocal(ctx context.Context, src envSource, p model.ConnectionProfile) model.LocalEngineReport {
	out := model.LocalEngineReport{Profile: p}
	st, err := src.Test(ctx, p)
	switch {
	case err != nil:
		out.Error = probeError(ctx, err)
	case st.State != model.StateConnected:
		out.Error = st.Error
	default:
		out.Reachable, out.ServerVersion, out.Kind = true, st.ServerVersion, st.Kind
	}
	return out
}

func probeError(ctx context.Context, err error) string {
	if ctx.Err() == context.DeadlineExceeded {
		return "Did not answer in time."
	}
	return err.Error()
}

// isInternalDistro marks the distributions Docker Desktop and Podman create for themselves: they are
// reached through their own pipe, never as a WSL connection.
func isInternalDistro(name string) bool {
	n := strings.ToLower(name)
	return strings.HasPrefix(n, "docker-desktop") || strings.HasPrefix(n, "podman-")
}
