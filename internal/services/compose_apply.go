package services

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"docklet/internal/compose"
	"docklet/internal/model"

	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
		return fail(planError(hashErr))
	}
	if jsonErr != nil {
		return fail(planError(jsonErr))
	}
	g, err := compose.ParseGraph(jsonOut)
	if err != nil {
		return fail(planError(err))
	}
	hashes := compose.ParseHashes(hashOut)
	all := make([]string, 0, len(g.Services))
	for _, sv := range g.Services {
		all = append(all, sv.Name)
	}
	return model.ComposeApplyPlan{Items: compose.PlanApply(compose.PlanInput{
		Hashes: hashes, AllServices: all, Containers: cs,
		ImageIDs: s.serviceImageIDs(ctx, connectionID, p.Name, g, hashes),
		Replicas: compose.ParseReplicas(jsonOut),
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

const maxPlanErrLines = 12

var (
	exitPrefixRe  = regexp.MustCompile(`^exit status \d+: `)
	warnLogLineRe = regexp.MustCompile(`^(time=.*level=warning|WARN\[)`)
)

// planError is a failed config run as the user should read it: no "exit status N: " prefix and
// no warning log lines (obsolete `version:`, unset variables) that would hide the real error.
func planError(err error) string {
	text := exitPrefixRe.ReplaceAllString(strings.TrimSpace(err.Error()), "")
	var keep []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" && !warnLogLineRe.MatchString(l) {
			keep = append(keep, l)
		}
	}
	if len(keep) == 0 {
		return strings.TrimSpace(text)
	}
	if len(keep) > maxPlanErrLines {
		keep = keep[:maxPlanErrLines]
	}
	return strings.Join(keep, "\n")
}

// serviceImageIDs asks the Engine which image ids count as each selected service's current
// image: the id of the reference and the id of its variant for the Engine's own platform
// (Compose stamps the latter, while the containerd image store answers the former with the
// index id). A service whose image is not present, cannot be named or has no platform id is
// left out.
func (s *ComposeService) serviceImageIDs(ctx context.Context, connectionID, project string, g model.ComposeGraph, hashes map[string]string) map[string][]string {
	ids := map[string][]string{}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return ids
	}
	ver, err := cli.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil || ver.Os == "" || ver.Arch == "" {
		return ids
	}
	plat := client.ImageInspectWithPlatform(&ocispec.Platform{OS: ver.Os, Architecture: ver.Arch})
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
		plain, err := cli.ImageInspect(ctx, ref)
		if err != nil || plain.ID == "" {
			continue
		}
		specific, err := cli.ImageInspect(ctx, ref, plat)
		if err != nil || specific.ID == "" {
			continue
		}
		ids[sv.Name] = []string{plain.ID, specific.ID}
	}
	return ids
}
