package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"docklet/internal/compose"
	"docklet/internal/model"
)

// extDeps are the side effects of launching a terminal; tests replace them.
type extDeps struct {
	goos     string
	lookPath func(string) (string, error)
	tempDir  string
	start    func(argv []string) error
	// dockerInDistro checks that docker runs inside a WSL distribution.
	dockerInDistro func(ctx context.Context, distro string) error
}

func defaultExtDeps() extDeps {
	return extDeps{
		goos:     runtime.GOOS,
		lookPath: exec.LookPath,
		tempDir:  filepath.Join(os.TempDir(), "docklet-terminal"),
		start:    startDetached,
		dockerInDistro: func(ctx context.Context, distro string) error {
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_, err := compose.ExecRunner{}.Output(cctx, compose.Command{Name: "wsl.exe", Args: []string{"-d", distro, "--exec", "docker", "--version"}})
			return err
		},
	}
}

func startDetached(argv []string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	hideParentWindow(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// OpenExternalTerminal opens the user's terminal running `docker exec -it <container> <shell>`
// against the SAME engine as the connection. It needs the docker CLI on this machine (or inside
// the WSL distribution, or on the SSH host).
func (s *ExecService) OpenExternalTerminal(ctx context.Context, connectionID, containerID, shell string, terminal model.ExternalTerminal) error {
	if s.Profiles == nil {
		return errors.New("connection profiles are not available")
	}
	p, err := s.Profiles.Profile(connectionID)
	if err != nil {
		return err
	}
	tc, err := buildTermCommand(p, containerID, shell)
	if err != nil {
		return err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return err
	}
	running, err := api.ContainerRunning(ctx, containerID)
	if err != nil {
		return friendlyError(err)
	}
	if !running {
		return errors.New("The container is not running. Start it to open a terminal.")
	}
	return launchExternal(ctx, defaultExtDeps(), tc, terminal)
}

func launchExternal(ctx context.Context, d extDeps, tc termCommand, terminal model.ExternalTerminal) error {
	if err := checkCLI(ctx, d, tc); err != nil {
		return err
	}
	script, err := buildScript(d.goos, tc)
	if err != nil {
		return err
	}
	path, err := writeScript(d, script)
	if err != nil {
		return err
	}
	argv, err := terminalArgv(d.goos, terminal, path, d.lookPath)
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	if err := d.start(argv); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("could not start the terminal (%s): %w", argv[0], err)
	}
	return nil
}

func checkCLI(ctx context.Context, d extDeps, tc termCommand) error {
	switch {
	case tc.Distro != "":
		if err := d.dockerInDistro(ctx, tc.Distro); err != nil {
			return fmt.Errorf("The docker CLI is not available in the WSL distribution %q. Install it there to use an external terminal.", tc.Distro)
		}
	case tc.NeedsBinary == "ssh":
		if _, err := d.lookPath("ssh"); err != nil {
			return errors.New("The ssh client was not found on this machine's PATH. Install OpenSSH to use an external terminal for this connection.")
		}
	case tc.NeedsBinary == "docker":
		if _, err := d.lookPath("docker"); err != nil {
			return errors.New("The docker CLI was not found on this machine's PATH. Install it (the Docker Engine itself is not needed locally) to use an external terminal.")
		}
	}
	return nil
}

// writeScript stores the launch script in a private temp dir and removes stale ones.
func writeScript(d extDeps, content string) (string, error) {
	if err := os.MkdirAll(d.tempDir, 0o700); err != nil {
		return "", fmt.Errorf("could not prepare the terminal script: %w", err)
	}
	pruneOldScripts(d.tempDir)
	f, err := os.CreateTemp(d.tempDir, "term-*"+scriptExt(d.goos))
	if err != nil {
		return "", fmt.Errorf("could not create the terminal script: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	_ = os.Chmod(f.Name(), 0o700)
	return f.Name(), nil
}

func pruneOldScripts(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// terminalArgv picks the terminal program and the argv that runs the script in it.
func terminalArgv(goos string, t model.ExternalTerminal, script string, look func(string) (string, error)) ([]string, error) {
	inv := []string{"/bin/sh", script}
	if goos == "windows" {
		inv = []string{"cmd.exe", "/c", script}
	}
	if t.Mode == "custom" {
		return expandTemplate(t.Template, inv)
	}
	have := func(name string) bool { _, err := look(name); return err == nil }
	switch goos {
	case "windows":
		if have("wt.exe") {
			return append([]string{"wt.exe"}, inv...), nil
		}
		return append([]string{"cmd.exe", "/c", "start", ""}, inv...), nil
	case "darwin":
		return []string{"open", "-a", "Terminal", script}, nil
	}
	for _, c := range []struct{ name, flag string }{
		{"x-terminal-emulator", "-e"}, {"gnome-terminal", "--"}, {"konsole", "-e"}, {"xterm", "-e"},
	} {
		if have(c.name) {
			return append([]string{c.name, c.flag}, inv...), nil
		}
	}
	return nil, errors.New("no terminal emulator found (tried x-terminal-emulator, gnome-terminal, konsole, xterm). Set a custom terminal command in Settings.")
}

// ValidateTerminalTemplate checks a custom command template.
func ValidateTerminalTemplate(tpl string) error {
	if !strings.Contains(tpl, "{cmd}") {
		return errors.New("the custom terminal command must contain {cmd}, e.g. alacritty -e {cmd}")
	}
	if len(splitFields(tpl)) == 0 {
		return errors.New("the custom terminal command is empty")
	}
	return nil
}

// expandTemplate splits the template into argv (double quotes group words, no shell) and
// replaces a lone {cmd} word by the invocation; {cmd} inside a longer word becomes the joined text.
func expandTemplate(tpl string, inv []string) ([]string, error) {
	if err := ValidateTerminalTemplate(tpl); err != nil {
		return nil, err
	}
	var out []string
	for _, f := range splitFields(tpl) {
		switch {
		case f == "{cmd}":
			out = append(out, inv...)
		case strings.Contains(f, "{cmd}"):
			out = append(out, strings.ReplaceAll(f, "{cmd}", strings.Join(inv, " ")))
		default:
			out = append(out, f)
		}
	}
	return out, nil
}

func splitFields(s string) []string {
	var out []string
	var cur strings.Builder
	inQ, has := false, false
	for _, r := range s {
		switch {
		case r == '"':
			inQ, has = !inQ, true
		case (r == ' ' || r == '\t') && !inQ:
			if has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if has {
		out = append(out, cur.String())
	}
	return out
}
