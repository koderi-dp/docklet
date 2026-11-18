package services

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/compose"
	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
)

// ComposeService is bound to the frontend. Projects are discovered from container
// labels of the selected Engine; the Compose CLI runs in its own execution
// environment (see internal/compose) and is separate from the Engine API.
type ComposeService struct {
	Engines engine.Provider
	Events  engine.Emitter
	Ops     *ops.Manager
	// Profiles tells Compose which transport a connection uses (where the CLI must run).
	Profiles engine.ProfileSource

	// Projects persists managed projects (user-selected Compose files). Nil disables
	// the managed-project features; discovery from labels keeps working.
	Projects ComposeProjectStore

	// runner executes CLI commands; nil means the real process runner (tests inject a fake).
	runner compose.Runner

	cliMu    sync.Mutex
	cliCache map[string]cliEntry
	svcCache map[string]svcEntry
}

type cliEntry struct {
	st compose.CLIStatus
	at time.Time
}

const cliCacheTTL = 30 * time.Second

func (s *ComposeService) run() compose.Runner {
	if s.runner != nil {
		return s.runner
	}
	return compose.ExecRunner{}
}

func (s *ComposeService) client(ctx context.Context, connectionID string) (*client.Client, error) {
	if s.Engines == nil {
		return nil, fmt.Errorf("no engine provider configured")
	}
	cli, err := s.Engines.Client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if cli == nil {
		return nil, fmt.Errorf("connection %q is not available", connectionID)
	}
	return cli, nil
}

func (s *ComposeService) emitChanged(connectionID string) {
	if s.Events != nil {
		s.Events.Emit(model.EventContainersChanged, model.ContainersChanged{ConnectionID: connectionID})
	}
}

// labelled lists containers of the Engine that carry the Compose project label
// (only the given project when name != "").
func (s *ComposeService) labelled(ctx context.Context, connectionID, name string) ([]compose.Container, error) {
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	f := client.Filters{}
	if name != "" {
		f = f.Add("label", compose.LabelProject+"="+name)
	} else {
		f = f.Add("label", compose.LabelProject)
	}
	res, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return nil, friendlyError(err)
	}
	rows := make([]model.Container, 0, len(res.Items))
	for _, it := range res.Items {
		rows = append(rows, mapSummary(connectionID, it))
	}
	classifyExited(ctx, cli, connectionID, rows, func(i int) bool { return isOneOffLabels(res.Items[i].Labels) })
	out := make([]compose.Container, 0, len(res.Items))
	for i, it := range res.Items {
		out = append(out, compose.Container{
			ID: it.ID, Name: firstName(it.Names), Image: it.Image, State: string(it.State),
			Created: it.Created, Labels: it.Labels, ExitCode: rows[i].ExitCode, RestartPolicy: rows[i].RestartPolicy,
		})
	}
	return out, nil
}

func firstName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return trimName(names[0])
}

func (s *ComposeService) env(connectionID string) (compose.ExecEnv, error) {
	if s.Profiles == nil {
		return compose.ExecEnv{}, fmt.Errorf("no connection profiles configured")
	}
	prof, err := s.Profiles.Profile(connectionID)
	if err != nil {
		return compose.ExecEnv{}, err
	}
	return compose.SelectEnv(prof), nil
}

func (s *ComposeService) cliStatus(ctx context.Context, env compose.ExecEnv) compose.CLIStatus {
	key := string(env.Kind) + "|" + env.Location + "|" + strings.Join(env.EnvSet, ";")
	s.cliMu.Lock()
	if e, ok := s.cliCache[key]; ok && time.Since(e.at) < cliCacheTTL {
		s.cliMu.Unlock()
		return e.st
	}
	s.cliMu.Unlock()
	st := compose.CheckCLI(ctx, s.run(), env)
	s.cliMu.Lock()
	if s.cliCache == nil {
		s.cliCache = map[string]cliEntry{}
	}
	s.cliCache[key] = cliEntry{st: st, at: time.Now()}
	s.cliMu.Unlock()
	return st
}

func (s *ComposeService) fsFor(env compose.ExecEnv) compose.FS {
	if env.Kind == compose.KindWSL {
		return compose.WSLFS{Runner: s.run(), Distro: env.Distro}
	}
	return compose.LocalFS{}
}

func (s *ComposeService) assess(ctx context.Context, env compose.ExecEnv, p *model.ComposeProject) {
	p.Capability = compose.Assess(ctx, compose.AssessInput{
		Env: env, HostOS: runtime.GOOS,
		ConfigFiles: p.ConfigFiles, EnvFiles: p.EnvFiles, WorkingDir: p.WorkingDir,
		FS:  s.fsFor(env),
		CLI: func() compose.CLIStatus { return s.cliStatus(ctx, env) },
	})
}

