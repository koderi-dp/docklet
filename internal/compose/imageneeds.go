package compose

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ImageNeed is the image of one service that Compose would pull rather than build.
type ImageNeed struct {
	Service    string
	Image      string
	PullPolicy string // as written in the file: "", "always", "missing", "never", ...
}

// ParseImageNeeds reads `docker compose config --format json` and returns, sorted by service,
// the services whose image comes from a registry. Services with a build section are left out:
// Compose builds those. With services given, only they and the services they depend on are returned.
func ParseImageNeeds(jsonText string, services []string) ([]ImageNeed, error) {
	var raw struct {
		Services map[string]struct {
			Image      string          `json:"image"`
			Build      json.RawMessage `json:"build"`
			PullPolicy string          `json:"pull_policy"`
			DependsOn  json.RawMessage `json:"depends_on"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return nil, fmt.Errorf("parse compose config: %w", err)
	}
	wanted := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		sv, ok := raw.Services[name]
		if !ok || wanted[name] {
			return
		}
		wanted[name] = true
		for _, d := range parseDependsOn(sv.DependsOn) {
			visit(d.Service)
		}
	}
	for _, name := range services {
		visit(name)
	}
	out := []ImageNeed{}
	for name, sv := range raw.Services {
		if (len(services) > 0 && !wanted[name]) || sv.Image == "" || isPresent(sv.Build) {
			continue
		}
		out = append(out, ImageNeed{Service: name, Image: sv.Image, PullPolicy: sv.PullPolicy})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out, nil
}
