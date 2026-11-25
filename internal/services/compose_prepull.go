package services

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/compose"
	"docklet/internal/ops"
	"docklet/internal/registryauth"
)

// The Compose CLI knows only the registry sign-ins of the place it runs in (inside a WSL
// distribution, that distribution's own Docker config). The sign-ins saved in Docklet never reach
// it, so before `up` and `pull` Docklet pulls through the Engine, with its own credentials, the
// images it has a sign-in for. Images without a sign-in stay with the CLI, as do built services.

func (s *ComposeService) registry() *registryauth.Resolver {
	if s.Registry != nil {
		return s.Registry
	}
	return registryauth.DefaultResolver()
}

// noProgress keeps a pull's byte progress out of the operation: several images are pulled one
// after another and the CLI follows, so a percentage would jump back and forth.
type noProgress struct{ *ops.Handle }

func (noProgress) Progress(float64) {}

// prePull pulls the images of the request that have a registry sign-in. For `up` only images that
// are missing (or marked pull_policy: always) are pulled; for `pull` every one of them. It returns
// the services whose image it pulled and, for `pull`, the services still left to the CLI.
// Anything it cannot find out (unreadable config, unreachable Engine) leaves the work to the CLI.
func (s *ComposeService) prePull(ctx context.Context, connectionID string, env compose.ExecEnv, r compose.Request, h *ops.Handle) (pulled, left []string, err error) {
	if r.Verb != compose.VerbUp && r.Verb != compose.VerbPull {
		return nil, nil, nil
	}
	cfg := compose.Request{
		Project: r.Project, ConfigFiles: r.ConfigFiles, EnvFiles: r.EnvFiles, WorkingDir: r.WorkingDir,
		Profiles: r.Profiles, Verb: compose.VerbConfig, ConfigMode: "json",
	}
	out, cerr := s.configOutput(ctx, env, cfg)
	if cerr != nil {
		return nil, nil, nil
	}
	needs, perr := compose.ParseImageNeeds(out, r.Services)
	if perr != nil {
		return nil, nil, nil
	}
	cli, cerr := s.client(ctx, connectionID)
	if cerr != nil {
		return nil, nil, nil
	}
	done := map[string]bool{} // image -> pulled here
	seen := map[string]bool{}
	for _, n := range needs {
		if !seen[n.Image] {
			seen[n.Image] = true
			ok, err := s.pullWithSignIn(ctx, cli, r.Verb, n, h)
			if err != nil {
				return nil, nil, err
			}
			done[n.Image] = ok
		}
		if done[n.Image] {
			pulled = append(pulled, n.Service)
		} else {
			left = append(left, n.Service)
		}
	}
	return pulled, left, nil
}

// pullWithSignIn pulls one image when a sign-in exists for its registry and the verb asks for it.
// It reports false when the image is left to the CLI.
func (s *ComposeService) pullWithSignIn(ctx context.Context, cli *client.Client, verb compose.Verb, n compose.ImageNeed, h *ops.Handle) (bool, error) {
	if n.PullPolicy == "never" || n.PullPolicy == "build" {
		return false, nil
	}
	creds, err := s.registry().Resolve(ctx, n.Image)
	if err != nil || !creds.Has() {
		return false, nil
	}
	if verb == compose.VerbUp && n.PullPolicy != "always" {
		ictx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, ierr := cli.ImageInspect(ictx, n.Image)
		cancel()
		if ierr == nil {
			return false, nil // already here: Compose starts it without pulling
		}
	}
	h.Log(fmt.Sprintf("Pulling %s with the sign-in for %s (%s)", n.Image, creds.Registry, creds.Source))
	resp, err := cli.ImagePull(ctx, n.Image, client.ImagePullOptions{RegistryAuth: creds.HeaderValue()})
	if err != nil {
		err = imageError(err)
	} else {
		err = consumePull(ctx, resp.JSONMessages(ctx), noProgress{h})
		_ = resp.Close()
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return false, fmt.Errorf("could not pull %s: %w", n.Image, registryAuthError(err, creds, n.Image, true))
	}
	return true, nil
}
