package services

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"docklet/internal/compose"
	"docklet/internal/model"
	"docklet/internal/ops"
)

const (
	noWatchRulesReason = "No develop.watch rules in this project's Compose file"
	watchStopWait      = 5 * time.Second
)

func watchKey(connectionID, project string) string { return connectionID + "\x00" + project }

// WatchInfo reports whether `docker compose watch` can run for the project and which
// develop.watch rules it would use, read from the resolved configuration.
func (s *ComposeService) WatchInfo(ctx context.Context, connectionID, name string) (model.ComposeWatchInfo, error) {
	p, env, err := s.project(ctx, connectionID, name)
	if err != nil {
		return model.ComposeWatchInfo{Services: []model.ComposeWatchService{}}, err
	}
	return s.watchInfo(ctx, p, env), nil
}

func (s *ComposeService) watchInfo(ctx context.Context, p model.ComposeProject, env compose.ExecEnv) model.ComposeWatchInfo {
	info := model.ComposeWatchInfo{Services: []model.ComposeWatchService{}}
	if !p.Capability.CanRun {
		info.Reason = p.Capability.Reason
		return info
	}
	var (
		wg      sync.WaitGroup
		version [3]int
		vText   string
		vOK     bool
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if cmd, ok := compose.VersionCommand(env); ok {
			vctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			if out, err := s.run().Output(vctx, cmd); err == nil {
				version, vText, vOK = compose.ParseComposeVersion(out)
			}
		}
	}()
	rules, err := s.watchRules(ctx, p, env)
	wg.Wait()
	info.CLIVersion = vText
	if err != nil {
		info.Reason = "The Compose configuration could not be read: " + err.Error()
		return info
	}
	info.Services = rules
	for _, sv := range rules {
		info.RuleCount += len(sv.Rules)
	}
	switch {
	case info.RuleCount == 0:
		info.Reason = noWatchRulesReason
	case vOK && !compose.WatchSupported(version):
		info.Reason = fmt.Sprintf("The Compose CLI is too old for watch (found %s; watch needs 2.22 or newer)", vText)
	default:
		info.Available = true
	}
	return info
}

func (s *ComposeService) watchRules(ctx context.Context, p model.ComposeProject, env compose.ExecEnv) ([]model.ComposeWatchService, error) {
	req := requestFor(p, compose.VerbConfig)
	req.ConfigMode = "json"
	cmd, err := compose.BuildCommand(env, req)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := s.run().Output(cctx, cmd)
	if err != nil {
		return nil, err
	}
	return compose.ParseWatchRules([]byte(out), p.WorkingDir)
}

// WatchState returns the watch of the project: running, or ended on its own with the reason.
// Active is false when no watch was started (or it was stopped and dismissed).
func (s *ComposeService) WatchState(ctx context.Context, connectionID, name string) (model.ComposeWatchState, error) {
	if r := s.watchRunFor(connectionID, name); r != nil {
		return r.snapshot(), nil
	}
	return model.ComposeWatchState{Services: []string{}, Events: []model.ComposeWatchEvent{}, Raw: []string{}, Stats: []model.ComposeWatchStat{}}, nil
}

func (s *ComposeService) watchRunFor(connectionID, name string) *watchRun {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	return s.watches[watchKey(connectionID, name)]
}

// StartWatch runs `docker compose watch` for the project as a cancellable operation (kind
// compose.watch). One watch per project and connection; it keeps running until StopWatch or app exit.
func (s *ComposeService) StartWatch(ctx context.Context, connectionID, name string, opts model.ComposeWatchOptions) (model.Operation, error) {
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
		return model.Operation{}, fmt.Errorf("Compose watch is not available for %q: %s", name, p.Capability.Reason)
	}
	info := s.watchInfo(ctx, p, env)
	if !info.Available {
		return model.Operation{}, fmt.Errorf("Compose watch is not available for %q: %s", name, info.Reason)
	}
	for _, sv := range opts.Services {
		if !slices.ContainsFunc(info.Services, func(w model.ComposeWatchService) bool { return w.Name == sv }) {
			return model.Operation{}, fmt.Errorf("service %q has no develop.watch rules", sv)
		}
	}
	r := compose.Request{Verb: compose.VerbWatch, NoUp: opts.NoUp, Services: opts.Services}
	r.Project, r.ConfigFiles, r.EnvFiles, r.WorkingDir = p.Name, p.ConfigFiles, p.EnvFiles, p.WorkingDir
	cmd, err := compose.BuildCommand(env, r)
	if err != nil {
		return model.Operation{}, err
	}

	key := watchKey(connectionID, name)
	run := newWatchRun(connectionID, name, opts, time.Now(), func() { s.emitWatch(connectionID, name) })
	s.watchMu.Lock()
	if cur := s.watches[key]; cur != nil && cur.running() {
		s.watchMu.Unlock()
		return model.Operation{}, fmt.Errorf("a watch is already running for %q", name)
	}
	if s.watches == nil {
		s.watches = map[string]*watchRun{}
	}
	s.watches[key] = run
	s.watchMu.Unlock()

	runner := s.run()
	op := s.Ops.Start(ops.Spec{
		Kind: "compose.watch", Title: "Compose watch " + name + svcSuffix(opts.Services),
		ConnectionID: connectionID, Resource: name, Cancellable: true,
	}, func(ctx context.Context, h *ops.Handle) error {
		defer s.emitChanged(connectionID)
		h.Log("$ " + cmd.Name + " " + strings.Join(cmd.Args, " "))
		err := runner.Run(ctx, cmd, func(l string) {
			if t := run.feed(l, time.Now()); t != "" {
				h.Log(t)
			}
		})
		if ctx.Err() != nil {
			s.dropWatch(key, run)
			run.finish(time.Now(), "")
			return ctx.Err()
		}
		reason := run.exitReason(err)
		run.finish(time.Now(), reason)
		return fmt.Errorf("Compose watch ended: %s", firstLine(reason))
	})
	run.setOperation(op.ID)
	s.emitWatch(connectionID, name)
	return op, nil
}

// StopWatch stops the watch of a project (the CLI process tree is killed). Containers keep running.
func (s *ComposeService) StopWatch(ctx context.Context, connectionID, name string) error {
	r := s.watchRunFor(connectionID, name)
	if r == nil || !r.running() {
		return fmt.Errorf("no watch is running for %q", name)
	}
	r.mu.Lock()
	id := r.st.OperationID
	r.mu.Unlock()
	if err := s.Ops.Cancel(id); err != nil {
		return err
	}
	select {
	case <-r.done:
	case <-time.After(watchStopWait):
		return fmt.Errorf("the watch process did not stop within %s", watchStopWait)
	case <-ctx.Done():
	}
	s.emitWatch(connectionID, name)
	return nil
}

// DismissWatch forgets a watch that ended on its own (its error is no longer shown).
func (s *ComposeService) DismissWatch(ctx context.Context, connectionID, name string) error {
	r := s.watchRunFor(connectionID, name)
	if r == nil || r.running() {
		return nil
	}
	s.dropWatch(watchKey(connectionID, name), r)
	s.emitWatch(connectionID, name)
	return nil
}

func (s *ComposeService) dropWatch(key string, r *watchRun) {
	s.watchMu.Lock()
	if s.watches[key] == r {
		delete(s.watches, key)
	}
	s.watchMu.Unlock()
}

func (s *ComposeService) emitWatch(connectionID, name string) {
	if s.Events != nil {
		s.Events.Emit(model.EventComposeWatch, model.ComposeWatchChanged{ConnectionID: connectionID, Project: name})
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
