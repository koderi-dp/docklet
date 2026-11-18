package services

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"docklet/internal/compose"
	"docklet/internal/engine"
	"docklet/internal/model"
)

// termCommand is what the external terminal must run: argv plus the environment the
// docker CLI needs to reach the SAME engine as the connection. Env is applied inside the
// launch script, so it also works when the terminal is an already running server process.
type termCommand struct {
	Argv  []string
	Set   []string // KEY=VALUE
	Unset []string
	// NeedsBinary is the executable that must exist locally ("docker" or "ssh"); empty for WSL
	// (docker is checked inside the distribution).
	NeedsBinary string
	Distro      string
}

var containerRefRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// externalShell resolves the shell to start: "auto" and "" use /bin/sh (always present);
// otherwise the same rules as the embedded terminal.
func externalShell(shell string) (string, error) {
	s := strings.TrimSpace(shell)
	if s == "" || s == "auto" {
		return "/bin/sh", nil
	}
	c, err := shellCandidates(s)
	if err != nil {
		return "", err
	}
	return c[0], nil
}

// buildTermCommand builds `docker exec -it <container> <shell>` for the connection. User input is only
// ever a separate argv element and has been validated against a strict charset.
func buildTermCommand(p model.ConnectionProfile, containerID, shell string) (termCommand, error) {
	if !containerRefRe.MatchString(containerID) {
		return termCommand{}, fmt.Errorf("invalid container reference %q", containerID)
	}
	sh, err := externalShell(shell)
	if err != nil {
		return termCommand{}, err
	}
	exec := []string{"docker", "exec", "-it", containerID, sh}
	switch p.Type {
	case model.ConnWSL:
		if err := compose.ValidateDistro(p.WSLDistro); err != nil {
			return termCommand{}, err
		}
		return termCommand{Argv: append([]string{"wsl.exe", "-d", p.WSLDistro, "--"}, exec...), Distro: p.WSLDistro}, nil
	case model.ConnSSH:
		return sshTermCommand(p, exec)
	case model.ConnContext:
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.Endpoint)), "ssh://") {
			p.Type = model.ConnSSH
			return sshTermCommand(p, exec)
		}
	case model.ConnNamedPipe, model.ConnUnix, model.ConnTCP:
		env := compose.SelectEnv(p)
		if env.Kind != compose.KindLocal {
			return termCommand{}, errors.New(env.Reason)
		}
		return termCommand{Argv: exec, Set: env.EnvSet, Unset: env.EnvUnset, NeedsBinary: "docker"}, nil
	}
	return termCommand{}, fmt.Errorf("an external terminal is not supported for connection type %q", p.Type)
}

// sshTermCommand runs docker on the remote host: ssh -t [opts] -- user@host docker exec -it ...
func sshTermCommand(p model.ConnectionProfile, exec []string) (termCommand, error) {
	vp, err := engine.ValidateProfile(p)
	if err != nil {
		return termCommand{}, err
	}
	u, err := url.Parse(vp.Endpoint)
	if err != nil {
		return termCommand{}, errors.New("invalid ssh endpoint")
	}
	user := vp.SSHUser
	if u.User != nil && u.User.Username() != "" {
		user = u.User.Username()
	}
	port := u.Port()
	if port == "" && vp.SSHPort > 0 {
		port = strconv.Itoa(vp.SSHPort)
	}
	argv := []string{"ssh", "-t"}
	if port != "" {
		argv = append(argv, "-p", port)
	}
	if vp.SSHIdentityFile != "" {
		argv = append(argv, "-i", vp.SSHIdentityFile)
	}
	if vp.SSHProxyJump != "" {
		argv = append(argv, "-J", vp.SSHProxyJump)
	}
	dest := u.Hostname()
	if user != "" {
		dest = user + "@" + dest
	}
	argv = append(argv, "--", dest)
	return termCommand{Argv: append(argv, exec...), NeedsBinary: "ssh"}, nil
}
