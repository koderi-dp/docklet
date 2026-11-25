package compose

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ParseCloneCollisions reads `docker compose config --format json` and lists, as sentences for
// the user, what a copy of the project under another name would still share with the original
// or fight over: fixed host ports, container names, volumes that keep one name for every
// project, and bind mounts from outside the project folder dir.
func ParseCloneCollisions(jsonText, dir string) ([]string, error) {
	var raw struct {
		Name     string `json:"name"`
		Services map[string]struct {
			ContainerName string            `json:"container_name"`
			Ports         []json.RawMessage `json:"ports"`
			Volumes       []json.RawMessage `json:"volumes"`
		} `json:"services"`
		Volumes map[string]struct {
			Name     string          `json:"name"`
			External json.RawMessage `json:"external"`
		} `json:"volumes"`
	}
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return nil, fmt.Errorf("parse compose config: %w", err)
	}
	out := []string{}
	names := make([]string, 0, len(raw.Services))
	for n := range raw.Services {
		names = append(names, n)
	}
	sort.Strings(names)
	root := strings.TrimRight(strings.ReplaceAll(dir, "\\", "/"), "/") + "/"
	for _, n := range names {
		sv := raw.Services[n]
		if sv.ContainerName != "" {
			out = append(out, fmt.Sprintf("Service %s has container_name %q: a container name can exist once, so the copy cannot start while the original has its container.", n, sv.ContainerName))
		}
		for _, p := range parsePorts(sv.Ports) {
			if p.Published != "" {
				out = append(out, fmt.Sprintf("Service %s publishes host port %s: only one of the two projects can hold it at a time.", n, p.Published))
			}
		}
		for _, v := range sv.Volumes {
			var m struct {
				Type   string `json:"type"`
				Source string `json:"source"`
			}
			if json.Unmarshal(v, &m) != nil || m.Type != "bind" || m.Source == "" {
				continue
			}
			if src := strings.ReplaceAll(m.Source, "\\", "/"); !strings.HasPrefix(strings.ToLower(src+"/"), strings.ToLower(root)) {
				out = append(out, fmt.Sprintf("Service %s mounts %s, which is outside the project folder: both projects use the same files there.", n, m.Source))
			}
		}
	}
	vols := make([]string, 0, len(raw.Volumes))
	for k := range raw.Volumes {
		vols = append(vols, k)
	}
	sort.Strings(vols)
	for _, k := range vols {
		v := raw.Volumes[k]
		name := v.Name
		if name == "" {
			name = k
		}
		switch {
		case isPresent(v.External) && string(v.External) != "false":
			out = append(out, fmt.Sprintf("Volume %s is external (%q): both projects use the same data.", k, name))
		case raw.Name != "" && v.Name != "" && v.Name != raw.Name+"_"+k:
			out = append(out, fmt.Sprintf("Volume %s has the fixed name %q: both projects use the same data.", k, name))
		}
	}
	return out, nil
}
