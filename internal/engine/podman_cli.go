package engine

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"docklet/internal/model"
)

// findPodmanCLI locates the podman executable: PATH first, then where its installers put it.
func findPodmanCLI() string {
	if p, err := exec.LookPath("podman"); err == nil {
		return p
	}
	var dirs []string
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			dirs = append(dirs, filepath.Join(v, "Programs", "Podman"))
		}
		if v := os.Getenv("ProgramFiles"); v != "" {
			dirs = append(dirs, filepath.Join(v, "RedHat", "Podman"))
		}
	case "darwin":
		dirs = []string{"/opt/podman/bin", "/opt/homebrew/bin", "/usr/local/bin"}
	default:
		return ""
	}
	exe := "podman"
	if runtime.GOOS == "windows" {
		exe = "podman.exe"
	}
	for _, d := range dirs {
		if st, err := os.Stat(filepath.Join(d, exe)); err == nil && !st.IsDir() {
			return filepath.Join(d, exe)
		}
	}
	return ""
}

func runPodman(ctx context.Context, cli string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, args...)
	hideWindow(cmd)
	return cmd.Output()
}

// PodmanMachines lists the Podman machines on Windows and macOS. It only reads: a stopped machine
// is reported with the hint to start it and is never started here. Without the podman CLI the
// machines whose pipe exists are still listed (as running); stopped ones cannot be known then.
func PodmanMachines(ctx context.Context) ([]model.PodmanMachineInfo, error) {
	switch runtime.GOOS {
	case "windows":
	case "darwin":
		return macMachines(ctx)
	default:
		// Linux has no machines: the rootless and rootful sockets that exist are the engines.
		var out []model.PodmanMachineInfo
		for _, p := range podmanProfiles() {
			name := strings.TrimSuffix(strings.TrimPrefix(p.Name, "Podman ("), ")")
			out = append(out, model.PodmanMachineInfo{Name: name, Running: true, Profile: p})
		}
		return out, nil
	}
	pipes := machinePipes()
	cli := findPodmanCLI()
	if cli == "" {
		return podmanMachineInfos(nil, pipes), nil
	}
	raw, err := runPodman(ctx, cli, "machine", "list", "--format", "json")
	if err != nil {
		return podmanMachineInfos(nil, pipes), errors.New("could not run `podman machine list`: " + err.Error())
	}
	machines, err := parseMachineList(raw)
	if err != nil {
		return podmanMachineInfos(nil, pipes), err
	}
	return podmanMachineInfos(machines, pipes), nil
}

// macMachines asks the CLI for each running machine's socket: the path depends on the Podman version.
func macMachines(ctx context.Context) ([]model.PodmanMachineInfo, error) {
	var out []model.PodmanMachineInfo
	known := map[string]bool{}
	if cli := findPodmanCLI(); cli != "" {
		raw, err := runPodman(ctx, cli, "machine", "list", "--format", "json")
		if err != nil {
			return nil, errors.New("could not run `podman machine list`: " + err.Error())
		}
		machines, err := parseMachineList(raw)
		if err != nil {
			return nil, err
		}
		for _, m := range machines {
			info := model.PodmanMachineInfo{Name: shortMachineName(m.Name), Running: m.Running}
			if !m.Running {
				info.Hint = stoppedHint(m.Name)
			} else if ins, err := runPodman(ctx, cli, "machine", "inspect", m.Name); err == nil {
				if list, err := parseMachineInspect(ins); err == nil && len(list) > 0 && list[0].ConnectionInfo.PodmanSocket.Path != "" {
					info.Profile = podmanSocketProfile("local-podman-"+info.Name, "Podman ("+info.Name+")", list[0].ConnectionInfo.PodmanSocket.Path)
				}
			}
			if info.Profile.Endpoint == "" {
				info.Profile = model.ConnectionProfile{ID: "local-podman-" + info.Name, Name: "Podman (" + info.Name + ")", Type: model.ConnUnix, Discovered: true}
			}
			known[info.Profile.Endpoint] = true
			out = append(out, info)
		}
	}
	for _, p := range podmanProfiles() {
		if !known[p.Endpoint] {
			out = append(out, model.PodmanMachineInfo{Name: shortMachineName(p.Name), Running: true, Profile: p})
		}
	}
	return out, nil
}
