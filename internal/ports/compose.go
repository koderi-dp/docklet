package ports

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ProjectPorts are the host ports a Compose project publishes, with where they were read from.
type ProjectPorts struct {
	Requests        []Request
	OwnContainerIDs []string // the project's existing containers: their bindings are not conflicts
	Source          string   // "compose-config", "containers" or "none"
	Note            string   // why the ports could not be read from the configuration, if so
}

// ProjectCheck is the answer for a project: one PortCheck per published host port.
type ProjectCheck struct {
	Checks []PortCheck `json:"checks"`
	Source string      `json:"source"`
	Note   string      `json:"note"`
}

const maxRange = 64

// ParseComposePorts reads the published host ports from `docker compose config --format
// json` output. service restricts it to one service. Ports without a fixed host port
// (random) and ranges beyond maxRange entries are skipped.
func ParseComposePorts(raw []byte, service string) ([]Request, error) {
	var doc struct {
		Services map[string]struct {
			Ports []struct {
				HostIP    string `json:"host_ip"`
				Published any    `json:"published"`
				Protocol  string `json:"protocol"`
			} `json:"ports"`
		} `json:"services"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("the Compose configuration is not valid JSON: %w", err)
	}
	names := make([]string, 0, len(doc.Services))
	for n := range doc.Services {
		if service == "" || n == service {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var out []Request
	seen := map[Request]bool{}
	for _, n := range names {
		for _, p := range doc.Services[n].Ports {
			lo, hi, ok := publishedRange(p.Published)
			if !ok {
				continue
			}
			for port := lo; port <= hi && port-lo < maxRange; port++ {
				r := Request{HostIP: p.HostIP, HostPort: port, Protocol: normProto(p.Protocol)}
				if !seen[r] {
					seen[r] = true
					out = append(out, r)
				}
			}
		}
	}
	return out, nil
}

// publishedRange reads "8080", 8080 or "8000-8002".
func publishedRange(v any) (lo, hi int, ok bool) {
	var s string
	switch t := v.(type) {
	case string:
		s = strings.TrimSpace(t)
	case float64:
		s = strconv.Itoa(int(t))
	}
	a, b, isRange := strings.Cut(s, "-")
	lo, err := strconv.Atoi(a)
	if err != nil || lo < 1 || lo > 65535 {
		return 0, 0, false
	}
	if !isRange {
		return lo, lo, true
	}
	hi, err = strconv.Atoi(b)
	if err != nil || hi < lo || hi > 65535 {
		return lo, lo, true
	}
	return lo, hi, true
}
