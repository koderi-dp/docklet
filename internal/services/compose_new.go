package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"docklet/internal/compose"
	"docklet/internal/compose/starter"
	"docklet/internal/model"
	"docklet/internal/runspec"
)

// NewFilePreview returns the text a new Compose file would start with. It writes nothing and
// reads the Engine only when the file is made from containers.
func (s *ComposeService) NewFilePreview(ctx context.Context, connectionID, name string, src model.ComposeNewSource) (model.ComposeNewPreview, error) {
	name = strings.TrimSpace(name)
	if err := compose.ValidateProjectName(name); err != nil {
		return model.ComposeNewPreview{}, fmt.Errorf("%w (lower-case letters, digits, - and _; must start with a letter or digit)", err)
	}
	out := model.ComposeNewPreview{SecretKeys: []string{}, NotIncluded: []string{}}
	switch src.Kind {
	case "minimal":
		out.Text = starter.Minimal(name)
	case "template":
		return model.ComposeNewPreview{}, fmt.Errorf("unknown template %q", src.Template)
	case "containers":
		f, err := s.fileFromContainers(ctx, connectionID, name, src)
		if err != nil {
			return model.ComposeNewPreview{}, err
		}
		out.Text, out.SecretKeys, out.NotIncluded = f.Text, f.SecretKeys, f.NotIncluded
	default:
		return model.ComposeNewPreview{}, fmt.Errorf("unknown starting point %q", src.Kind)
	}
	return out, nil
}

// fileFromContainers inspects the containers in the given order and renders them as one file.
func (s *ComposeService) fileFromContainers(ctx context.Context, connectionID, name string, src model.ComposeNewSource) (runspec.ComposeFile, error) {
	if len(src.ContainerIDs) == 0 {
		return runspec.ComposeFile{}, errors.New("choose at least one container")
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return runspec.ComposeFile{}, err
	}
	ctx, cancel := callCtx(ctx)
	defer cancel()
	specs := make([]runspec.Spec, 0, len(src.ContainerIDs))
	nameOf := map[string]string{} // full container id -> container name
	for _, id := range src.ContainerIDs {
		l, err := loadSpecInputs(ctx, cli, id)
		if err != nil {
			return runspec.ComposeFile{}, fmt.Errorf("container %s: %w", id, err)
		}
		sp := runspec.FromInspect(l.in, l.img, l.hints)
		specs = append(specs, sp)
		nameOf[l.in.ID] = sp.Name
	}
	// A network mode that names another chosen container by id is given its name, which the
	// renderer turns into that container's service.
	for i := range specs {
		if id, ok := strings.CutPrefix(specs[i].NetworkMode, "container:"); ok {
			if n, found := nameOf[id]; found && n != "" {
				specs[i].NetworkMode = "container:" + n
			}
		}
	}
	return runspec.ToComposeFile(name, specs, runspec.Options{IncludeSecrets: src.IncludeSecrets}), nil
}
