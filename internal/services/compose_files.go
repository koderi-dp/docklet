package services

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"docklet/internal/compose"
	"docklet/internal/model"
)

const noConfigFilesReason = "The project's containers carry no config-file label, so the Compose file location is unknown."

// filesFor resolves the project and the file access of its environment.
func (s *ComposeService) filesFor(ctx context.Context, connectionID, name string) (model.ComposeProject, compose.ExecEnv, compose.FileIO, error) {
	p, env, err := s.project(ctx, connectionID, name)
	if err != nil {
		return p, env, nil, err
	}
	if env.Kind == compose.KindNone {
		return p, env, nil, fmt.Errorf("%s", env.Reason)
	}
	io, ok := s.fsFor(env).(compose.FileIO)
	if !ok {
		return p, env, nil, fmt.Errorf("files cannot be read in this environment")
	}
	return p, env, io, nil
}

// ownFile checks that path is exactly one of the project's own files and reports whether it is an env file.
func ownFile(p model.ComposeProject, path string) (isEnv bool, err error) {
	switch {
	case slices.Contains(p.ConfigFiles, path):
		return false, nil
	case slices.Contains(p.EnvFiles, path):
		return true, nil
	}
	return false, fmt.Errorf("%q is not a file of project %q", path, p.Name)
}

// ProjectFiles lists the config files then the env files of the project.
func (s *ComposeService) ProjectFiles(ctx context.Context, connectionID, name string) (model.ComposeFileList, error) {
	p, env, err := s.project(ctx, connectionID, name)
	if err != nil {
		return model.ComposeFileList{}, err
	}
	out := model.ComposeFileList{Files: []model.ComposeFileRef{}}
	for _, f := range p.ConfigFiles {
		out.Files = append(out.Files, model.ComposeFileRef{Path: f, Kind: "config"})
	}
	for _, f := range p.EnvFiles {
		out.Files = append(out.Files, model.ComposeFileRef{Path: f, Kind: "env"})
	}
	switch {
	case env.Kind != compose.KindLocal && env.Kind != compose.KindWSL:
		out.Reason = env.Reason
	case len(p.ConfigFiles) == 0:
		out.Reason = noConfigFilesReason
	default:
		fs := s.fsFor(env)
		var missing []string
		for _, f := range p.ConfigFiles {
			if !fs.FileExists(ctx, f) {
				missing = append(missing, f)
			}
		}
		switch {
		case len(missing) == 0:
			out.CanEdit = true
		case env.Kind == compose.KindWSL:
			out.Reason = fmt.Sprintf("Compose file(s) not found inside %s: %s", env.Location, strings.Join(missing, ", "))
		default:
			out.Reason = fmt.Sprintf("Compose file(s) not found on this machine: %s", strings.Join(missing, ", "))
		}
	}
	return out, nil
}

// ReadProjectFile returns one of the project's files as editor text.
func (s *ComposeService) ReadProjectFile(ctx context.Context, connectionID, name, path string) (model.ComposeFileContent, error) {
	p, _, io, err := s.filesFor(ctx, connectionID, name)
	if err != nil {
		return model.ComposeFileContent{}, err
	}
	isEnv, err := ownFile(p, path)
	if err != nil {
		return model.ComposeFileContent{}, err
	}
	data, err := io.ReadFile(ctx, path)
	if err != nil {
		return model.ComposeFileContent{}, err
	}
	text, _, err := compose.SplitText(data)
	if err != nil {
		return model.ComposeFileContent{}, err
	}
	return model.ComposeFileContent{Path: path, Text: text, Version: compose.FileVersion(data), ReadOnly: isEnv}, nil
}

// SaveProjectFile writes text to a config file of the project. It writes nothing when the YAML
// is invalid or, unless overwrite is set, when the file on disk is no longer baseVersion.
func (s *ComposeService) SaveProjectFile(ctx context.Context, connectionID, name, path, text, baseVersion string, overwrite bool) (model.ComposeSaveResult, error) {
	p, _, io, err := s.filesFor(ctx, connectionID, name)
	if err != nil {
		return model.ComposeSaveResult{}, err
	}
	isEnv, err := ownFile(p, path)
	if err != nil {
		return model.ComposeSaveResult{}, err
	}
	if isEnv {
		return model.ComposeSaveResult{}, fmt.Errorf("env files are read-only")
	}
	if o := compose.Outline(text); o.Error != nil {
		return model.ComposeSaveResult{Error: o.Error}, nil
	}
	current, err := io.ReadFile(ctx, path)
	if err != nil {
		return model.ComposeSaveResult{}, err
	}
	if !overwrite && compose.FileVersion(current) != baseVersion {
		return model.ComposeSaveResult{Conflict: true}, nil
	}
	_, style, err := compose.SplitText(current)
	if err != nil {
		return model.ComposeSaveResult{}, err
	}
	data := compose.JoinText(text, style)
	if err := io.WriteFile(ctx, path, data); err != nil {
		return model.ComposeSaveResult{}, err
	}
	return model.ComposeSaveResult{Saved: true, Version: compose.FileVersion(data)}, nil
}

// FileOutline lists the services of a Compose file text, or the YAML error that stops it.
func (s *ComposeService) FileOutline(text string) model.ComposeFileOutline {
	return compose.Outline(text)
}
