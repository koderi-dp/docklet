package engine

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"docklet/internal/model"
)

// commandRunner runs a command and returns its combined output. Overridable in tests.
type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

var (
	runCommand   commandRunner = realRunCommand
	wslSupported               = runtime.GOOS == "windows"
)

func realRunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)
	return cmd.CombinedOutput()
}

// decodeWSLOutput converts wsl.exe output to UTF-8. wsl.exe writes its own
// messages as UTF-16LE (with or without BOM); output relayed from a Linux
// process is UTF-8. NUL bytes are the discriminator.
func decodeWSLOutput(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		return decodeUTF16LE(b[2:])
	}
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return string(b[3:])
	}
	for i := 0; i < len(b) && i < 512; i++ {
		if b[i] == 0 {
			return decodeUTF16LE(b)
		}
	}
	return string(b)
}

func decodeUTF16LE(b []byte) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u))
}

// parseWSLList parses `wsl.exe -l -v`. The header line is skipped regardless of
// locale; columns are read from the right (VERSION, STATE) so names survive.
func parseWSLList(raw []byte) []model.WSLDistro {
	text := strings.ReplaceAll(decodeWSLOutput(raw), "\r", "")
	text = strings.TrimPrefix(text, "\ufeff")
	out := []model.WSLDistro{}
	header := true
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t\x00")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if header {
			header = false
			continue
		}
		def := false
		t := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(t, "*") {
			def = true
			t = strings.TrimLeft(t[1:], " \t")
		}
		f := strings.Fields(t)
		if len(f) < 3 {
			continue
		}
		ver, err := strconv.Atoi(f[len(f)-1])
		if err != nil {
			continue
		}
		out = append(out, model.WSLDistro{
			Name:    strings.Join(f[:len(f)-2], " "),
			State:   f[len(f)-2],
			Version: ver,
			Default: def,
		})
	}
	return out
}

// WSLDistros lists installed WSL distributions; empty (not an error) when WSL
// is unavailable or this is not Windows.
func (m *Manager) WSLDistros(ctx context.Context) ([]model.WSLDistro, error) {
	if !wslSupported {
		return []model.WSLDistro{}, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := runCommand(cctx, "wsl.exe", "-l", "-v")
	if err != nil {
		// wsl.exe missing or WSL not installed: no distros, not a failure.
		var ee *exec.ExitError
		if errors.As(err, &ee) || errors.Is(err, exec.ErrNotFound) {
			return []model.WSLDistro{}, nil
		}
		return nil, fmt.Errorf("listing WSL distributions: %w", err)
	}
	return parseWSLList(out), nil
}

func (m *Manager) wslRunning(ctx context.Context, distro string) bool {
	out, err := runCommand(ctx, "wsl.exe", "--list", "--running", "--quiet")
	if err != nil {
		return false
	}
	for _, l := range strings.Split(strings.ReplaceAll(decodeWSLOutput(out), "\r", ""), "\n") {
		if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(l, "\ufeff")), distro) {
			return true
		}
	}
	return false
}

func validDistro(d string) (string, error) {
	d = strings.TrimSpace(d)
	if d == "" || !safeDistro.MatchString(d) {
		return "", errors.New("invalid WSL distribution name")
	}
	return d, nil
}

// classifyWSLDocker interprets `docker version --format {{.Client.Version}}|{{.Server.Version}}`
// run inside a distro.
func classifyWSLDocker(distro, out string, runErr error) model.WSLDockerInfo {
	info := model.WSLDockerInfo{Distro: distro, Running: true}
	low := strings.ToLower(out)
	switch {
	case runErr == nil && strings.Contains(out, "|"):
		parts := strings.SplitN(firstLine(out), "|", 2)
		info.CLIFound = true
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			info.DaemonReachable = true
			info.ServerVersion = strings.TrimSpace(parts[1])
			info.Detail = "Docker Engine " + info.ServerVersion + " is running."
		} else {
			info.Detail = "Docker CLI found but the daemon did not answer."
		}
	case strings.Contains(low, "cannot connect to the docker daemon"), strings.Contains(low, "is the docker daemon running"):
		info.CLIFound = true
		info.Detail = "Docker CLI found, but the daemon is not running. Start it in the distribution (for example: sudo service docker start)."
	case strings.Contains(low, "permission denied"):
		info.CLIFound = true
		info.Detail = "Docker CLI found, but access to the daemon socket was denied. Add your user to the docker group in this distribution."
	case strings.Contains(low, "execvpe(docker)"), strings.Contains(low, "not found"), strings.Contains(low, "no such file"):
		info.Detail = "Docker was not found in this distribution. Install Docker CE there or choose another connection."
	default:
		info.Detail = "Docker status could not be determined."
		if t := strings.TrimSpace(Redact(out)); t != "" {
			info.Detail += " " + truncate(t, 300)
		}
	}
	return info
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// WSLDockerInfo detects Docker inside one distribution. It never boots a
// stopped distribution: that is reported as Running=false.
func (m *Manager) WSLDockerInfo(ctx context.Context, distro string) (model.WSLDockerInfo, error) {
	distro, err := validDistro(distro)
	if err != nil {
		return model.WSLDockerInfo{}, err
	}
	info := model.WSLDockerInfo{Distro: distro}
	if !wslSupported {
		info.Detail = "WSL is only available on Windows."
		return info, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if !m.wslRunning(cctx, distro) {
		info.Detail = "Distribution is stopped. Start it to detect Docker."
		return info, nil
	}
	out, runErr := runCommand(cctx, "wsl.exe", "-d", distro, "--exec", "docker", "version",
		"--format", "{{.Client.Version}}|{{.Server.Version}}")
	return classifyWSLDocker(distro, decodeWSLOutput(out), runErr), nil
}

// StartWSLDistro boots a distribution by running a no-op command. Explicit user action only.
func (m *Manager) StartWSLDistro(ctx context.Context, distro string) error {
	distro, err := validDistro(distro)
	if err != nil {
		return err
	}
	if !wslSupported {
		return errors.New("WSL is only available on Windows")
	}
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if out, err := runCommand(cctx, "wsl.exe", "-d", distro, "--exec", "/bin/true"); err != nil {
		return fmt.Errorf("could not start %s: %s", distro, truncate(Redact(strings.TrimSpace(decodeWSLOutput(out))+" "+err.Error()), 300))
	}
	return nil
}
