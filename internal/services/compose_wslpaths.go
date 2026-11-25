package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
)

// A Compose file written for Docker Desktop mounts folders by their Windows path. On a WSL
// connection the engine is Linux and refuses that. Docklet offers to turn the path into a
// variable that keeps the Windows path as its default (the file keeps working everywhere it
// worked) and to give the Linux path in an env file used for this connection only.

// wslEnvFileName is the env file that holds the Linux paths of a WSL connection.
const wslEnvFileName = ".env.wsl"

// wslPathDeps is what the plan and the fix need from the place the files are in.
type wslPathDeps struct {
	io      compose.FileIO
	fs      compose.FS
	creator compose.FileCreator
	kind    compose.Kind
	// translate returns the Linux path of a Windows path, or an error when the distribution
	// cannot reach it.
	translate func(ctx context.Context, windowsPath string) (string, error)
}

var usedVarRe = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)`)

func projectDir(kind compose.Kind, p model.ComposeProject) string {
	if p.WorkingDir != "" || len(p.ConfigFiles) == 0 {
		return p.WorkingDir
	}
	return compose.DirOf(kind, p.ConfigFiles[0])
}

// planWSLPaths lists the Windows-path bind mounts found in the resolved config and, for each,
// the line of the project's Compose files that would change.
func planWSLPaths(ctx context.Context, p model.ComposeProject, configJSON string, d wslPathDeps) (model.ComposeWSLPathReport, error) {
	rep := model.ComposeWSLPathReport{Issues: []model.ComposeWSLPathIssue{}}
	binds, err := compose.FindWindowsBinds(configJSON)
	if err != nil || len(binds) == 0 {
		return rep, err
	}
	rep.EnvFile = compose.JoinPath(d.kind, projectDir(d.kind, p), wslEnvFileName)
	texts := map[string]string{}
	taken := map[string]bool{}
	for _, f := range p.ConfigFiles {
		data, err := d.io.ReadFile(ctx, f)
		if err != nil {
			return rep, err
		}
		text, _, err := compose.SplitText(data)
		if err != nil {
			return rep, err
		}
		texts[f] = text
		for _, m := range usedVarRe.FindAllStringSubmatch(text, -1) {
			taken[m[1]] = true
		}
	}
	planned := map[string]compose.LineEdit{} // file + line -> edit, so one line gets one variable
	for _, b := range binds {
		is := model.ComposeWSLPathIssue{Service: b.Service, Source: b.Source, Target: b.Target}
		linux, terr := d.translate(ctx, b.Source)
		if terr != nil {
			is.Problem = terr.Error()
			rep.Issues = append(rep.Issues, is)
			continue
		}
		is.LinuxPath = linux
		found := false
		// A later file overrides an earlier one, so it is the one to change.
		for i := len(p.ConfigFiles) - 1; i >= 0 && !found; i-- {
			f := p.ConfigFiles[i]
			e, ok := compose.PlanBindVariable(texts[f], b.Source, b.Target, taken)
			if !ok {
				continue
			}
			key := fmt.Sprintf("%s\x00%d", f, e.Line)
			if prev, dup := planned[key]; dup {
				e = prev
			}
			planned[key] = e
			taken[e.Variable] = true
			is.File, is.Line, is.OldLine, is.NewLine, is.Variable = f, e.Line, e.Old, e.New, e.Variable
			found, rep.Fixable = true, true
		}
		if !found {
			is.Problem = "The path is not written in the Compose file: it comes from a variable or an env file. Set that variable to " + linux + " for this connection."
		}
		rep.Issues = append(rep.Issues, is)
	}
	return rep, nil
}

// applyWSLPaths carries out the fixes of a report: it rewrites the planned lines, sets the
// variables in the env file and returns the env files the project should use from now on.
// undo puts every file back as it was.
func applyWSLPaths(ctx context.Context, p model.ComposeProject, rep model.ComposeWSLPathReport, d wslPathDeps) (envFiles []string, undo func(), err error) {
	var steps []func()
	undo = func() {
		for i := len(steps) - 1; i >= 0; i-- {
			steps[i]()
		}
	}
	bg := context.WithoutCancel(ctx)
	fail := func(err error) ([]string, func(), error) {
		undo()
		return nil, func() {}, err
	}

	vars := map[string]string{}
	var names []string
	edits := map[string][]compose.LineEdit{}
	for _, is := range rep.Issues {
		if is.Variable == "" || is.LinuxPath == "" {
			continue
		}
		if _, seen := vars[is.Variable]; !seen {
			names = append(names, is.Variable)
		}
		vars[is.Variable] = is.LinuxPath
		if is.NewLine != is.OldLine {
			edits[is.File] = append(edits[is.File], compose.LineEdit{Line: is.Line, Old: is.OldLine, New: is.NewLine})
		}
	}
	if len(names) == 0 {
		return nil, func() {}, errors.New("there is nothing to fix")
	}

	for _, f := range p.ConfigFiles {
		if len(edits[f]) == 0 {
			continue
		}
		orig, err := d.io.ReadFile(ctx, f)
		if err != nil {
			return fail(err)
		}
		text, style, err := compose.SplitText(orig)
		if err != nil {
			return fail(err)
		}
		next := text
		for _, e := range edits[f] {
			next = compose.ApplyLineEdit(next, e)
		}
		if next == text {
			return fail(fmt.Errorf("%s changed since it was checked; nothing was written", f))
		}
		if err := d.io.WriteFile(ctx, f, compose.JoinText(next, style)); err != nil {
			return fail(err)
		}
		steps = append(steps, func() { _ = d.io.WriteFile(bg, f, orig) })
	}

	if d.fs.FileExists(ctx, rep.EnvFile) {
		orig, err := d.io.ReadFile(ctx, rep.EnvFile)
		if err != nil {
			return fail(err)
		}
		text, style, err := compose.SplitText(orig)
		if err != nil {
			return fail(err)
		}
		for _, n := range names {
			text = compose.SetEnvVar(text, n, vars[n])
		}
		if err := d.io.WriteFile(ctx, rep.EnvFile, compose.JoinText(text, style)); err != nil {
			return fail(err)
		}
		steps = append(steps, func() { _ = d.io.WriteFile(bg, rep.EnvFile, orig) })
	} else {
		text := ""
		for _, n := range names {
			text = compose.SetEnvVar(text, n, vars[n])
		}
		if err := d.creator.CreateFile(ctx, rep.EnvFile, []byte(text)); err != nil {
			return fail(err)
		}
		steps = append(steps, func() { _ = d.creator.Remove(bg, rep.EnvFile) })
	}

	envFiles = append([]string{}, p.EnvFiles...)
	if len(envFiles) == 0 {
		// Once --env-file is given Compose no longer reads .env on its own.
		if def := compose.JoinPath(d.kind, projectDir(d.kind, p), ".env"); d.fs.FileExists(ctx, def) {
			envFiles = append(envFiles, def)
		}
	}
	if !slices.Contains(envFiles, rep.EnvFile) {
		envFiles = append(envFiles, rep.EnvFile)
	}
	return envFiles, undo, nil
}

// wslPathPlan resolves the project and plans the fix. The report is empty for every connection
// that is not WSL and for a project the CLI cannot run.
func (s *ComposeService) wslPathPlan(ctx context.Context, connectionID, name string, profiles []string) (model.ComposeProject, compose.ExecEnv, wslPathDeps, model.ComposeWSLPathReport, error) {
	empty := model.ComposeWSLPathReport{Issues: []model.ComposeWSLPathIssue{}}
	p, env, io, err := s.filesFor(ctx, connectionID, name)
	if err != nil {
		return p, env, wslPathDeps{}, empty, err
	}
	if env.Kind != compose.KindWSL || !p.Capability.CanRun {
		return p, env, wslPathDeps{}, empty, nil
	}
	d := wslPathDeps{
		io: io, fs: s.fsFor(env), creator: s.creatorFor(env), kind: env.Kind,
		translate: func(ctx context.Context, wp string) (string, error) {
			linux, err := s.wslPath(ctx, env.Distro, wp)
			if err != nil {
				return "", fmt.Errorf("The WSL distribution %q cannot reach %s: its drive is not mounted there.", env.Distro, wp)
			}
			return linux, nil
		},
	}
	req := requestFor(p, compose.VerbConfig)
	req.ConfigMode, req.Profiles = "json", profiles
	out, err := s.configOutput(ctx, env, req)
	if err != nil {
		return p, env, d, empty, nil // Up reports what is wrong with the files
	}
	rep, err := planWSLPaths(ctx, p, out, d)
	return p, env, d, rep, err
}

// WSLPathIssues reports the bind mounts of the project that use a Windows path although the
// project runs inside a WSL distribution, each with the fix Docklet can make. It changes nothing.
func (s *ComposeService) WSLPathIssues(ctx context.Context, connectionID, name string, profiles []string) (model.ComposeWSLPathReport, error) {
	if err := compose.ValidateProjectName(name); err != nil {
		return model.ComposeWSLPathReport{}, err
	}
	_, _, _, rep, err := s.wslPathPlan(ctx, connectionID, name, profiles)
	return rep, err
}

// FixWSLPaths makes the fixes WSLPathIssues reports: each Windows path in the Compose file
// becomes ${NAME:-<the Windows path>}, the Linux paths go to .env.wsl next to it, and that file
// is added to the env files of the project for this connection. The files are put back as they
// were when Compose rejects the result.
func (s *ComposeService) FixWSLPaths(ctx context.Context, connectionID, name string, profiles []string) (model.ComposeProject, error) {
	none := model.ComposeProject{}
	if s.Projects == nil {
		return none, errNoProjectStore
	}
	if err := compose.ValidateProjectName(name); err != nil {
		return none, err
	}
	p, env, d, rep, err := s.wslPathPlan(ctx, connectionID, name, profiles)
	if err != nil {
		return none, err
	}
	if !rep.Fixable {
		return none, errors.New("there is nothing Docklet can fix in this project's mounts")
	}
	envFiles, undo, err := applyWSLPaths(ctx, p, rep, d)
	if err != nil {
		return none, err
	}
	dir := projectDir(env.Kind, p)
	if err := s.validateConfig(ctx, env, p.Name, p.ConfigFiles, envFiles, dir); err != nil {
		undo()
		return none, fmt.Errorf("the files were put back as they were: %w", err)
	}
	if _, err := s.Projects.SaveComposeProject(ctx, appdb.ComposeProjectRecord{
		ConnectionID: connectionID, Name: p.Name, WorkingDir: dir, ConfigFiles: p.ConfigFiles, EnvFiles: envFiles,
	}); err != nil {
		undo()
		return none, fmt.Errorf("could not save the project, so the files were put back as they were: %w", err)
	}
	s.emitChanged(connectionID)
	fixed, _, err := s.project(ctx, connectionID, name)
	return fixed, err
}
