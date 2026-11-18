package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/buildkit"
	"docklet/internal/compose"
	"docklet/internal/model"
	"docklet/internal/ops"
)

// startCLIBuild runs the build through the docker CLI where the connection's CLI lives (this computer or
// inside WSL) as a cancellable operation. The Engine receives the image with --load.
func (s *ImageService) startCLIBuild(ctx context.Context, connectionID string, spec model.BuildSpec, cap model.BuildCapability, env compose.ExecEnv) (model.Operation, error) {
	b, err := s.resolveCLIContext(ctx, env, spec)
	if err != nil {
		return model.Operation{}, err
	}
	opts := buildOptions(cap, b, spec)
	cmd, err := buildkit.Command(env, opts, os.Getenv("WSLENV"))
	if err != nil {
		return model.Operation{}, err
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	if s.Ops == nil {
		return model.Operation{}, fmt.Errorf("operations are not available")
	}
	live := buildkit.NewScrubber(buildkit.SecretValues(opts.Args), 3)
	hide := buildkit.NewScrubber(buildkit.AllValues(opts.Args), 4)
	meta := buildMeta{
		connectionID: connectionID, builder: model.BuilderBuildKit, method: methodLabel(cap), contextDir: b.ContextDir,
		dockerfile: b.Dockerfile, tags: b.Tags, target: b.Target, platform: opts.Platform, noCache: b.NoCache, pull: b.Pull,
		command: buildkit.CopyCommand(opts), started: time.Now(), hide: hide,
	}
	for _, a := range opts.Args {
		meta.argNames = append(meta.argNames, a.Name)
	}

	title, resource := "Build "+dirBase(b.ContextDir), dirBase(b.ContextDir)
	if len(b.Tags) > 0 {
		title, resource = "Build "+b.Tags[0], b.Tags[0]
	}
	runner := s.run()
	op := s.Ops.Start(ops.Spec{Kind: "image.build", Title: title, ConnectionID: connectionID, Resource: resource, Cancellable: true},
		func(octx context.Context, h *ops.Handle) error {
			defer s.emitChanged(connectionID)
			tracker := buildkit.NewTracker(live)
			tail := &tailBuffer{}
			tracker.SetSink(func(l string) { tail.Add(l); h.Log(l) })
			s.live.add(h.ID(), tracker)
			defer s.live.finish(h.ID())

			h.Log("$ " + meta.command)
			h.Log(cap.Summary + " (" + env.Location + ")")
			lastDetail := ""
			runErr := runner.Run(octx, cmd, func(line string) {
				tracker.Feed(line)
				if d := stepSummary(tracker.Status()); d != lastDetail {
					lastDetail = d
					h.Detail(d)
				}
			})
			cancelled := runErr != nil && octx.Err() != nil
			tracker.Finish(cancelled)
			snap := tracker.Snapshot(h.ID(), false)
			out := buildOutcome{cancelled: cancelled, steps: snap.Steps, stepsTotal: snap.Total, stepsCached: snap.Cached, tail: tail.Lines()}
			var result error
			switch {
			case cancelled:
				result = octx.Err()
				h.Log("Build cancelled")
			case runErr != nil:
				reason := tracker.ErrorMessage()
				if reason == "" {
					reason = runErr.Error()
				}
				result = errors.New("Build failed: " + hide.Apply(reason))
				out.err = result
			default:
				out.imageID = s.imageIDOf(cli, b.Tags)
				if out.imageID != "" {
					h.Log("Built image " + out.imageID)
				}
			}
			s.saveBuild(meta, out, h.Log)
			return result
		})
	return op, nil
}

func methodLabel(c model.BuildCapability) string {
	if c.Method == buildkit.MethodDockerBuild {
		return "docker build " + c.Version
	}
	return "buildx " + c.Version
}

func dirBase(p string) string {
	p = strings.TrimRight(strings.ReplaceAll(p, `\`, "/"), "/")
	if i := strings.LastIndex(p, "/"); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}

// imageIDOf looks up the image a build produced (by its first tag); "" when unknown.
func (s *ImageService) imageIDOf(cli *client.Client, tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := cli.ImageInspect(ctx, tags[0])
	if err != nil {
		return ""
	}
	return res.ID
}
