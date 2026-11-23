package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
)

// newComposeFileName is the name every file made by CreateProject gets.
const newComposeFileName = "compose.yaml"

func (s *ComposeService) creatorFor(env compose.ExecEnv) compose.FileCreator {
	if s.creator != nil {
		return s.creator
	}
	if env.Kind == compose.KindWSL {
		return compose.WSLFS{Runner: s.run(), Distro: env.Distro}
	}
	return compose.LocalFS{}
}

// CreateProject writes text as compose.yaml into dir (created when missing), validates it with
// `docker compose config -q` and saves the project. It never replaces a file: when dir already
// has a Compose file, that file is reported and nothing is written. A failure after the file
// was written removes it again, and the folder too when this call created it.
func (s *ComposeService) CreateProject(ctx context.Context, connectionID, dir, name, text string) (model.ComposeCreateResult, error) {
	none := model.ComposeCreateResult{}
	if s.Projects == nil {
		return none, errNoProjectStore
	}
	env, err := s.env(connectionID)
	if err != nil {
		return none, err
	}
	if env.Kind == compose.KindNone {
		return none, errors.New(env.Reason)
	}
	if st := s.cliStatus(ctx, env); !st.OK {
		return none, errors.New(st.Detail)
	}

	if strings.TrimSpace(dir) == "" {
		return none, errors.New("choose a folder")
	}
	dirs, err := normalizeFiles(env, []string{dir}, "folder")
	if err != nil {
		return none, err
	}
	dir = dirs[0]
	fs := s.fsFor(env)
	if fs.FileExists(ctx, dir) {
		return none, fmt.Errorf("%s is a file, not a folder", dir)
	}
	// A pasted file path would otherwise become a folder called compose.yaml.
	if base := strings.ToLower(baseName(env, dir)); strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") {
		return none, fmt.Errorf("%s looks like a file name; choose the folder the file should be in", dir)
	}

	name = strings.TrimSpace(name)
	if err := compose.ValidateProjectName(name); err != nil {
		return none, fmt.Errorf("%w (lower-case letters, digits, - and _; must start with a letter or digit)", err)
	}
	saved, err := s.managed(ctx, connectionID)
	if err != nil {
		return none, err
	}
	for _, r := range saved {
		if r.Name == name {
			return none, fmt.Errorf("a project named %q is already saved for this connection", name)
		}
	}

	if strings.TrimSpace(text) == "" {
		return none, errors.New("the Compose file is empty")
	}
	if len(text) > compose.MaxEditableFile {
		return none, fmt.Errorf("the Compose file is larger than %d KB", compose.MaxEditableFile/1024)
	}

	for _, n := range compose.ComposeFileNames {
		if p := compose.JoinPath(env.Kind, dir, n); fs.FileExists(ctx, p) {
			return model.ComposeCreateResult{ExistingFile: p}, nil
		}
	}

	// The folders that do not exist yet, innermost first: they are what a failure takes away again.
	var missing []string
	for p := dir; !fs.DirExists(ctx, p); {
		missing = append(missing, p)
		parent := compose.DirOf(env.Kind, p)
		if parent == p {
			break
		}
		p = parent
	}
	creator := s.creatorFor(env)
	file := compose.JoinPath(env.Kind, dir, newComposeFileName)
	// undo removes what this call put on disk; it must work even when ctx is already cancelled.
	undo := func(fileWritten bool) {
		bg := context.WithoutCancel(ctx)
		if fileWritten {
			_ = creator.Remove(bg, file)
		}
		for _, d := range missing {
			_ = creator.Remove(bg, d) // only empty folders go
		}
	}
	if _, err := creator.MakeDir(ctx, dir); err != nil {
		undo(false) // parents may exist by now even though the last folder failed
		return none, err
	}
	if err := creator.CreateFile(ctx, file, []byte(text)); err != nil {
		undo(false)
		if errors.Is(err, compose.ErrFileExists) {
			return model.ComposeCreateResult{ExistingFile: file}, nil
		}
		return none, err
	}
	if err := s.validateConfig(ctx, env, name, []string{file}, nil, dir); err != nil {
		undo(true)
		return none, err
	}
	if _, err := s.Projects.SaveComposeProject(ctx, appdb.ComposeProjectRecord{
		ConnectionID: connectionID, Name: name, WorkingDir: dir, ConfigFiles: []string{file}, EnvFiles: []string{},
	}); err != nil {
		undo(true)
		return none, fmt.Errorf("could not save the project: %w", err)
	}
	p, _, err := s.project(ctx, connectionID, name)
	if err != nil {
		return none, err
	}
	return model.ComposeCreateResult{Project: &p}, nil
}
