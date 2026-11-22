package services

import (
	"context"
	"sync"
	"time"

	"docklet/internal/compose"
	"docklet/internal/model"
)

// ApplyPlan works out what `docker compose up -d` would change for the project with the given
// profiles. A failing config comes back as plan.Error, not as a Go error.
func (s *ComposeService) ApplyPlan(ctx context.Context, connectionID, name string, profiles []string) (model.ComposeApplyPlan, error) {
	p, env, cs, err := s.projectWithContainers(ctx, connectionID, name)
	if err != nil {
		return model.ComposeApplyPlan{}, err
	}
	fail := func(msg string) (model.ComposeApplyPlan, error) {
		return model.ComposeApplyPlan{Items: []model.ComposeApplyItem{}, Error: msg}, nil
	}
	if !p.Capability.CanRun {
		return fail(p.Capability.Reason)
	}
	hashReq := requestFor(p, compose.VerbConfig)
	hashReq.ConfigMode, hashReq.Profiles = "hash", profiles
	jsonReq := requestFor(p, compose.VerbConfig)
	jsonReq.ConfigMode, jsonReq.Profiles = "json", []string{"*"}

	var hashOut, jsonOut string
	var hashErr, jsonErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); hashOut, hashErr = s.configOutput(ctx, env, hashReq) }()
	go func() { defer wg.Done(); jsonOut, jsonErr = s.configOutput(ctx, env, jsonReq) }()
	wg.Wait()
	if hashErr != nil {
		return fail(errFirstLine(hashErr))
	}
	if jsonErr != nil {
		return fail(errFirstLine(jsonErr))
	}
	g, err := compose.ParseGraph(jsonOut)
	if err != nil {
		return fail(errFirstLine(err))
	}
	hashes := compose.ParseHashes(hashOut)
	all := make([]string, 0, len(g.Services))
	for _, sv := range g.Services {
		all = append(all, sv.Name)
	}
	return model.ComposeApplyPlan{Items: compose.PlanApply(compose.PlanInput{
		Hashes: hashes, AllServices: all, Containers: cs,
		ImageIDs: s.serviceImageIDs(ctx, connectionID, p.Name, g, hashes),
	})}, nil
}

func (s *ComposeService) configOutput(ctx context.Context, env compose.ExecEnv, req compose.Request) (string, error) {
	cmd, err := compose.BuildCommand(env, req)
	if err != nil {
		return "", err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.run().Output(cctx, cmd)
}

// serviceImageIDs asks the Engine for the id of each selected service's image. A service whose
// image is not present (or that cannot be named) is left out.
func (s *ComposeService) serviceImageIDs(ctx context.Context, connectionID, project string, g model.ComposeGraph, hashes map[string]string) map[string]string {
	ids := map[string]string{}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return ids
	}
	for _, sv := range g.Services {
		if _, ok := hashes[sv.Name]; !ok {
			continue
		}
		ref := sv.Image
		if ref == "" && sv.Build {
			ref = project + "-" + sv.Name
		}
		if ref == "" {
			continue
		}
		if res, err := cli.ImageInspect(ctx, ref); err == nil && res.ID != "" {
			ids[sv.Name] = res.ID
		}
	}
	return ids
}
