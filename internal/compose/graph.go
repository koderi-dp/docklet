package compose

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"docklet/internal/model"
)

// Graph sources.
const (
	GraphSourceConfig     = "config"
	GraphSourceContainers = "containers"
)

// rawGraph reads only the fields the graph needs from `docker compose config
// --format json`. Environment, command and everything else is never decoded.
type rawGraph struct {
	Networks map[string]struct {
		Name string `json:"name"`
	} `json:"networks"`
	Volumes  map[string]json.RawMessage `json:"volumes"`
	Services map[string]rawService      `json:"services"`
}

type rawService struct {
	Image     string            `json:"image"`
	Build     json.RawMessage   `json:"build"`
	Ports     []json.RawMessage `json:"ports"`
	Volumes   []json.RawMessage `json:"volumes"`
	DependsOn json.RawMessage   `json:"depends_on"`
	Profiles  []string          `json:"profiles"`
}

// ParseGraph builds the service graph from `docker compose config --format json`.
func ParseGraph(jsonText string) (model.ComposeGraph, error) {
	var raw rawGraph
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return model.ComposeGraph{}, fmt.Errorf("parse compose config: %w", err)
	}
	g := model.ComposeGraph{Source: GraphSourceConfig, Services: []model.ComposeGraphService{}, Networks: []string{}, Volumes: []string{}}
	for key, n := range raw.Networks {
		if n.Name != "" {
			g.Networks = append(g.Networks, n.Name)
		} else {
			g.Networks = append(g.Networks, key)
		}
	}
	for key := range raw.Volumes {
		g.Volumes = append(g.Volumes, key)
	}
	sort.Strings(g.Networks)
	sort.Strings(g.Volumes)
	for name, s := range raw.Services {
		g.Services = append(g.Services, model.ComposeGraphService{
			Name:      name,
			Image:     s.Image,
			Build:     isPresent(s.Build),
			Ports:     parsePorts(s.Ports),
			Volumes:   parseVolumes(s.Volumes),
			DependsOn: parseDependsOn(s.DependsOn),
			Profiles:  nonNil(s.Profiles),
		})
	}
	sort.Slice(g.Services, func(i, j int) bool { return g.Services[i].Name < g.Services[j].Name })
	return g, nil
}

func isPresent(r json.RawMessage) bool {
	t := bytes.TrimSpace(r)
	return len(t) > 0 && string(t) != "null"
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func isObject(r json.RawMessage) bool {
	t := bytes.TrimSpace(r)
	return len(t) > 0 && t[0] == '{'
}

func parsePorts(in []json.RawMessage) []model.ComposeGraphPort {
	out := []model.ComposeGraphPort{}
	for _, r := range in {
		if isObject(r) {
			var o struct {
				HostIP    string          `json:"host_ip"`
				Target    int             `json:"target"`
				Published json.RawMessage `json:"published"`
				Protocol  string          `json:"protocol"`
			}
			if json.Unmarshal(r, &o) == nil && o.Target > 0 {
				pub := strings.Trim(strings.TrimSpace(string(o.Published)), `"`)
				if pub == "null" {
					pub = ""
				}
				out = append(out, model.ComposeGraphPort{HostIP: o.HostIP, Published: pub, Target: o.Target, Protocol: o.Protocol})
			}
			continue
		}
		var s string
		if json.Unmarshal(r, &s) != nil {
			// A bare number is a container port.
			s = strings.TrimSpace(string(r))
		}
		if p, ok := parsePortString(s); ok {
			out = append(out, p)
		}
	}
	return out
}

// parsePortString reads "[ip:][published:]target[/proto]".
func parsePortString(s string) (model.ComposeGraphPort, bool) {
	p := model.ComposeGraphPort{Protocol: "tcp"}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		if proto := s[i+1:]; proto != "" {
			p.Protocol = proto
		}
		s = s[:i]
	}
	parts := strings.Split(s, ":")
	target, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || target <= 0 {
		return p, false
	}
	p.Target = target
	if len(parts) >= 2 {
		p.Published = parts[len(parts)-2]
	}
	if len(parts) >= 3 {
		p.HostIP = strings.Trim(strings.Join(parts[:len(parts)-2], ":"), "[]")
	}
	return p, true
}

func parseVolumes(in []json.RawMessage) []string {
	out := []string{}
	for _, r := range in {
		src := ""
		if isObject(r) {
			var o struct {
				Source string `json:"source"`
			}
			if json.Unmarshal(r, &o) != nil {
				continue
			}
			src = o.Source
		} else {
			var s string
			if json.Unmarshal(r, &s) != nil {
				continue
			}
			src = shortVolumeSource(s)
		}
		if src != "" {
			out = append(out, src)
		}
	}
	return out
}

// shortVolumeSource returns the source of "source:target[:mode]"; an anonymous
// volume ("/data") has none.
func shortVolumeSource(s string) string {
	// A Windows drive letter ("C:\data:/app") is part of the source.
	skip := 0
	if len(s) > 2 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') &&
		((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')) {
		skip = 2
	}
	i := strings.Index(s[skip:], ":")
	if i < 0 {
		return ""
	}
	return s[:skip+i]
}

func parseDependsOn(r json.RawMessage) []model.ComposeGraphDep {
	out := []model.ComposeGraphDep{}
	if !isPresent(r) {
		return out
	}
	var list []string
	if json.Unmarshal(r, &list) == nil {
		for _, n := range list {
			out = append(out, model.ComposeGraphDep{Service: n})
		}
		return out
	}
	var m map[string]struct {
		Condition string `json:"condition"`
	}
	if json.Unmarshal(r, &m) == nil {
		for n, d := range m {
			out = append(out, model.ComposeGraphDep{Service: n, Condition: d.Condition})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out
}

// GraphFromContainers is the fallback when the Compose config is unavailable:
// one node per service label, arrows only from the depends_on label Compose
// wrote ("name:condition:restart,..."). Nothing is inferred.
func GraphFromContainers(cs []Container) model.ComposeGraph {
	g := model.ComposeGraph{Source: GraphSourceContainers, Services: []model.ComposeGraphService{}, Networks: []string{}, Volumes: []string{}}
	seen := map[string]int{}
	for _, c := range cs {
		name := c.Labels[LabelService]
		if name == "" || isOneOff(c.Labels) {
			continue
		}
		if i, ok := seen[name]; ok {
			if g.Services[i].Image == "" {
				g.Services[i].Image = c.Image
			}
			continue
		}
		seen[name] = len(g.Services)
		g.Services = append(g.Services, model.ComposeGraphService{
			Name: name, Image: c.Image,
			Ports: []model.ComposeGraphPort{}, Volumes: []string{}, Profiles: []string{},
			DependsOn: parseDependsLabel(c.Labels[LabelDependsOn]),
		})
	}
	sort.Slice(g.Services, func(i, j int) bool { return g.Services[i].Name < g.Services[j].Name })
	return g
}

func parseDependsLabel(v string) []model.ComposeGraphDep {
	out := []model.ComposeGraphDep{}
	for _, item := range SplitList(v) {
		parts := strings.Split(item, ":")
		if parts[0] == "" {
			continue
		}
		d := model.ComposeGraphDep{Service: parts[0]}
		if len(parts) > 1 {
			d.Condition = parts[1]
		}
		out = append(out, d)
	}
	return out
}
