package services

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ports"
)

// ProjectPortSource reads the host ports a Compose project would publish (implemented by ComposeService).
type ProjectPortSource interface {
	ProjectPorts(ctx context.Context, connectionID, project, service string, profiles []string) (ports.ProjectPorts, error)
}

// PortService is bound to the frontend: it answers what already uses a host port.
type PortService struct {
	Engines  engine.Provider
	Profiles engine.ProfileSource
	Compose  ProjectPortSource

	// hosts picks the host sources for a profile; nil means the real ones (tests inject fakes).
	hosts func(model.ConnectionProfile) ([]ports.HostSource, string)
}

// Check reports who holds each requested host port. Containers listed in
// ignoreContainerIDs (one being recreated) are not counted. A source that cannot be read
// makes a port "unknown", never "free".
func (s *PortService) Check(ctx context.Context, connectionID string, requests []ports.Request, ignoreContainerIDs []string) ([]ports.PortCheck, error) {
	c, err := s.checker(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	return c.Check(ctx, requests, ignoreContainerIDs), nil
}

// NextFree suggests the next port above the requested one that nothing holds.
func (s *PortService) NextFree(ctx context.Context, connectionID string, request ports.Request, ignoreContainerIDs []string) (ports.Suggestion, error) {
	c, err := s.checker(ctx, connectionID)
	if err != nil {
		return ports.Suggestion{}, err
	}
	return c.NextFree(ctx, request, ignoreContainerIDs), nil
}

// Listeners returns every published port of the Engine and, for a local Engine, every
// listener of this computer.
func (s *PortService) Listeners(ctx context.Context, connectionID string) (ports.Listing, error) {
	c, err := s.checker(ctx, connectionID)
	if err != nil {
		return ports.Listing{}, err
	}
	return c.List(ctx), nil
}

// CheckProject checks the host ports a Compose project would publish when started
// (only service's ports when service is set). Its own containers are not conflicts.
func (s *PortService) CheckProject(ctx context.Context, connectionID, project, service string, profiles []string) (ports.ProjectCheck, error) {
	if s.Compose == nil {
		return ports.ProjectCheck{}, fmt.Errorf("Compose is not available")
	}
	pp, err := s.Compose.ProjectPorts(ctx, connectionID, project, service, profiles)
	if err != nil {
		return ports.ProjectCheck{}, err
	}
	out := ports.ProjectCheck{Checks: []ports.PortCheck{}, Source: pp.Source, Note: pp.Note}
	if len(pp.Requests) == 0 {
		return out, nil
	}
	c, err := s.checker(ctx, connectionID)
	if err != nil {
		return out, err
	}
	out.Checks = c.Check(ctx, pp.Requests, pp.OwnContainerIDs)
	return out, nil
}

func (s *PortService) checker(ctx context.Context, connectionID string) (*ports.Checker, error) {
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return nil, err
	}
	c := &ports.Checker{Engine: engineSource{cli: cli}}
	if s.Profiles == nil {
		c.HostUnknown = "the connection details are not available"
		return c, nil
	}
	prof, err := s.Profiles.Profile(connectionID)
	if err != nil {
		return nil, err
	}
	hostsFor := s.hosts
	if hostsFor == nil {
		hostsFor = hostSources
	}
	c.Hosts, c.HostUnknown = hostsFor(prof)
	return c, nil
}

// hostSources decides which machine's listeners can hold the ports an Engine publishes.
// An Engine reached over the network (tcp, ssh) runs on another computer: nothing to read.
func hostSources(p model.ConnectionProfile) ([]ports.HostSource, string) {
	ep := strings.ToLower(p.Endpoint)
	switch {
	case p.Type == model.ConnWSL:
		hosts := []ports.HostSource{ports.NewWSLHost(p.WSLDistro)}
		if runtime.GOOS == "windows" {
			// WSL forwards published ports to Windows localhost.
			hosts = append(hosts, ports.NewWindowsHost())
		}
		return hosts, ""
	case p.Type == model.ConnNamedPipe, p.Type == model.ConnUnix, strings.HasPrefix(ep, "npipe:"), strings.HasPrefix(ep, "unix:"):
		return []ports.HostSource{ports.NewLocalHost()}, ""
	}
	return nil, "the Engine runs on another computer, so programs there cannot be seen"
}
