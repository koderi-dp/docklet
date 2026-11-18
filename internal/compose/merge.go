package compose

import (
	"sort"

	"docklet/internal/model"
)

// ManagedRecord is a user-saved project definition (files chosen by the user).
type ManagedRecord struct {
	Name        string
	WorkingDir  string
	ConfigFiles []string
	EnvFiles    []string
}

// ManagedSource is the Source label of a managed project for an execution environment.
func ManagedSource(env ExecEnv) string {
	if env.Kind == KindWSL {
		return model.ComposeSourceManagedWSL
	}
	return model.ComposeSourceManagedLocal
}

// ApplyManaged overlays a managed record on a project (discovered from labels, or an
// empty one when nothing is running). The record supplies the Compose files; a project
// without containers is "not created".
func ApplyManaged(p model.ComposeProject, rec ManagedRecord, env ExecEnv) model.ComposeProject {
	p.Managed = true
	p.Source = ManagedSource(env)
	p.ConfigFiles = append([]string{}, rec.ConfigFiles...)
	p.EnvFiles = append([]string{}, rec.EnvFiles...)
	p.WorkingDir = rec.WorkingDir
	p.NotCreated = p.Total == 0
	return p
}

// Merge combines projects discovered from labels with managed records of one
// connection. A record matches a discovered project by exact name.
func Merge(connectionID string, discovered []model.ComposeProject, managed []ManagedRecord, env ExecEnv) []model.ComposeProject {
	byName := map[string]ManagedRecord{}
	for _, r := range managed {
		byName[r.Name] = r
	}
	out := make([]model.ComposeProject, 0, len(discovered)+len(managed))
	seen := map[string]bool{}
	for _, p := range discovered {
		seen[p.Name] = true
		if r, ok := byName[p.Name]; ok {
			p = ApplyManaged(p, r, env)
		}
		out = append(out, p)
	}
	for _, r := range managed {
		if seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		out = append(out, ApplyManaged(EmptyProject(connectionID, r.Name), r, env))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// EmptyProject is a project with no containers.
func EmptyProject(connectionID, name string) model.ComposeProject {
	return model.ComposeProject{
		ConnectionID: connectionID, Name: name, Source: model.ComposeSourceLabels,
		Services: []model.ComposeServiceInfo{}, ContainerIDs: []string{},
		ConfigFiles: []string{}, EnvFiles: []string{},
	}
}

// AddConfigServices adds services known from `docker compose config --services` that have
// no container yet (Total 0), keeping existing services untouched.
func AddConfigServices(p model.ComposeProject, names []string) model.ComposeProject {
	have := map[string]bool{}
	for _, s := range p.Services {
		have[s.Name] = true
	}
	for _, n := range names {
		if n != "" && !have[n] {
			have[n] = true
			p.Services = append(p.Services, model.ComposeServiceInfo{Name: n, Containers: []model.ComposeContainerRef{}})
		}
	}
	sort.Slice(p.Services, func(i, j int) bool { return p.Services[i].Name < p.Services[j].Name })
	return p
}
