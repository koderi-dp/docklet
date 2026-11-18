package services

import (
	"context"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/compose"
	"docklet/internal/ports"
)

// ProjectPorts lists the host ports the project publishes: from `docker compose config
// --format json` when the CLI can run it, otherwise from the bindings of the project's
// existing containers. It never starts anything.
func (s *ComposeService) ProjectPorts(ctx context.Context, connectionID, name, service string, profiles []string) (ports.ProjectPorts, error) {
	p, env, err := s.project(ctx, connectionID, name)
	if err != nil {
		return ports.ProjectPorts{}, err
	}
	out := ports.ProjectPorts{OwnContainerIDs: p.ContainerIDs, Source: "none"}
	if !p.Capability.CanRun {
		out.Note = p.Capability.Reason
	} else {
		req := requestFor(p, compose.VerbConfig)
		req.ConfigMode, req.Profiles = "json", profiles
		if reqs, err := s.configPorts(ctx, env, req, service); err == nil {
			out.Requests, out.Source = reqs, "compose-config"
			return out, nil
		} else {
			out.Note = "The Compose configuration could not be read (" + err.Error() + ")."
		}
	}
	return s.portsFromContainers(ctx, connectionID, name, service, out)
}

// portsFromContainers is the fallback: what the project's existing containers publish.
func (s *ComposeService) portsFromContainers(ctx context.Context, connectionID, name, service string, out ports.ProjectPorts) (ports.ProjectPorts, error) {
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return out, err
	}
	f := client.Filters{}.Add("label", compose.LabelProject+"="+name)
	if service != "" {
		f = f.Add("label", compose.LabelService+"="+service)
	}
	bs, err := engineSource{cli: cli, filter: f}.Bindings(ctx)
	if err != nil {
		return out, friendlyError(err)
	}
	seen := map[ports.Request]bool{}
	for _, b := range bs {
		r := ports.Request{HostIP: b.HostIP, HostPort: b.HostPort, Protocol: b.Protocol}
		if !seen[r] {
			seen[r] = true
			out.Requests = append(out.Requests, r)
		}
	}
	if len(out.Requests) > 0 {
		out.Source = "containers"
	}
	return out, nil
}

func (s *ComposeService) configPorts(ctx context.Context, env compose.ExecEnv, req compose.Request, service string) ([]ports.Request, error) {
	cmd, err := compose.BuildCommand(env, req)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	text, err := s.run().Output(cctx, cmd)
	if err != nil {
		return nil, err
	}
	return ports.ParseComposePorts([]byte(text), service)
}
