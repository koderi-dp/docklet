package services

import (
	"context"
	"strings"
	"time"

	"docklet/internal/compose"
	"docklet/internal/model"
)

// Graph returns the service structure of a project. It reads `docker compose config`
// (all profiles) when the CLI can run, and falls back to the container labels
// otherwise; the fallback carries a Note saying why.
func (s *ComposeService) Graph(ctx context.Context, connectionID, name string) (model.ComposeGraph, error) {
	p, env, err := s.project(ctx, connectionID, name)
	if err != nil {
		return model.ComposeGraph{}, err
	}
	note := p.Capability.Reason
	if p.Capability.CanRun {
		g, gerr := s.graphFromConfig(ctx, env, p)
		if gerr == nil {
			return g, nil
		}
		note = "docker compose config failed: " + strings.TrimSpace(firstLine(strings.TrimSpace(gerr.Error())))
	}
	cs, err := s.labelled(ctx, connectionID, name)
	if err != nil {
		return model.ComposeGraph{}, err
	}
	g := compose.GraphFromContainers(cs)
	g.Note = note
	return g, nil
}

func (s *ComposeService) graphFromConfig(ctx context.Context, env compose.ExecEnv, p model.ComposeProject) (model.ComposeGraph, error) {
	req := requestFor(p, compose.VerbConfig)
	req.ConfigMode = "json"
	req.Profiles = []string{"*"}
	cmd, err := compose.BuildCommand(env, req)
	if err != nil {
		return model.ComposeGraph{}, err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := s.run().Output(cctx, cmd)
	if err != nil {
		return model.ComposeGraph{}, err
	}
	return compose.ParseGraph(string(out))
}
