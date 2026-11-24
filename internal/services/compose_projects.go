package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
)

// ComposeProjectStore persists managed Compose projects (implemented by *appdb.DB).
type ComposeProjectStore interface {
	ListComposeProjects(ctx context.Context, connectionID string) ([]appdb.ComposeProjectRecord, error)
	SaveComposeProject(ctx context.Context, r appdb.ComposeProjectRecord) (appdb.ComposeProjectRecord, error)
	DeleteComposeProject(ctx context.Context, connectionID, name string) error
}

var errNoProjectStore = errors.New("Managed Compose projects are not available: the application database is not open")

func toManaged(r appdb.ComposeProjectRecord) compose.ManagedRecord {
	return compose.ManagedRecord{Name: r.Name, WorkingDir: r.WorkingDir, ConfigFiles: r.ConfigFiles, EnvFiles: r.EnvFiles}
}

// managed returns the saved records of a connection (nil when the store is not wired).
func (s *ComposeService) managed(ctx context.Context, connectionID string) ([]compose.ManagedRecord, error) {
	if s.Projects == nil {
		return nil, nil
	}
	recs, err := s.Projects.ListComposeProjects(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("could not read saved Compose projects: %w", err)
	}
	out := make([]compose.ManagedRecord, 0, len(recs))
	for _, r := range recs {
		out = append(out, toManaged(r))
	}
	return out, nil
}

// Location tells the frontend where the Compose CLI runs for a connection and how to
// write file paths there.
func (s *ComposeService) Location(ctx context.Context, connectionID string) (model.ComposeLocation, error) {
	env, err := s.env(connectionID)
	if err != nil {
		return model.ComposeLocation{}, err
	}
	loc := model.ComposeLocation{
		Available: env.Kind != compose.KindNone, Kind: string(env.Kind), ExecutionLocation: env.Location,
		Reason: env.Reason, Warning: env.Warning, ManagedAvailable: s.Projects != nil,
	}
	switch env.Kind {
	case compose.KindWSL:
		loc.PathNote = fmt.Sprintf("Files are read inside the WSL distribution %q. Paths are shown as Linux paths there, such as /home/user/app/compose.yaml. A file on a Windows drive works too: C:\\... becomes /mnt/c/...", env.Distro)
	case compose.KindLocal:
		loc.PathNote = "Files are read from this machine; use absolute paths."
	}
	return loc, nil
}

