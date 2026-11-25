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

const nameRule = "(lower-case letters, digits, - and _; must start with a letter or digit)"

// nameFree fails when a project called name is saved for the connection or has containers on
// its Engine: a second project of that name would take those over.
func (s *ComposeService) nameFree(ctx context.Context, connectionID, name string) error {
	saved, err := s.managed(ctx, connectionID)
	if err != nil {
		return err
	}
	for _, r := range saved {
		if r.Name == name {
			return fmt.Errorf("a project named %q is already saved for this connection", name)
		}
	}
	cs, err := s.labelled(ctx, connectionID, name)
	if err != nil {
		return err
	}
	if len(cs) > 0 {
		return fmt.Errorf("a project named %q already has containers on this Engine", name)
	}
	return nil
}

// RenameProject gives a saved project another name. Only the saved entry changes: no file is
// touched and Compose is not run. It is refused while the project has containers, because
// Compose finds a project's containers, networks and volumes by its name and would lose them.
func (s *ComposeService) RenameProject(ctx context.Context, connectionID, name, newName string) (model.ComposeProject, error) {
	none := model.ComposeProject{}
	if s.Projects == nil {
		return none, errNoProjectStore
	}
	newName = strings.TrimSpace(newName)
	if err := compose.ValidateProjectName(newName); err != nil {
		return none, fmt.Errorf("%w %s", err, nameRule)
	}
	if newName == name {
		return none, errors.New("that is the same name")
	}
	p, _, err := s.project(ctx, connectionID, name)
	if err != nil {
		return none, err
	}
	if !p.Managed {
		return none, fmt.Errorf("%q is not a saved project: it is only known from its containers", name)
	}
	if p.Total > 0 {
		return none, fmt.Errorf("%q has containers. Compose finds them by the project name, so a renamed project would lose them; run Down first, or clone the project", name)
	}
	if err := s.nameFree(ctx, connectionID, newName); err != nil {
		return none, err
	}
	if _, err := s.Projects.SaveComposeProject(ctx, appdb.ComposeProjectRecord{
		ConnectionID: connectionID, Name: newName, WorkingDir: p.WorkingDir, ConfigFiles: p.ConfigFiles, EnvFiles: p.EnvFiles,
	}); err != nil {
		return none, fmt.Errorf("could not save the project: %w", err)
	}
	if err := s.Projects.DeleteComposeProject(ctx, connectionID, name); err != nil {
		// Both entries point at the same files; nothing is lost, but say so.
		return none, fmt.Errorf("the project was saved as %q, but the entry %q could not be removed: %w", newName, name, err)
	}
	s.emitChanged(connectionID)
	renamed, _, err := s.project(ctx, connectionID, newName)
	return renamed, err
}
