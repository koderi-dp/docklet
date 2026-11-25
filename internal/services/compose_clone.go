package services

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
	"docklet/internal/ops"
)

// A clone is a copy of the project folder saved under another project name. The original is
// only read. Named volumes are not copied: the clone gets its own, empty ones on its first Up.

func (s *ComposeService) copierFor(env compose.ExecEnv) compose.TreeCopier {
	if env.Kind == compose.KindWSL {
		return compose.WSLFS{Runner: s.run(), Distro: env.Distro}
	}
	return compose.LocalFS{}
}

// pathKey makes paths comparable: one kind of slash, no trailing slash, and no letter case on
// a Windows file system.
func pathKey(kind compose.Kind, p string) string {
	p = strings.TrimRight(strings.ReplaceAll(p, "\\", "/"), "/")
	if kind == compose.KindLocal && runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

// relInside returns the part of p below dir ("" and false when p is not inside dir).
func relInside(kind compose.Kind, dir, p string) (string, bool) {
	d, k := pathKey(kind, dir), pathKey(kind, p)
	if !strings.HasPrefix(k, d+"/") {
		return "", false
	}
	clean := strings.TrimRight(strings.ReplaceAll(p, "\\", "/"), "/")
	return clean[len(d)+1:], true
}

func inside(kind compose.Kind, dir, p string) bool {
	_, ok := relInside(kind, dir, p)
	return ok
}

// remap moves the files that are inside src to the same place inside dst; the others stay.
func remap(kind compose.Kind, src, dst string, files []string) (moved, outside []string) {
	moved = make([]string, 0, len(files))
	for _, f := range files {
		rel, ok := relInside(kind, src, f)
		if !ok {
			moved, outside = append(moved, f), append(outside, f)
			continue
		}
		target := dst
		for _, part := range strings.Split(rel, "/") {
			target = compose.JoinPath(kind, target, part)
		}
		moved = append(moved, target)
	}
	return moved, outside
}

// cloneSource resolves the project to clone and why it cannot be cloned ("" when it can).
func (s *ComposeService) cloneSource(ctx context.Context, connectionID, name string) (model.ComposeProject, compose.ExecEnv, string, error) {
	p, env, err := s.project(ctx, connectionID, name)
	if err != nil {
		return p, env, "", err
	}
	switch {
	case !p.Managed:
		return p, env, "Only a saved project can be cloned: this one is only known from its containers.", nil
	case !p.Capability.CanRun:
		return p, env, p.Capability.Reason, nil
	case p.WorkingDir == "":
		return p, env, "The project has no folder to copy.", nil
	}
	if _, outside := remap(env.Kind, p.WorkingDir, p.WorkingDir, p.ConfigFiles); len(outside) == len(p.ConfigFiles) {
		return p, env, "None of the project's Compose files is inside its folder " + p.WorkingDir + ", so a copy of the folder would not be a copy of the project.", nil
	}
	return p, env, "", nil
}

// ClonePreview describes what CloneProject would copy and what the copy would still share
// with the original. It changes nothing.
func (s *ComposeService) ClonePreview(ctx context.Context, connectionID, name string) (model.ComposeClonePreview, error) {
	pv := model.ComposeClonePreview{Outside: []string{}, Collisions: []string{}}
	p, env, blocked, err := s.cloneSource(ctx, connectionID, name)
	if err != nil {
		return pv, err
	}
	pv.SourceDir, pv.Blocked, pv.Running = p.WorkingDir, blocked, p.Running > 0
	if blocked != "" {
		return pv, nil
	}
	fs := s.fsFor(env)
	parent, base := compose.DirOf(env.Kind, p.WorkingDir), baseName(env, p.WorkingDir)
	for i := 1; i < 100 && (pv.SuggestedName == "" || pv.SuggestedDir == ""); i++ {
		suffix := "-copy"
		if i > 1 {
			suffix = fmt.Sprintf("-copy-%d", i)
		}
		if pv.SuggestedName == "" && compose.ValidateProjectName(name+suffix) == nil && s.nameFree(ctx, connectionID, name+suffix) == nil {
			pv.SuggestedName = name + suffix
		}
		if d := compose.JoinPath(env.Kind, parent, base+suffix); pv.SuggestedDir == "" && !fs.DirExists(ctx, d) && !fs.FileExists(ctx, d) {
			pv.SuggestedDir = d
		}
	}
	_, o1 := remap(env.Kind, p.WorkingDir, p.WorkingDir, p.ConfigFiles)
	_, o2 := remap(env.Kind, p.WorkingDir, p.WorkingDir, p.EnvFiles)
	pv.Outside = append(append(pv.Outside, o1...), o2...)

	sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if n, files, err := s.copierFor(env).TreeSize(sctx, p.WorkingDir); err == nil {
		pv.SizeBytes, pv.Files, pv.SizeKnown = n, files, true
	}
	req := requestFor(p, compose.VerbConfig)
	req.ConfigMode = "json"
	if out, err := s.configOutput(ctx, env, req); err == nil {
		if c, err := compose.ParseCloneCollisions(out, p.WorkingDir); err == nil {
			pv.Collisions = c
		}
	}
	return pv, nil
}

// CloneProject copies the project's folder to destDir and saves the copy as newName. destDir must
// not exist yet or be empty. The copy runs as a cancellable operation; when it fails, is
// cancelled or Compose rejects the copied files, what was copied is removed again.
func (s *ComposeService) CloneProject(ctx context.Context, connectionID, name, newName, destDir string) (model.Operation, error) {
	none := model.Operation{}
	if s.Projects == nil {
		return none, errNoProjectStore
	}
	if s.Ops == nil {
		return none, errors.New("operation manager unavailable")
	}
	newName = strings.TrimSpace(newName)
	if err := compose.ValidateProjectName(newName); err != nil {
		return none, fmt.Errorf("%w %s", err, nameRule)
	}
	p, env, blocked, err := s.cloneSource(ctx, connectionID, name)
	if err != nil {
		return none, err
	}
	if blocked != "" {
		return none, errors.New(blocked)
	}
	if newName == name {
		return none, fmt.Errorf("a project named %q is already saved for this connection", name)
	}
	if err := s.nameFree(ctx, connectionID, newName); err != nil {
		return none, err
	}
	if strings.TrimSpace(destDir) == "" {
		return none, errors.New("choose a folder for the copy")
	}
	dirs, err := s.normalizeFiles(ctx, env, []string{destDir}, "folder")
	if err != nil {
		return none, err
	}
	dst, src := dirs[0], p.WorkingDir
	fs, copier := s.fsFor(env), s.copierFor(env)
	switch {
	case pathKey(env.Kind, dst) == pathKey(env.Kind, src):
		return none, errors.New("that is the same folder as the original")
	case inside(env.Kind, src, dst):
		return none, fmt.Errorf("%s is inside the original folder; choose a folder next to it", dst)
	case inside(env.Kind, dst, src):
		return none, fmt.Errorf("the original folder is inside %s; choose another folder", dst)
	case fs.FileExists(ctx, dst):
		return none, fmt.Errorf("%s is a file, not a folder", dst)
	}
	existed := fs.DirExists(ctx, dst)
	if existed && !copier.DirEmpty(ctx, dst) {
		return none, fmt.Errorf("%s is not empty; choose a new or an empty folder", dst)
	}
	cfg, _ := remap(env.Kind, src, dst, p.ConfigFiles)
	envf, _ := remap(env.Kind, src, dst, p.EnvFiles)

	op := s.Ops.Start(ops.Spec{
		Kind: "compose.clone", Title: "Clone " + name + " to " + newName,
		ConnectionID: connectionID, Resource: newName, Cancellable: true,
	}, func(ctx context.Context, h *ops.Handle) error {
		// fail removes what was copied; it must work even when ctx is already cancelled.
		fail := func(err error) error {
			bg := context.WithoutCancel(ctx)
			if rerr := copier.RemoveTree(bg, dst); rerr != nil {
				h.Log("Could not remove the incomplete copy " + dst + ": " + rerr.Error())
			} else {
				if existed {
					_, _ = s.creatorFor(env).MakeDir(bg, dst)
				}
				h.Log("Removed the incomplete copy " + dst)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		h.Log(fmt.Sprintf("Copying %s to %s (%s)", src, dst, env.Location))
		if err := copier.CopyTree(ctx, src, dst); err != nil {
			return fail(fmt.Errorf("could not copy the folder: %w", err))
		}
		h.Log("Checking the copy with docker compose config")
		if err := s.validateConfig(ctx, env, newName, cfg, envf, dst); err != nil {
			return fail(err)
		}
		if _, err := s.Projects.SaveComposeProject(ctx, appdb.ComposeProjectRecord{
			ConnectionID: connectionID, Name: newName, WorkingDir: dst, ConfigFiles: cfg, EnvFiles: envf,
		}); err != nil {
			return fail(fmt.Errorf("could not save the project: %w", err))
		}
		h.Log("Saved as project " + newName + ". Its volumes are new and empty until it is started.")
		s.emitChanged(connectionID)
		return nil
	})
	return op, nil
}