// AddProject saves a managed project after validating, in the connection's execution
// location, that the files exist and that `docker compose config -q` accepts them.
func (s *ComposeService) AddProject(ctx context.Context, connectionID string, configFiles, envFiles []string, name string) (model.ComposeProject, error) {
	if s.Projects == nil {
		return model.ComposeProject{}, errNoProjectStore
	}
	env, err := s.env(connectionID)
	if err != nil {
		return model.ComposeProject{}, err
	}
	if env.Kind == compose.KindNone {
		return model.ComposeProject{}, errors.New(env.Reason)
	}
	cfg, err := s.normalizeFiles(ctx, env, configFiles, "Compose file")
	if err != nil {
		return model.ComposeProject{}, err
	}
	if len(cfg) == 0 {
		return model.ComposeProject{}, errors.New("choose at least one Compose file")
	}
	envf, err := s.normalizeFiles(ctx, env, envFiles, "env file")
	if err != nil {
		return model.ComposeProject{}, err
	}
	fs := s.fsFor(env)
	var missing []string
	for _, f := range append(append([]string{}, cfg...), envf...) {
		if !fs.FileExists(ctx, f) {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return model.ComposeProject{}, fmt.Errorf("file(s) not found on %s: %s", env.Location, strings.Join(missing, ", "))
	}
	if st := s.cliStatus(ctx, env); !st.OK {
		return model.ComposeProject{}, errors.New(st.Detail)
	}
	dir := compose.DirOf(env.Kind, cfg[0])

	name = strings.TrimSpace(name)
	if name == "" {
		name = s.detectName(ctx, env, cfg, envf, dir)
	}
	if err := compose.ValidateProjectName(name); err != nil {
		return model.ComposeProject{}, fmt.Errorf("%w (lower-case letters, digits, - and _; must start with a letter or digit)", err)
	}
	if err := s.validateConfig(ctx, env, name, cfg, envf, dir); err != nil {
		return model.ComposeProject{}, err
	}
	if _, err := s.Projects.SaveComposeProject(ctx, appdb.ComposeProjectRecord{
		ConnectionID: connectionID, Name: name, WorkingDir: dir, ConfigFiles: cfg, EnvFiles: envf,
	}); err != nil {
		return model.ComposeProject{}, fmt.Errorf("could not save the project: %w", err)
	}
	p, _, err := s.project(ctx, connectionID, name)
	return p, err
}

// validateConfig runs `docker compose config -q` on the files where the CLI runs.
func (s *ComposeService) validateConfig(ctx context.Context, env compose.ExecEnv, name string, cfg, envf []string, dir string) error {
	req := compose.Request{Project: name, ConfigFiles: cfg, EnvFiles: envf, WorkingDir: dir, Verb: compose.VerbConfig, ConfigMode: "quiet"}
	cmd, err := compose.BuildCommand(env, req)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err := s.run().Output(cctx, cmd); err != nil {
		return fmt.Errorf("the Compose files are not valid (docker compose config -q): %w", err)
	}
	return nil
}

// RemoveProject forgets a managed project record. It never runs Compose and never
// touches containers, networks, volumes or files.
func (s *ComposeService) RemoveProject(ctx context.Context, connectionID, name string) error {
	if s.Projects == nil {
		return errNoProjectStore
	}
	if name == "" {
		return errors.New("project name is required")
	}
	return s.Projects.DeleteComposeProject(ctx, connectionID, name)
}

// detectName asks `docker compose config --format json` for the project name and falls
// back to the normalised directory name (the Compose default).
func (s *ComposeService) detectName(ctx context.Context, env compose.ExecEnv, cfg, envf []string, dir string) string {
	fallback := compose.NormalizeProjectName(baseName(env, dir))
	cmd, err := compose.BuildCommand(env, compose.Request{ConfigFiles: cfg, EnvFiles: envf, WorkingDir: dir, Verb: compose.VerbConfig, ConfigMode: "json"})
	if err != nil {
		return fallback
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := s.run().Output(cctx, cmd)
	if err != nil {
		return fallback
	}
	var v struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(out), &v) == nil && compose.ValidateProjectName(v.Name) == nil {
		return v.Name
	}
	return fallback
}

func baseName(env compose.ExecEnv, dir string) string {
	d := strings.TrimRight(strings.ReplaceAll(dir, "\\", "/"), "/")
	if i := strings.LastIndex(d, "/"); i >= 0 {
		d = d[i+1:]
	}
	return d
}

// configServices lists service names from the Compose files (cached for a short time).
func (s *ComposeService) configServices(ctx context.Context, env compose.ExecEnv, p model.ComposeProject) []string {
	key := "svc|" + p.ConnectionID + "|" + p.Name + "|" + strings.Join(p.ConfigFiles, ";") + "|" + strings.Join(p.EnvFiles, ";")
	s.cliMu.Lock()
	if e, ok := s.svcCache[key]; ok && time.Since(e.at) < cliCacheTTL {
		s.cliMu.Unlock()
		return e.names
	}
	s.cliMu.Unlock()
	req := requestFor(p, compose.VerbConfig)
	req.ConfigMode = "services"
	cmd, err := compose.BuildCommand(env, req)
	if err != nil {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := s.run().Output(cctx, cmd)
	if err != nil {
		return nil
	}
	var names []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			names = append(names, l)
		}
	}
	s.cliMu.Lock()
	if s.svcCache == nil {
		s.svcCache = map[string]svcEntry{}
	}
	s.svcCache[key] = svcEntry{names: names, at: time.Now()}
	s.cliMu.Unlock()
	return names
}

type svcEntry struct {
	names []string
	at    time.Time
}
