package compose

import (
	"sort"
	"strings"

	"docklet/internal/model"
)

// Label names written by Docker Compose on every container it creates.
const (
	LabelProject    = "com.docker.compose.project"
	LabelService    = "com.docker.compose.service"
	LabelNumber     = "com.docker.compose.container-number"
	LabelWorkingDir = "com.docker.compose.project.working_dir"
	LabelConfigFile = "com.docker.compose.project.config_files"
	LabelEnvFile    = "com.docker.compose.project.environment_file"
	LabelOneOff     = "com.docker.compose.oneoff"
	LabelDependsOn  = "com.docker.compose.depends_on"
)

// Container is the minimal input of discovery: an Engine container with its labels.
type Container struct {
	ID      string
	Name    string
	Image   string
	State   string
	Created int64
	Labels  map[string]string
	// ExitCode and RestartPolicy classify exited containers (see IsCompleted).
	ExitCode      *int
	RestartPolicy string
}

// SplitList splits a comma separated label value, dropping empties.
func SplitList(v string) []string {
	out := []string{}
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func isOneOff(l map[string]string) bool { return strings.EqualFold(l[LabelOneOff], "true") }

// Discover groups containers by the exact com.docker.compose.project label of
// one Engine. Names, images and paths are never used for grouping. Capability
// is left zero: it depends on the execution environment.
func Discover(connectionID string, cs []Container) []model.ComposeProject {
	byProject := map[string][]Container{}
	for _, c := range cs {
		if p := c.Labels[LabelProject]; p != "" {
			byProject[p] = append(byProject[p], c)
		}
	}
	out := make([]model.ComposeProject, 0, len(byProject))
	for name, list := range byProject {
		out = append(out, buildProject(connectionID, name, list))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func buildProject(connectionID, name string, list []Container) model.ComposeProject {
	p := model.ComposeProject{
		ConnectionID: connectionID, Name: name, Source: model.ComposeSourceLabels,
		Services: []model.ComposeServiceInfo{}, ContainerIDs: []string{},
		ConfigFiles: []string{}, EnvFiles: []string{},
	}
	// Newest regular container first: its labels describe the latest `up`.
	sorted := append([]Container(nil), list...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if isOneOff(a.Labels) != isOneOff(b.Labels) {
			return !isOneOff(a.Labels)
		}
		return a.Created > b.Created
	})
	for _, c := range sorted {
		if len(p.ConfigFiles) == 0 {
			p.ConfigFiles = SplitList(c.Labels[LabelConfigFile])
		}
		if len(p.EnvFiles) == 0 {
			p.EnvFiles = SplitList(c.Labels[LabelEnvFile])
		}
		if p.WorkingDir == "" {
			p.WorkingDir = strings.TrimSpace(c.Labels[LabelWorkingDir])
		}
	}
	svcs := map[string]*model.ComposeServiceInfo{}
	for _, c := range list {
		sn := c.Labels[LabelService]
		s := svcs[sn]
		if s == nil {
			s = &model.ComposeServiceInfo{Name: sn, Containers: []model.ComposeContainerRef{}}
			svcs[sn] = s
		}
		if s.Image == "" {
			s.Image = c.Image
		}
		s.Containers = append(s.Containers, model.ComposeContainerRef{ID: c.ID, Name: c.Name, State: c.State, OneOff: isOneOff(c.Labels)})
		s.Total++
		p.Total++
		p.ContainerIDs = append(p.ContainerIDs, c.ID)
		switch c.State {
		case "running":
			s.Running++
			p.Running++
		case "exited", "created", "dead":
			switch {
			case IsCompleted(c.State, c.ExitCode, c.RestartPolicy, isOneOff(c.Labels)):
				p.Completed++
			case IsFailed(c.State, c.ExitCode):
				p.Failed++
				p.Stopped++
			default:
				p.Stopped++
			}
		}
	}
	for _, s := range svcs {
		sort.Slice(s.Containers, func(i, j int) bool { return s.Containers[i].Name < s.Containers[j].Name })
		p.Services = append(p.Services, *s)
	}
	sort.Slice(p.Services, func(i, j int) bool { return p.Services[i].Name < p.Services[j].Name })
	sort.Strings(p.ContainerIDs)
	return p
}
