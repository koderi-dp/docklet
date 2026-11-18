package services

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/wsldocker"
)

const kindInstallDocker = "wsl.install-docker"

// installing guards against two installs into the same distribution.
var installing sync.Map

func (s *ConnectionService) wslExec() wsldocker.Exec {
	if s.WSLExec != nil {
		return s.WSLExec
	}
	return wsldocker.NewExec()
}

// assess inspects a distribution (read-only) and builds the plan.
func (s *ConnectionService) assess(ctx context.Context, distro string) (wsldocker.Plan, error) {
	m, err := s.mgr()
	if err != nil {
		return wsldocker.Plan{}, err
	}
	info, err := m.WSLDockerInfo(ctx, distro) // validates the name and never boots the distribution
	if err != nil {
		return wsldocker.Plan{}, err
	}
	if !info.Running {
		return wsldocker.Plan{State: model.WSLStateStopped, Facts: wsldocker.Facts{Distro: info.Distro},
			Reason: "The distribution is stopped. Start it first so Docker can be inspected."}, nil
	}
	facts, err := wsldocker.Detect(ctx, s.wslExec(), info.Distro)
	if err != nil {
		return wsldocker.Plan{}, err
	}
	plan := wsldocker.BuildPlan(facts)
	if plan.Supported {
		if distros, _ := m.WSLDistros(ctx); isWSL1(distros, info.Distro) {
			plan.Supported = false
			plan.Reason = "This distribution runs on WSL 1, which cannot run Docker Engine. Convert it with `wsl --set-version " + info.Distro + " 2` first."
		}
	}
	return plan, nil
}

func isWSL1(distros []model.WSLDistro, name string) bool {
	for _, d := range distros {
		if d.Name == name {
			return d.Version == 1
		}
	}
	return false
}

// WSLDockerInstallPlan inspects a running distribution and returns what installing
// Docker Engine would do, step by step. It changes nothing.
func (s *ConnectionService) WSLDockerInstallPlan(ctx context.Context, distro string) (model.WSLInstallPlan, error) {
	plan, err := s.assess(ctx, distro)
	if err != nil {
		return model.WSLInstallPlan{}, err
	}
	return plan.DTO(), nil
}

// InstallDockerInWSL installs Docker Engine in the distribution as one cancellable
// operation. The plan is rebuilt and checked again here, so a stale UI cannot
// install over an engine that appeared, or remove packages the user did not confirm.
func (s *ConnectionService) InstallDockerInWSL(ctx context.Context, distro string, opts model.WSLInstallOptions) (model.Operation, error) {
	if s.Ops == nil {
		return model.Operation{}, errors.New("operations are not available")
	}
	plan, err := s.assess(ctx, distro)
	if err != nil {
		return model.Operation{}, err
	}
	if err := plan.Preflight(opts.RemoveConflicting); err != nil {
		return model.Operation{}, err
	}
	name := plan.Facts.Distro
	if _, busy := installing.LoadOrStore(name, true); busy {
		return model.Operation{}, fmt.Errorf("an install is already running in %s", name)
	}
	steps := plan.WithOptions(wsldocker.Options{EnableSystemd: opts.EnableSystemd})
	ex := s.wslExec()
	return s.Ops.Start(ops.Spec{Kind: kindInstallDocker, Title: "Install Docker Engine in " + name, Resource: name, Cancellable: true},
		func(ctx context.Context, h *ops.Handle) error {
			defer installing.Delete(name)
			h.Log(fmt.Sprintf("Installing Docker Engine in %s (%d steps). Commands run as root inside the distribution through wsl.exe.", name, len(steps)))
			return wsldocker.Run(ctx, ex, name, steps, wsldocker.Reporter{
				Log: h.Log,
				Step: func(i, total int, _ string) {
					h.Progress(float64(i-1) / float64(total))
				},
			})
		}), nil
}

// StartDockerInWSL starts the daemon of an installed Docker Engine as root and
// waits for it to answer. Explicit user action only.
func (s *ConnectionService) StartDockerInWSL(ctx context.Context, distro string) (model.WSLDockerInfo, error) {
	m, err := s.mgr()
	if err != nil {
		return model.WSLDockerInfo{}, err
	}
	info, err := m.WSLDockerInfo(ctx, distro)
	if err != nil {
		return info, err
	}
	if !info.Running {
		return info, errors.New("the distribution is stopped; start it first")
	}
	if _, err := wsldocker.Start(ctx, s.wslExec(), info.Distro); err != nil {
		return info, err
	}
	return m.WSLDockerInfo(ctx, info.Distro)
}
