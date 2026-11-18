package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/moby/moby/api/types/system"

	"docklet/internal/model"
)

// KindFromVersion tells Podman from Docker using the Engine's own /version answer: Podman lists a
// "Podman Engine" component (and may name itself in the platform). Anything unrecognised is Docker.
func KindFromVersion(platformName string, components []system.ComponentVersion) model.EngineKind {
	if strings.Contains(strings.ToLower(platformName), "podman") {
		return model.EnginePodman
	}
	for _, c := range components {
		if strings.Contains(strings.ToLower(c.Name), "podman") {
			return model.EnginePodman
		}
	}
	return model.EngineDocker
}

// PodmanVersion is the version of the "Podman Engine" component, or "" when there is none.
func PodmanVersion(components []system.ComponentVersion) string {
	for _, c := range components {
		if strings.Contains(strings.ToLower(c.Name), "podman engine") {
			return c.Version
		}
	}
	return ""
}

// ---- machines (Windows and macOS) ----

// podmanMachine is one row of `podman machine list --format json`.
type podmanMachine struct {
	Name     string `json:"Name"`
	Running  bool   `json:"Running"`
	Starting bool   `json:"Starting"`
	VMType   string `json:"VMType"`
}

// parseMachineList reads `podman machine list --format json`. Empty output means no machines.
func parseMachineList(raw []byte) ([]podmanMachine, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, nil
	}
	var out []podmanMachine
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unexpected `podman machine list` output: %w", err)
	}
	return out, nil
}

// podmanInspect is the part of `podman machine inspect` Docklet reads: where the API listens.
type podmanInspect struct {
	Name           string `json:"Name"`
	ConnectionInfo struct {
		PodmanSocket struct{ Path string } `json:"PodmanSocket"`
		PodmanPipe   struct{ Path string } `json:"PodmanPipe"`
	} `json:"ConnectionInfo"`
}

func parseMachineInspect(raw []byte) ([]podmanInspect, error) {
	var out []podmanInspect
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unexpected `podman machine inspect` output: %w", err)
	}
	return out, nil
}

// shortMachineName drops the "podman-machine-" / "podman-" prefix Podman puts on its machines, so
// the connection reads "Podman (default)".
func shortMachineName(name string) string {
	for _, p := range []string{"podman-machine-", "podman-"} {
		if strings.HasPrefix(name, p) && len(name) > len(p) {
			return name[len(p):]
		}
	}
	return name
}

// podmanPipeName is the Windows named pipe of a machine: the machine name, prefixed with "podman-"
// unless it already starts with "podman".
func podmanPipeName(machine string) string {
	if strings.HasPrefix(machine, "podman") {
		return machine
	}
	return "podman-" + machine
}

func podmanPipeProfile(pipe string) model.ConnectionProfile {
	return model.ConnectionProfile{ID: "local-podman-" + pipe, Name: "Podman (" + shortMachineName(pipe) + ")", Type: model.ConnNamedPipe,
		Endpoint: "npipe:////./pipe/" + pipe, Discovered: true}
}

// podmanPipeProfiles turns the names found under the pipe directory into profiles: every pipe that
// starts with "podman-" is a machine's Docker-compatible API.
func podmanPipeProfiles(pipeNames []string) []model.ConnectionProfile {
	var out []model.ConnectionProfile
	for _, n := range pipeNames {
		if strings.HasPrefix(n, "podman-") {
			out = append(out, podmanPipeProfile(n))
		}
	}
	return out
}

// stoppedHint is what the user has to do for a machine that is not running.
func stoppedHint(machine string) string {
	return "Start it with: podman machine start " + machine
}

// podmanMachineInfos merges the CLI's machine list with the pipes that exist: a machine is running
// when the CLI says so and its pipe is there; a listed machine without a pipe is stopped.
func podmanMachineInfos(machines []podmanMachine, pipeNames []string) []model.PodmanMachineInfo {
	have := map[string]bool{}
	for _, n := range pipeNames {
		have[n] = true
	}
	var out []model.PodmanMachineInfo
	seen := map[string]bool{}
	for _, m := range machines {
		pipe := podmanPipeName(m.Name)
		seen[pipe] = true
		running := m.Running && have[pipe]
		info := model.PodmanMachineInfo{Name: shortMachineName(m.Name), Running: running, Profile: podmanPipeProfile(pipe)}
		if !running {
			info.Hint = stoppedHint(m.Name)
		}
		out = append(out, info)
	}
	for _, n := range pipeNames {
		if strings.HasPrefix(n, "podman-") && !seen[n] {
			out = append(out, model.PodmanMachineInfo{Name: shortMachineName(n), Running: true, Profile: podmanPipeProfile(n)})
		}
	}
	return out
}

// ---- sockets (Linux and macOS) ----

func podmanSocketProfile(id, name, path string) model.ConnectionProfile {
	return model.ConnectionProfile{ID: id, Name: name, Type: model.ConnUnix, Endpoint: "unix://" + filepath.ToSlash(path), Discovered: true}
}

// podmanLinuxCandidates lists the rootless (user) and rootful (system) Podman API sockets that exist.
func podmanLinuxCandidates(xdgRuntimeDir string, uid int, exists func(string) bool) []model.ConnectionProfile {
	var out []model.ConnectionProfile
	var rootless []string
	if xdgRuntimeDir != "" {
		rootless = append(rootless, xdgRuntimeDir+"/podman/podman.sock")
	}
	if uid > 0 {
		rootless = append(rootless, fmt.Sprintf("/run/user/%d/podman/podman.sock", uid))
	}
	for _, p := range rootless {
		if exists(p) {
			out = append(out, podmanSocketProfile("local-podman-rootless", "Podman (rootless)", p))
			break // XDG_RUNTIME_DIR usually is /run/user/<uid>: one connection, not two
		}
	}
	if exists("/run/podman/podman.sock") {
		out = append(out, podmanSocketProfile("local-podman-rootful", "Podman (rootful)", "/run/podman/podman.sock"))
	}
	return out
}

// podmanMacCandidates finds the machine sockets Podman creates on macOS. glob expands a pattern.
func podmanMacCandidates(home, tmpDir string, glob func(string) []string) []model.ConnectionProfile {
	var patterns []string
	if tmpDir != "" {
		patterns = append(patterns, strings.TrimRight(tmpDir, "/")+"/podman/*-api.sock")
	}
	if home != "" {
		patterns = append(patterns, home+"/.local/share/containers/podman/machine/*/podman.sock")
	}
	var out []model.ConnectionProfile
	seen := map[string]bool{}
	for _, pat := range patterns {
		for _, p := range glob(pat) {
			p = filepath.ToSlash(p)
			base := p[strings.LastIndex(p, "/")+1:]
			name := strings.TrimSuffix(base, "-api.sock")
			if base == "podman.sock" {
				dir := p[:strings.LastIndex(p, "/")]
				name = dir[strings.LastIndex(dir, "/")+1:]
			}
			name = shortMachineName(name)
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, podmanSocketProfile("local-podman-"+name, "Podman ("+name+")", p))
		}
	}
	return out
}