// ListProjects returns the projects discovered from container labels of the Engine merged
// with the managed (saved) projects of the connection. A project is operable only when
// Capability.CanRun.
func (s *ComposeService) ListProjects(ctx context.Context, connectionID string) ([]model.ComposeProject, error) {
	cs, err := s.labelled(ctx, connectionID, "")
	if err != nil {
		return nil, err
	}
	env, err := s.env(connectionID)
	if err != nil {
		return nil, err
	}
	recs, err := s.managed(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	projects := compose.Merge(connectionID, compose.Discover(connectionID, cs), recs, env)
	var wg sync.WaitGroup
	for i := range projects {
		wg.Add(1)
		go func(p *model.ComposeProject) {
			defer wg.Done()
			s.assess(ctx, env, p)
		}(&projects[i])
	}
	wg.Wait()
	return projects, nil
}

// GetProject returns one project (exact label match) or an error when no container carries it.
func (s *ComposeService) GetProject(ctx context.Context, connectionID, name string) (model.ComposeProject, error) {
	p, env, err := s.project(ctx, connectionID, name)
	if err == nil && p.Managed && p.Capability.CanRun {
		// Services defined in the files but without a container yet (never created).
		p = compose.AddConfigServices(p, s.configServices(ctx, env, p))
	}
	return p, err
}

func (s *ComposeService) project(ctx context.Context, connectionID, name string) (model.ComposeProject, compose.ExecEnv, error) {
	if name == "" {
		return model.ComposeProject{}, compose.ExecEnv{}, fmt.Errorf("project name is required")
	}
	cs, err := s.labelled(ctx, connectionID, name)
	if err != nil {
		return model.ComposeProject{}, compose.ExecEnv{}, err
	}
	env, err := s.env(connectionID)
	if err != nil {
		return model.ComposeProject{}, compose.ExecEnv{}, err
	}
	recs, err := s.managed(ctx, connectionID)
	if err != nil {
		return model.ComposeProject{}, env, err
	}
	for _, p := range compose.Merge(connectionID, compose.Discover(connectionID, cs), recs, env) {
		if p.Name == name {
			s.assess(ctx, env, &p)
			return p, env, nil
		}
	}
	return model.ComposeProject{}, env, fmt.Errorf("Compose project %q was not found on this Engine", name)
}

// Capability reports whether and where the Compose CLI can operate the project.
func (s *ComposeService) Capability(ctx context.Context, connectionID, name string) (model.ComposeCapability, error) {
	p, _, err := s.project(ctx, connectionID, name)
	return p.Capability, err
}

// Config returns `docker compose config` output (secret-like values masked) when the CLI
// can run, otherwise only the file list known from labels.
func (s *ComposeService) Config(ctx context.Context, connectionID, name string) (model.ComposeConfigResult, error) {
	p, env, err := s.project(ctx, connectionID, name)
	if err != nil {
		return model.ComposeConfigResult{}, err
	}
	res := model.ComposeConfigResult{Files: p.ConfigFiles}
	if !p.Capability.CanRun {
		res.Note = p.Capability.Reason
		return res, nil
	}
	cmd, err := compose.BuildCommand(env, requestFor(p, compose.VerbConfig))
	if err != nil {
		return res, err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := s.run().Output(cctx, cmd)
	if err != nil {
		return res, fmt.Errorf("docker compose config failed: %w", err)
	}
	res.Rendered = maskComposeConfig(out)
	res.Available = true
	return res, nil
}

// Resources returns the networks and volumes labelled com.docker.compose.project=<name>.
func (s *ComposeService) Resources(ctx context.Context, connectionID, name string) (model.ComposeResources, error) {
	out := model.ComposeResources{Networks: []model.ComposeNetwork{}, Volumes: []model.ComposeVolume{}}
	if name == "" {
		return out, fmt.Errorf("project name is required")
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return out, err
	}
	f := client.Filters{}.Add("label", compose.LabelProject+"="+name)
	nets, err := cli.NetworkList(ctx, client.NetworkListOptions{Filters: f})
	if err != nil {
		return out, friendlyError(err)
	}
	for _, n := range nets.Items {
		out.Networks = append(out.Networks, model.ComposeNetwork{ID: n.ID, Name: n.Name, Driver: n.Driver, Scope: n.Scope})
	}
	vols, err := cli.VolumeList(ctx, client.VolumeListOptions{Filters: f})
	if err != nil {
		return out, friendlyError(err)
	}
	for _, v := range vols.Items {
		out.Volumes = append(out.Volumes, model.ComposeVolume{Name: v.Name, Driver: v.Driver, Mountpoint: v.Mountpoint})
	}
	sort.Slice(out.Networks, func(i, j int) bool { return out.Networks[i].Name < out.Networks[j].Name })
	sort.Slice(out.Volumes, func(i, j int) bool { return out.Volumes[i].Name < out.Volumes[j].Name })
	return out, nil
}

// ---- Verbs (background operations) ----
