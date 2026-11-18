package compose

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"docklet/internal/model"
)

// FS answers whether project files are visible to the Compose CLI environment.
type FS interface {
	FileExists(ctx context.Context, path string) bool
	DirExists(ctx context.Context, path string) bool
}

// LocalFS checks this machine's filesystem.
type LocalFS struct{}

func (LocalFS) FileExists(_ context.Context, p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func (LocalFS) DirExists(_ context.Context, p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// WSLFS checks files inside a WSL distribution with `wsl.exe -d <distro> --exec test`.
type WSLFS struct {
	Runner Runner
	Distro string
}

func (w WSLFS) test(ctx context.Context, flag, p string) bool {
	if ValidateDistro(w.Distro) != nil || validatePath("path", p) != nil || !strings.HasPrefix(p, "/") {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := w.Runner.Output(cctx, Command{Name: "wsl.exe", Args: []string{"-d", w.Distro, "--exec", "test", flag, p}})
	return err == nil
}

func (w WSLFS) FileExists(ctx context.Context, p string) bool { return w.test(ctx, "-f", p) }
func (w WSLFS) DirExists(ctx context.Context, p string) bool  { return w.test(ctx, "-d", p) }

// CLIStatus is the result of `docker compose version` in the execution environment.
type CLIStatus struct {
	OK     bool
	Detail string
}

// CheckCLI runs `docker compose version` in the environment.
func CheckCLI(ctx context.Context, r Runner, env ExecEnv) CLIStatus {
	var c Command
	switch env.Kind {
	case KindLocal:
		c = Command{Name: "docker", Args: []string{"compose", "version"}, EnvSet: env.EnvSet, EnvUnset: env.EnvUnset}
	case KindWSL:
		if ValidateDistro(env.Distro) != nil {
			return CLIStatus{Detail: "invalid WSL distribution name"}
		}
		c = Command{Name: "wsl.exe", Args: []string{"-d", env.Distro, "--exec", "docker", "compose", "version"}}
	default:
		return CLIStatus{Detail: env.Reason}
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := r.Output(cctx, c); err != nil {
		if env.Kind == KindWSL {
			return CLIStatus{Detail: fmt.Sprintf("docker compose is not available in the WSL distribution %q.", env.Distro)}
		}
		return CLIStatus{Detail: "docker compose is not installed or not runnable on this machine."}
	}
	return CLIStatus{OK: true}
}

// AssessInput is the pure input of Assess.
type AssessInput struct {
	Env         ExecEnv
	HostOS      string // runtime.GOOS of this machine
	ConfigFiles []string
	EnvFiles    []string
	WorkingDir  string
	FS          FS
	CLI         func() CLIStatus // evaluated lazily, only when files are visible
}

var winDriveRe = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// foreignPath reports a label path that cannot belong to this machine's OS.
func foreignPath(hostOS, p string) bool {
	if hostOS == "windows" {
		return strings.HasPrefix(p, "/")
	}
	return winDriveRe.MatchString(p)
}

// Assess decides whether the CLI can operate the project, and why not.
// A project discovered from labels is never operable on labels alone.
func Assess(ctx context.Context, in AssessInput) model.ComposeCapability {
	cap := model.ComposeCapability{
		ExecutionLocation: in.Env.Location,
		Warning:           in.Env.Warning,
		ConfigFiles:       append([]string{}, in.ConfigFiles...),
		WorkingDir:        in.WorkingDir,
	}
	deny := func(format string, a ...any) model.ComposeCapability {
		cap.Reason = fmt.Sprintf(format, a...)
		return cap
	}
	if in.Env.Kind == KindNone {
		return deny("%s", in.Env.Reason)
	}
	if len(in.ConfigFiles) == 0 {
		return deny("The project's containers carry no config-file label, so the Compose file location is unknown.")
	}
	files := append(append([]string{}, in.ConfigFiles...), in.EnvFiles...)
	var missing []string
	foreign := false
	for _, f := range files {
		if in.FS == nil || !in.FS.FileExists(ctx, f) {
			missing = append(missing, f)
			if in.Env.Kind == KindLocal && foreignPath(in.HostOS, f) {
				foreign = true
			}
		}
	}
	if len(missing) > 0 {
		list := strings.Join(missing, ", ")
		switch {
		case foreign:
			return deny("The Compose files (%s) have paths from another operating system and do not exist on this machine, so the Compose CLI cannot see them. Connect through WSL to run Compose where the files live.", list)
		case in.Env.Kind == KindWSL:
			return deny("Compose file(s) not found inside %s: %s", in.Env.Location, list)
		default:
			return deny("Compose file(s) not found on this machine: %s", list)
		}
	}
	if in.WorkingDir != "" && (in.FS == nil || !in.FS.DirExists(ctx, in.WorkingDir)) {
		return deny("The project working directory %s does not exist where the Compose CLI runs (%s).", in.WorkingDir, in.Env.Location)
	}
	if in.CLI != nil {
		if st := in.CLI(); !st.OK {
			return deny("%s", st.Detail)
		}
	}
	cap.CanRun = true
	return cap
}
