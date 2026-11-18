package services

import (
	"context"
	"fmt"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/runspec"
)

// CopyAsService generates "Copy as…" text (docker run, Compose, connection strings) from a container's
// inspect data. Secret-looking environment values are only returned when the caller asks for them.
type CopyAsService struct {
	Engines  engine.Provider
	Profiles engine.ProfileSource
}

// loaded is everything one generation reads from the Engine.
type loaded struct {
	in    container.InspectResponse
	img   *image.InspectResponse
	hints runspec.Hints
}

func (s *CopyAsService) load(ctx context.Context, connectionID, id string) (loaded, error) {
	if s.Engines == nil {
		return loaded{}, fmt.Errorf("no engine provider configured")
	}
	cli, err := s.Engines.Client(ctx, connectionID)
	if err != nil {
		return loaded{}, err
	}
	if cli == nil {
		return loaded{}, fmt.Errorf("connection %q is not available", connectionID)
	}
	res, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return loaded{}, friendlyError(err)
	}
	l := loaded{in: res.Container, hints: runspec.Hints{IsSecret: isSecretKey}}
	if ir, ierr := cli.ImageInspect(ctx, res.Container.Image); ierr == nil {
		l.img = &ir.InspectResponse
	}
	if info, ierr := cli.Info(ctx, client.InfoOptions{}); ierr == nil {
		l.hints.DaemonLogDriver = info.Info.LoggingDriver
	}
	return l, nil
}

func (s *CopyAsService) spec(ctx context.Context, connectionID, id string) (runspec.Spec, container.InspectResponse, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	l, err := s.load(ctx, connectionID, id)
	if err != nil {
		return runspec.Spec{}, l.in, err
	}
	return runspec.FromInspect(l.in, l.img, l.hints), l.in, nil
}

// Info reports which "Copy as" entries apply to the container.
func (s *CopyAsService) Info(ctx context.Context, connectionID, id string) (model.CopyAsInfo, error) {
	sp, in, err := s.spec(ctx, connectionID, id)
	if err != nil {
		return model.CopyAsInfo{}, err
	}
	info := model.CopyAsInfo{SecretKeys: nonNil(sp.SecretKeys())}
	if ci, ok := runspec.ConnectionStrings(runspec.ConnInputFromInspect(in), runspec.ConnContext{}, false); ok {
		info.ConnectionKind, info.ConnectionTitle = ci.Kind, ci.Title
	}
	return info, nil
}

// DockerRun renders the container as a `docker run` command for the shell ("bash" or "powershell").
func (s *CopyAsService) DockerRun(ctx context.Context, connectionID, id, shell string, includeSecrets bool) (model.CopyAsResult, error) {
	sp, _, err := s.spec(ctx, connectionID, id)
	if err != nil {
		return model.CopyAsResult{}, err
	}
	text := runspec.ToDockerRun(sp, runspec.Options{Shell: runspec.ParseShell(shell), IncludeSecrets: includeSecrets})
	return model.CopyAsResult{Text: text, SecretKeys: nonNil(sp.SecretKeys()), NotIncluded: nonNil(sp.NotIncluded)}, nil
}

// Compose renders the container as a Compose service snippet.
func (s *CopyAsService) Compose(ctx context.Context, connectionID, id string, includeSecrets bool) (model.CopyAsResult, error) {
	sp, _, err := s.spec(ctx, connectionID, id)
	if err != nil {
		return model.CopyAsResult{}, err
	}
	text := runspec.ToCompose(sp, runspec.Options{IncludeSecrets: includeSecrets})
	return model.CopyAsResult{Text: text, SecretKeys: nonNil(sp.SecretKeys()), NotIncluded: nonNil(sp.NotIncluded)}, nil
}

// ConnectionStrings builds connection strings for a recognised database container. The password is a
// placeholder unless includePassword is set.
func (s *CopyAsService) ConnectionStrings(ctx context.Context, connectionID, id string, includePassword bool) (model.ConnectionStrings, error) {
	_, in, err := s.spec(ctx, connectionID, id)
	if err != nil {
		return model.ConnectionStrings{}, err
	}
	var cc runspec.ConnContext
	if s.Profiles != nil {
		if p, perr := s.Profiles.Profile(connectionID); perr == nil {
			cc = runspec.ConnContext{Type: string(p.Type), Endpoint: p.Endpoint}
		}
	}
	ci, ok := runspec.ConnectionStrings(runspec.ConnInputFromInspect(in), cc, includePassword)
	if !ok {
		return model.ConnectionStrings{}, fmt.Errorf("This container is not a recognised database, so there is no connection string for it.")
	}
	out := model.ConnectionStrings{Kind: ci.Kind, Title: ci.Title, Host: ci.Host, Reachable: ci.Reachable, Note: ci.Note,
		PasswordEnv: ci.PasswordEnv, Entries: []model.ConnectionEntry{}}
	for _, e := range ci.Entries {
		out.Entries = append(out.Entries, model.ConnectionEntry{Label: e.Label, Text: e.Text, UsesPassword: e.UsesPassword, Internal: e.Internal})
		if e.UsesPassword && includePassword && ci.PasswordEnv != "" {
			out.PasswordIn = true
		}
	}
	return out, nil
}
