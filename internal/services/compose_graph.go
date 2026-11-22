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
	p, env, cs, err := s.projectWithContainers(ctx, connectionID, name)
	if err != nil {
		return model.ComposeGraph{}, err
	}
	var out string
	var cerr error
	if p.Capability.CanRun {
		out, cerr = s.configJSON(ctx, env, p)
	}
	return graphOrFallback(out, cerr, p.Capability.CanRun, p.Capability.Reason, cs), nil
}

// graphOrFallback parses the config output, or falls back to the containers with a Note saying why.
func graphOrFallback(configOut string, configErr error, canRun bool, reason string, cs []compose.Container) model.ComposeGraph {
	note := reason
	if canRun {
		err := configErr
		if err == nil {
			g, perr := compose.ParseGraph(configOut)
			if perr == nil {
				return g
			}
			err = perr
		}
		note = "docker compose config failed: " + strings.TrimSpace(firstLine(strings.TrimSpace(err.Error())))
	}
	g := compose.GraphFromContainers(cs)
	g.Note = note
	return g
}

func (s *ComposeService) configJSON(ctx context.Context, env compose.ExecEnv, p model.ComposeProject) (string, error) {
	req := requestFor(p, compose.VerbConfig)
	req.ConfigMode = "json"
	req.Profiles = []string{"*"}
	cmd, err := compose.BuildCommand(env, req)
	if err != nil {
		return "", err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.run().Output(cctx, cmd)
}
