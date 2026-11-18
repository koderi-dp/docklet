package services

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"docklet/internal/buildkit"
	"docklet/internal/compose"
	"docklet/internal/model"
)

const capabilityTTL = 60 * time.Second

type capEntry struct {
	cap model.BuildCapability
	at  time.Time
}

func (s *ImageService) run() compose.Runner {
	if s.runner != nil {
		return s.runner
	}
	return compose.ExecRunner{}
}

func (s *ImageService) execEnv(connectionID string) (compose.ExecEnv, error) {
	if s.Profiles == nil {
		return compose.ExecEnv{}, fmt.Errorf("no connection profiles configured")
	}
	prof, err := s.Profiles.Profile(connectionID)
	if err != nil {
		return compose.ExecEnv{}, err
	}
	return compose.SelectEnv(prof), nil
}

// BuildCapability reports whether this connection can build with BuildKit through the
// docker CLI (and which CLI), cached for a minute per connection.
func (s *ImageService) BuildCapability(ctx context.Context, connectionID string) (model.BuildCapability, error) {
	c, _, err := s.capability(ctx, connectionID)
	return c, err
}

func (s *ImageService) capability(ctx context.Context, connectionID string) (model.BuildCapability, compose.ExecEnv, error) {
	env, err := s.execEnv(connectionID)
	if err != nil {
		return model.BuildCapability{}, env, err
	}
	key := connectionID + "|" + string(env.Kind) + "|" + env.Distro + "|" + strings.Join(env.EnvSet, ";")
	s.capMu.Lock()
	if e, ok := s.caps[key]; ok && time.Since(e.at) < capabilityTTL {
		s.capMu.Unlock()
		return e.cap, env, nil
	}
	s.capMu.Unlock()
	var c model.BuildCapability
	podman, known := s.isPodman(ctx, connectionID)
	if podman {
		c = podmanCapability(env.Location)
	} else {
		c = s.probeCapability(ctx, env)
	}
	if !known {
		return c, env, nil // the engine could not be asked: do not remember an answer that may be wrong
	}
	s.capMu.Lock()
	if s.caps == nil {
		s.caps = map[string]capEntry{}
	}
	s.caps[key] = capEntry{cap: c, at: time.Now()}
	s.capMu.Unlock()
	return c, env, nil
}

// cliCommand builds `docker <args>` for the connection's execution environment.
func cliCommand(env compose.ExecEnv, args ...string) (compose.Command, bool) {
	switch env.Kind {
	case compose.KindLocal:
		return compose.Command{Name: "docker", Args: args, EnvSet: env.EnvSet, EnvUnset: env.EnvUnset}, true
	case compose.KindWSL:
		if compose.ValidateDistro(env.Distro) != nil {
			return compose.Command{}, false
		}
		return compose.Command{Name: "wsl.exe", Args: append([]string{"-d", env.Distro, "--exec", "docker"}, args...)}, true
	}
	return compose.Command{}, false
}

func (s *ImageService) probeCapability(ctx context.Context, env compose.ExecEnv) model.BuildCapability {
	var c model.BuildCapability
	if env.Kind != compose.KindLocal && env.Kind != compose.KindWSL {
		c = model.BuildCapability{Reason: "the docker CLI cannot be used for this connection type"}
		c.Summary = "Classic builder: " + c.Reason
	} else {
		var p buildkit.Probe
		run := func(args ...string) (string, bool) {
			cmd, ok := cliCommand(env, args...)
			if !ok {
				return "", false
			}
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			out, err := s.run().Output(cctx, cmd)
			return out, err == nil || out != ""
		}
		if out, ok := run("buildx", "version"); ok {
			p.Buildx, p.HasBuildx = buildkit.ParseBuildxVersion(out)
		}
		if !p.HasBuildx {
			if out, ok := run("version", "--format", "{{.Client.Version}}"); ok {
				p.Docker, p.HasDocker = buildkit.ParseDockerVersion(out)
			}
		}
		c = buildkit.Decide(p)
	}
	c.Location = env.Location
	if env.Kind == compose.KindWSL {
		c.ContextHint = "The docker CLI runs inside " + env.Location + ", so the build context must be a Linux path there (for example /home/you/app, or /mnt/c/... for a Windows drive)."
	}
	return c
}

var winDriveRe = regexp.MustCompile(`^([A-Za-z]):[\\/]`)

