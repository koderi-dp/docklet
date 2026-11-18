package services

import (
	"context"
	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
	"docklet/internal/ops"
	"fmt"
	"strings"
	"sync"
)

func requestFor(p model.ComposeProject, verb compose.Verb) compose.Request {
	return compose.Request{Project: p.Name, ConfigFiles: p.ConfigFiles, EnvFiles: p.EnvFiles, WorkingDir: p.WorkingDir, Verb: verb}
}

// Up runs `docker compose up -d` (optionally --build / --remove-orphans / --profile).
func (s *ComposeService) Up(ctx context.Context, connectionID, name string, opts model.ComposeUpOptions) (model.Operation, error) {
	return s.start(ctx, connectionID, name, upRequest(opts, nil), "Compose up")
}

// UpService runs `docker compose up -d <service>`.
func (s *ComposeService) UpService(ctx context.Context, connectionID, name, service string, opts model.ComposeUpOptions) (model.Operation, error) {
	return s.start(ctx, connectionID, name, upRequest(opts, []string{service}), "Compose up")
}

func upRequest(opts model.ComposeUpOptions, services []string) compose.Request {
	return compose.Request{Verb: compose.VerbUp, Build: opts.Build, RemoveOrphans: opts.RemoveOrphans, Profiles: opts.Profiles, Services: services}
}

// Down runs `docker compose down`. It removes containers and networks; volumes are
// removed ONLY when opts.Volumes is explicitly true.
func (s *ComposeService) Down(ctx context.Context, connectionID, name string, opts model.ComposeDownOptions) (model.Operation, error) {
	r := compose.Request{Verb: compose.VerbDown, Volumes: opts.Volumes, RemoveOrphans: opts.RemoveOrphans, Profiles: opts.Profiles}
	return s.start(ctx, connectionID, name, r, "Compose down")
}

// Start runs `docker compose start`.
func (s *ComposeService) Start(ctx context.Context, connectionID, name string) (model.Operation, error) {
	return s.start(ctx, connectionID, name, compose.Request{Verb: compose.VerbStart}, "Compose start")
}

// Stop runs `docker compose stop` (never down).
func (s *ComposeService) Stop(ctx context.Context, connectionID, name string) (model.Operation, error) {
	return s.start(ctx, connectionID, name, compose.Request{Verb: compose.VerbStop}, "Compose stop")
}

// Restart runs `docker compose restart`.
func (s *ComposeService) Restart(ctx context.Context, connectionID, name string) (model.Operation, error) {
	return s.start(ctx, connectionID, name, compose.Request{Verb: compose.VerbRestart}, "Compose restart")
}

// RestartService runs `docker compose restart <service>`.
func (s *ComposeService) RestartService(ctx context.Context, connectionID, name, service string) (model.Operation, error) {
	return s.start(ctx, connectionID, name, compose.Request{Verb: compose.VerbRestart, Services: []string{service}}, "Compose restart")
}

// Pull runs `docker compose pull`.
func (s *ComposeService) Pull(ctx context.Context, connectionID, name string) (model.Operation, error) {
	return s.start(ctx, connectionID, name, compose.Request{Verb: compose.VerbPull}, "Compose pull")
}

// Build runs `docker compose build`.
func (s *ComposeService) Build(ctx context.Context, connectionID, name string) (model.Operation, error) {
	return s.start(ctx, connectionID, name, compose.Request{Verb: compose.VerbBuild}, "Compose build")
}

func svcSuffix(services []string) string {
	if len(services) == 0 {
		return ""
	}
	return " / " + strings.Join(services, ", ")
}

// start validates, resolves the project from Engine labels or its managed record (never from caller-supplied
// paths), checks the capability and launches the CLI as a cancellable operation.
func (s *ComposeService) start(ctx context.Context, connectionID, name string, r compose.Request, title string) (model.Operation, error) {
	if s.Ops == nil {
		return model.Operation{}, fmt.Errorf("operation manager unavailable")
	}
	if err := compose.ValidateProjectName(name); err != nil {
		return model.Operation{}, err
	}
	p, env, err := s.project(ctx, connectionID, name)
	if err != nil {
		return model.Operation{}, err
	}
	if !p.Capability.CanRun {
		return model.Operation{}, fmt.Errorf("Compose %s is not available for %q: %s", r.Verb, name, p.Capability.Reason)
	}
	r.Project, r.ConfigFiles, r.EnvFiles, r.WorkingDir = p.Name, p.ConfigFiles, p.EnvFiles, p.WorkingDir
	if r.Verb == compose.VerbDown && !p.Managed && s.Projects != nil && len(p.ConfigFiles) > 0 {
		// Down removes the containers a discovered project is known from; keep its files as a
		// managed project so it stays listed ("Not created") and can be brought Up again.
		_, _ = s.Projects.SaveComposeProject(ctx, appdb.ComposeProjectRecord{
			ConnectionID: connectionID, Name: p.Name, WorkingDir: p.WorkingDir, ConfigFiles: p.ConfigFiles, EnvFiles: p.EnvFiles,
		})
	}
	cmd, err := compose.BuildCommand(env, r)
	if err != nil {
		return model.Operation{}, err
	}
	runner := s.run()
	op := s.Ops.Start(ops.Spec{
		Kind: "compose." + string(r.Verb), Title: title + " " + name + svcSuffix(r.Services),
		ConnectionID: connectionID, Resource: name, Cancellable: true,
	}, func(ctx context.Context, h *ops.Handle) error {
		defer s.emitChanged(connectionID)
		h.Log("$ " + cmd.Name + " " + strings.Join(cmd.Args, " "))
		var tail errTail
		if err := runner.Run(ctx, cmd, func(l string) { tail.add(l); h.Log(l) }); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if m := tail.message(); m != "" {
				return fmt.Errorf("%s failed: %s (%w)", title, m, err)
			}
			return fmt.Errorf("%s failed: %w", title, err)
		}
		return nil
	})
	return op, nil
}

// errTail remembers the most informative recent CLI output line (the last one that
// mentions an error, else the last line) so a failed operation reports the CLI's reason.
type errTail struct {
	mu        sync.Mutex
	last, err string
}

func (t *errTail) add(l string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.last = l
	if strings.Contains(strings.ToLower(l), "error") {
		t.err = l
	}
}

func (t *errTail) message() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != "" {
		return t.err
	}
	return t.last
}
