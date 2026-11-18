package compose

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"docklet/internal/model"
)

// MinWatchVersion is the first Compose release where `compose watch` is generally available.
var MinWatchVersion = [3]int{2, 22, 0}

var versionRe = regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)

// ParseComposeVersion extracts major.minor.patch from `docker compose version [--short]` output
// ("5.7.0", "Docker Compose version v2.29.1-desktop.1"). ok is false when none is found.
func ParseComposeVersion(out string) (v [3]int, text string, ok bool) {
	m := versionRe.FindStringSubmatch(out)
	if m == nil {
		return v, "", false
	}
	for i := 0; i < 3; i++ {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	return v, m[1] + "." + m[2] + "." + m[3], true
}

// WatchSupported reports whether a Compose version has `watch`.
func WatchSupported(v [3]int) bool {
	for i := 0; i < 3; i++ {
		if v[i] != MinWatchVersion[i] {
			return v[i] > MinWatchVersion[i]
		}
	}
	return true
}

// VersionCommand is `docker compose version --short` in the execution environment.
func VersionCommand(env ExecEnv) (Command, bool) {
	switch env.Kind {
	case KindLocal:
		return Command{Name: "docker", Args: []string{"compose", "version", "--short"}, EnvSet: env.EnvSet, EnvUnset: env.EnvUnset}, true
	case KindWSL:
		if ValidateDistro(env.Distro) != nil {
			return Command{}, false
		}
		return Command{Name: "wsl.exe", Args: []string{"-d", env.Distro, "--exec", "docker", "compose", "version", "--short"}}, true
	}
	return Command{}, false
}

type watchRuleJSON struct {
	Path   string   `json:"path"`
	Action string   `json:"action"`
	Target string   `json:"target"`
	Ignore []string `json:"ignore"`
	Exec   struct {
		Command any `json:"command"`
	} `json:"exec"`
}

// ParseWatchRules reads services.<name>.develop.watch[] from `docker compose config --format
// json` output. Services without rules are left out and the rest sorted by name; rules keep
// their file order. workDir (the project directory) is only used to shorten paths for display.
func ParseWatchRules(configJSON []byte, workDir string) ([]model.ComposeWatchService, error) {
	var cfg struct {
		Services map[string]struct {
			Develop struct {
				Watch []watchRuleJSON `json:"watch"`
			} `json:"develop"`
		} `json:"services"`
	}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("the Compose configuration is not valid JSON: %w", err)
	}
	names := make([]string, 0, len(cfg.Services))
	for n, s := range cfg.Services {
		if len(s.Develop.Watch) > 0 {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	out := make([]model.ComposeWatchService, 0, len(names))
	for _, n := range names {
		svc := model.ComposeWatchService{Name: n}
		for _, r := range cfg.Services[n].Develop.Watch {
			ign := r.Ignore
			if ign == nil {
				ign = []string{}
			}
			svc.Rules = append(svc.Rules, model.ComposeWatchRule{
				Action: r.Action, Path: r.Path, DisplayPath: DisplayPath(workDir, r.Path),
				Target: r.Target, Ignore: ign, Exec: execText(r.Exec.Command),
			})
		}
		out = append(out, svc)
	}
	return out, nil
}

func execText(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, p := range v {
			parts = append(parts, fmt.Sprint(p))
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// DisplayPath shows p relative to dir ("./site") when it lies inside it, else unchanged.
func DisplayPath(dir, p string) string {
	if dir == "" || p == "" {
		return p
	}
	norm := func(s string) string { return strings.TrimRight(strings.ReplaceAll(s, `\`, "/"), "/") }
	d, q := norm(dir), norm(p)
	rest := q
	if len(d) >= 2 && d[1] == ':' { // Windows drive paths compare case-insensitively
		d, q = strings.ToLower(d), strings.ToLower(q)
	}
	switch {
	case q == d:
		return "."
	case strings.HasPrefix(q, d+"/"):
		return "./" + rest[len(d)+1:]
	}
	return p
}
