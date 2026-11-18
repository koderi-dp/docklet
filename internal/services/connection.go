package services

import (
	"context"
	"errors"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/wsldocker"
)

// ErrNotImplemented marks contract methods whose implementation is owned by a
// workstream that has not landed yet. It is an honest error, never fake data.
var ErrNotImplemented = errors.New("not implemented yet")

// ConnectionService is bound to the frontend. Every resource call elsewhere
// takes an explicit connectionID; there is no implicit "current" connection in Go.
type ConnectionService struct {
	Manager *engine.Manager
	// Ops runs the Docker Engine install in the background (nil disables it).
	Ops *ops.Manager
	// WSLExec runs commands inside WSL distributions; nil uses wsl.exe.
	WSLExec wsldocker.Exec
	// EnvironmentHook may rewrite the detection result (test aid, see app_services.go); nil in normal use.
	EnvironmentHook func(model.EnvironmentReport) model.EnvironmentReport
}

func (s *ConnectionService) mgr() (*engine.Manager, error) {
	if s.Manager == nil {
		return nil, errors.New("connection manager is not initialised")
	}
	return s.Manager, nil
}

// List returns saved profiles, or the discovered local default when none are saved.
func (s *ConnectionService) List(ctx context.Context) ([]model.ConnectionProfile, error) {
	m, err := s.mgr()
	if err != nil {
		return nil, err
	}
	return m.List(ctx)
}

// Save creates or updates a profile (ID empty => new). Secrets never pass through here.
func (s *ConnectionService) Save(ctx context.Context, p model.ConnectionProfile) (model.ConnectionProfile, error) {
	m, err := s.mgr()
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	return m.Save(ctx, p)
}

func (s *ConnectionService) Remove(ctx context.Context, id string) error {
	m, err := s.mgr()
	if err != nil {
		return err
	}
	return m.Remove(ctx, id)
}

// Connect (re)establishes the Engine client and starts event streaming.
// Failures are returned as a ConnectionStatus with State=error, not a Go error.
func (s *ConnectionService) Connect(ctx context.Context, id string) (model.ConnectionStatus, error) {
	m, err := s.mgr()
	if err != nil {
		return model.ConnectionStatus{}, err
	}
	return m.Connect(ctx, id)
}

// Disconnect stops streams and closes the client for this connection.
func (s *ConnectionService) Disconnect(ctx context.Context, id string) error {
	m, err := s.mgr()
	if err != nil {
		return err
	}
	return m.Disconnect(ctx, id)
}

// Status returns the current known status without side effects.
func (s *ConnectionService) Status(ctx context.Context, id string) (model.ConnectionStatus, error) {
	m, err := s.mgr()
	if err != nil {
		return model.ConnectionStatus{}, err
	}
	return m.Status(ctx, id)
}

// Test checks a profile with a temporary client without saving or activating it.
// Failures are returned as ConnectionStatus{State: error} with friendly text and
// redacted diagnostics, not as a Go error.
func (s *ConnectionService) Test(ctx context.Context, p model.ConnectionProfile) (model.ConnectionStatus, error) {
	m, err := s.mgr()
	if err != nil {
		return model.ConnectionStatus{}, err
	}
	return m.Test(ctx, p)
}

// Duplicate saves a copy of a profile with a unique name.
func (s *ConnectionService) Duplicate(ctx context.Context, id string) (model.ConnectionProfile, error) {
	m, err := s.mgr()
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	return m.Duplicate(ctx, id)
}

// WSLDistros lists installed WSL distributions (empty off Windows).
func (s *ConnectionService) WSLDistros(ctx context.Context) ([]model.WSLDistro, error) {
	m, err := s.mgr()
	if err != nil {
		return nil, err
	}
	return m.WSLDistros(ctx)
}

// WSLDockerInfo detects Docker inside a running distribution without starting it.
func (s *ConnectionService) WSLDockerInfo(ctx context.Context, distro string) (model.WSLDockerInfo, error) {
	m, err := s.mgr()
	if err != nil {
		return model.WSLDockerInfo{}, err
	}
	return m.WSLDockerInfo(ctx, distro)
}

// StartWSLDistro boots a stopped distribution. Call only on explicit user action.
func (s *ConnectionService) StartWSLDistro(ctx context.Context, distro string) error {
	m, err := s.mgr()
	if err != nil {
		return err
	}
	return m.StartWSLDistro(ctx, distro)
}

// DockerContexts lists the Docker CLI contexts on this machine (read-only).
func (s *ConnectionService) DockerContexts(ctx context.Context) ([]model.DockerContextInfo, error) {
	m, err := s.mgr()
	if err != nil {
		return nil, err
	}
	return m.DockerContexts(ctx)
}

// ContextProfile returns the unsaved profile that importing the context would create.
func (s *ConnectionService) ContextProfile(ctx context.Context, name string) (model.ConnectionProfile, error) {
	m, err := s.mgr()
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	return m.ContextProfile(ctx, name)
}

// ImportContext saves a connection created from a Docker context.
func (s *ConnectionService) ImportContext(ctx context.Context, name string) (model.ConnectionProfile, error) {
	m, err := s.mgr()
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	return m.ImportContext(ctx, name)
}

// SetSecret stores an SSH password or key passphrase (kind "ssh-password" or
// "ssh-passphrase") for a saved connection in the OS credential store. The value
// is write-only: it is never returned, logged or persisted anywhere else.
func (s *ConnectionService) SetSecret(ctx context.Context, connectionID, kind, value string) error {
	m, err := s.mgr()
	if err != nil {
		return err
	}
	return m.SetSecret(connectionID, kind, value)
}

// HasSecret reports whether a secret of that kind is stored for the connection.
func (s *ConnectionService) HasSecret(ctx context.Context, connectionID, kind string) (bool, error) {
	m, err := s.mgr()
	if err != nil {
		return false, err
	}
	return m.HasSecret(connectionID, kind)
}

// DeleteSecret removes a stored secret.
func (s *ConnectionService) DeleteSecret(ctx context.Context, connectionID, kind string) error {
	m, err := s.mgr()
	if err != nil {
		return err
	}
	return m.DeleteSecret(connectionID, kind)
}
