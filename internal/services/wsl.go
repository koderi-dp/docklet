package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/wslinfo"
)

// wslHost is what WSLService needs from the engine manager.
type wslHost interface {
	Profile(connectionID string) (model.ConnectionProfile, error)
	WSLDistros(ctx context.Context) ([]model.WSLDistro, error)
	StartWSLDistro(ctx context.Context, distro string) error
	Connect(ctx context.Context, id string) (model.ConnectionStatus, error)
	Disconnect(ctx context.Context, id string) error
}

// WSLService shows the state of the WSL distribution behind a WSL connection and runs
// a few safe maintenance actions on it. Everything inside the distribution runs as
// root through wsl.exe; no password is ever asked for and no configuration file is changed.
type WSLService struct {
	Manager *engine.Manager
	Ops     *ops.Manager

	// Replaced by tests.
	host wslHost
	deps wslDeps
}

const kindWSLPrefix = "wsl."

// wslBusy guards against two actions running on the same distribution.
var wslBusy sync.Map

func (s *WSLService) hostAPI() (wslHost, error) {
	if s.host != nil {
		return s.host, nil
	}
	if s.Manager == nil {
		return nil, errors.New("connection manager is not initialised")
	}
	return s.Manager, nil
}

// target resolves the connection to its distribution entry. It fails for connections that are not WSL.
func (s *WSLService) target(ctx context.Context, connectionID string) (wslHost, model.WSLDistro, []model.WSLDistro, error) {
	h, err := s.hostAPI()
	if err != nil {
		return nil, model.WSLDistro{}, nil, err
	}
	p, err := h.Profile(connectionID)
	if err != nil {
		return nil, model.WSLDistro{}, nil, err
	}
	if p.Type != model.ConnWSL || p.WSLDistro == "" {
		return nil, model.WSLDistro{}, nil, errors.New("this connection does not run in a WSL distribution")
	}
	all, err := h.WSLDistros(ctx)
	if err != nil {
		return nil, model.WSLDistro{}, nil, err
	}
	for _, d := range all {
		if strings.EqualFold(d.Name, p.WSLDistro) {
			return h, d, all, nil
		}
	}
	return nil, model.WSLDistro{}, nil, fmt.Errorf("WSL distribution %q was not found; it may have been unregistered", p.WSLDistro)
}

// Info reads the state of the distribution of a WSL connection. It is read-only and
// never starts a stopped distribution.
func (s *WSLService) Info(ctx context.Context, connectionID string) (model.WSLInfo, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	_, d, all, err := s.target(ctx, connectionID)
	if err != nil {
		return model.WSLInfo{}, err
	}
	info := s.collector().Collect(ctx, d, all)
	info.ConnectionID = connectionID
	return info, nil
}

// Run starts one maintenance action as a background operation. The state is read again
// here, so a stale screen cannot run an action that is no longer possible.
func (s *WSLService) Run(ctx context.Context, connectionID, action string) (model.Operation, error) {
	if s.Ops == nil {
		return model.Operation{}, errors.New("operations are not available")
	}
	rctx, cancel := callCtx(ctx)
	defer cancel()
	h, d, all, err := s.target(rctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	info := s.collector().Collect(rctx, d, all)
	a, ok := wslinfo.Action(info.Actions, action)
	if !ok {
		return model.Operation{}, fmt.Errorf("unknown WSL action %q", action)
	}
	if a.Disabled != "" {
		return model.Operation{}, errors.New(a.Disabled)
	}
	if _, busy := wslBusy.LoadOrStore(d.Name, true); busy {
		return model.Operation{}, fmt.Errorf("another WSL action is already running for %s", d.Name)
	}
	run := &wslRun{s: s, host: h, connectionID: connectionID, info: info}
	var fn ops.Func
	var title string
	switch a.ID {
	case model.WSLActionReclaimMemory:
		title, fn = "Reclaim memory in "+d.Name, run.reclaim
	case model.WSLActionRestartDocker:
		title, fn = "Restart Docker in "+d.Name, run.restartDocker
	case model.WSLActionRestartDistro:
		title, fn = "Restart "+d.Name, run.restartDistro
	default:
		title, fn = "Shut down WSL", run.shutdown
	}
	return s.Ops.Start(ops.Spec{Kind: kindWSLPrefix + a.ID, Title: title, ConnectionID: connectionID, Resource: d.Name},
		func(ctx context.Context, hd *ops.Handle) error {
			defer wslBusy.Delete(d.Name)
			return fn(ctx, hd)
		}), nil
}