// wslContextPath converts a typed or picked folder into a Linux path inside the distribution. Windows drive
// paths are not translated silently; the error suggests the /mnt/<drive> form.
func wslContextPath(env compose.ExecEnv, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if m := winDriveRe.FindStringSubmatch(raw); m != nil {
		hint := "/mnt/" + strings.ToLower(m[1]) + "/" + strings.ReplaceAll(raw[3:], `\`, "/")
		return "", fmt.Errorf("\"%s\" is a Windows folder. The docker CLI for this connection runs inside %s and cannot read it; type its Linux path (for example %s) or use the Classic builder, which uploads the folder through the API", raw, env.Location, hint)
	}
	p, err := compose.ConvertWSLPath(env.Distro, raw)
	if err != nil {
		return "", err
	}
	return p, nil
}

// cliBuild is a build context as the docker CLI will see it.
type cliBuild struct {
	validatedBuild
	DockerfileFull string // path of the Dockerfile as the CLI sees it
}

// resolveCLIContext validates the folder and Dockerfile where the CLI runs (this
// computer, or inside the WSL distribution).
func (s *ImageService) resolveCLIContext(ctx context.Context, env compose.ExecEnv, spec model.BuildSpec) (cliBuild, error) {
	if env.Kind == compose.KindWSL {
		var b cliBuild
		if strings.TrimSpace(spec.ContextDir) == "" {
			return b, fmt.Errorf("choose a build context directory")
		}
		dir, err := wslContextPath(env, spec.ContextDir)
		if err != nil {
			return b, err
		}
		rel, err := cleanDockerfileRel(spec.Dockerfile)
		if err != nil {
			return b, err
		}
		fs := compose.WSLFS{Runner: s.run(), Distro: env.Distro}
		if !fs.DirExists(ctx, dir) {
			return b, fmt.Errorf("build context %s does not exist inside %s", dir, env.Location)
		}
		full := path.Join(dir, rel)
		if !fs.FileExists(ctx, full) {
			return b, fmt.Errorf("Dockerfile %q was not found in %s inside %s", rel, dir, env.Location)
		}
		b.ContextDir, b.Dockerfile, b.DockerfileFull = dir, rel, full
		return b, validateBuildOptions(spec, &b.validatedBuild)
	}
	v, err := validateBuildSpec(spec)
	if err != nil {
		return cliBuild{}, err
	}
	return cliBuild{validatedBuild: v, DockerfileFull: filepath.Join(v.ContextDir, filepath.FromSlash(v.Dockerfile))}, nil
}

// buildOptions turns a resolved build into CLI options.
func buildOptions(cap model.BuildCapability, b cliBuild, spec model.BuildSpec) buildkit.Options {
	o := buildkit.Options{
		Method: cap.Method, RawJSON: cap.RawJSON, ContextDir: b.ContextDir, Dockerfile: b.DockerfileFull,
		Tags: b.Tags, Target: b.Target, NoCache: b.NoCache, Pull: b.Pull, Platform: strings.TrimSpace(spec.Platform),
	}
	for _, k := range sortedArgNames(spec.BuildArgs) {
		o.Args = append(o.Args, buildkit.Arg{Name: strings.TrimSpace(k), Value: spec.BuildArgs[k]})
	}
	return o
}

// BuildPlan says which builder a build with this spec would use and whether the chosen folder can be
// used by it, so the dialog can explain itself before anything starts.
func (s *ImageService) BuildPlan(ctx context.Context, connectionID string, spec model.BuildSpec) (model.BuildPlan, error) {
	cap, env, err := s.capability(ctx, connectionID)
	if err != nil {
		return model.BuildPlan{}, err
	}
	plan := model.BuildPlan{Capability: cap, Builder: model.BuilderClassic, Summary: cap.Summary}
	switch spec.Builder {
	case model.BuilderClassic:
		plan.Summary = "Classic builder (chosen)"
		if cap.Available {
			plan.Summary += ". " + cap.Summary + " is available"
		}
	case model.BuilderBuildKit:
		if !cap.Available {
			plan.ContextProblem = "BuildKit is not available for this connection: " + cap.Reason
			plan.OfferClassic = true
		}
		fallthrough
	default:
		if cap.Available {
			plan.Builder = model.BuilderBuildKit
		}
	}
	if strings.TrimSpace(spec.ContextDir) == "" || plan.ContextProblem != "" {
		return plan, nil
	}
	// Only the folder and Dockerfile are checked here; the other fields are validated when the build starts.
	folder := model.BuildSpec{ContextDir: spec.ContextDir, Dockerfile: spec.Dockerfile}
	if plan.Builder == model.BuilderBuildKit {
		if _, err := s.resolveCLIContext(ctx, env, folder); err != nil {
			plan.ContextProblem = err.Error()
			_, cerr := validateBuildSpec(folder)
			plan.OfferClassic = cerr == nil
		}
	} else if _, err := validateBuildSpec(folder); err != nil {
		plan.ContextProblem = err.Error()
	}
	return plan, nil
}

// isPodman reports whether the connection's engine is Podman; known is false when it cannot be asked.
func (s *ImageService) isPodman(ctx context.Context, connectionID string) (podman, known bool) {
	if s.Engines == nil {
		return false, true // no engine provider (tests): the docker CLI probe is all there is
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return false, false
	}
	return engineKindOf(ctx, cli) == model.EnginePodman, true
}

// podmanCapability: Podman serves the classic build API only. `docker buildx` would build with the
// builder of the user's current Docker context, not on Podman, so BuildKit is never offered here.
func podmanCapability(location string) model.BuildCapability {
	const reason = "Podman builds through its Docker-compatible build API; `docker buildx` cannot build on it"
	return model.BuildCapability{Reason: reason, Summary: "Classic builder: Podman has no BuildKit endpoint", Location: location}
}
