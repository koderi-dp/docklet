package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"docklet/internal/model"
	"docklet/internal/secrets"
)

// sshTestArgs is a test-only hook: extra ssh arguments (e.g. -o
// UserKnownHostsFile=...) inserted before the target. Always nil in production.
var sshTestArgs []string

// proxyJumpPattern accepts [user@]host[:port] hops separated by commas. It can
// never start with '-', contain whitespace, control characters or shell/ssh
// option syntax.
var proxyJumpPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@\[\]:-]*(,[A-Za-z0-9][A-Za-z0-9._@\[\]:-]*)*$`)

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// validateSSHFields checks the optional SSH profile fields.
func validateSSHFields(p *model.ConnectionProfile) error {
	p.SSHUser = strings.TrimSpace(p.SSHUser)
	p.SSHIdentityFile = strings.TrimSpace(p.SSHIdentityFile)
	p.SSHProxyJump = strings.TrimSpace(p.SSHProxyJump)
	if p.SSHUser != "" && !safeToken.MatchString(p.SSHUser) {
		return errors.New("invalid ssh user name")
	}
	if p.SSHPort < 0 || p.SSHPort > 65535 {
		return errors.New("invalid ssh port")
	}
	if p.SSHProxyJump != "" && !proxyJumpPattern.MatchString(p.SSHProxyJump) {
		return errors.New("invalid ssh ProxyJump: use [user@]host[:port], comma separated")
	}
	if f := p.SSHIdentityFile; f != "" {
		if strings.HasPrefix(f, "-") || hasControl(f) {
			return errors.New("invalid ssh identity file path")
		}
		st, err := os.Stat(f)
		if err != nil || st.IsDir() {
			return fmt.Errorf("ssh identity file %q does not exist", f)
		}
	}
	return nil
}

// sshArgs builds argv (after "ssh"). Host key verification is always strict:
// unknown or changed host keys are refused, never auto-accepted. With askpass
// the password/passphrase prompt is allowed (answered by our askpass helper);
// otherwise BatchMode prevents hanging prompts.
func sshArgs(p model.ConnectionProfile, askpass bool) ([]string, error) {
	u, err := url.Parse(p.Endpoint)
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("invalid ssh endpoint")
	}
	if !safeToken.MatchString(u.Hostname()) {
		return nil, errors.New("invalid ssh host")
	}
	if err := validateSSHFields(&p); err != nil {
		return nil, err
	}
	args := []string{"-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=10"}
	if askpass {
		args = append(args, "-o", "BatchMode=no", "-o", "NumberOfPasswordPrompts=1")
	} else {
		args = append(args, "-o", "BatchMode=yes")
	}
	args = append(args, sshTestArgs...)
	user := p.SSHUser
	if user == "" && u.User != nil {
		user = u.User.Username()
		if !safeToken.MatchString(user) {
			return nil, errors.New("invalid ssh user name")
		}
	}
	if user != "" {
		args = append(args, "-l", user)
	}
	port := u.Port()
	if p.SSHPort > 0 {
		port = strconv.Itoa(p.SSHPort)
	}
	if port != "" {
		args = append(args, "-p", port)
	}
	if p.SSHIdentityFile != "" {
		args = append(args, "-i", p.SSHIdentityFile, "-o", "IdentitiesOnly=yes")
		if !p.SSHUseAgent {
			args = append(args, "-o", "IdentityAgent=none")
		}
	}
	if p.SSHProxyJump != "" {
		args = append(args, "-J", p.SSHProxyJump)
	}
	return append(args, "--", u.Hostname()), nil
}

// sshDialer spawns the system OpenSSH running `docker system dial-stdio`. When
// a password or key passphrase is stored for the profile, the ssh child gets an
// askpass helper (see internal/secrets) for the lifetime of that connection.
func sshDialer(p model.ConnectionProfile, st secrets.Store) (func(context.Context, string, string) (net.Conn, error), error) {
	if _, err := sshArgs(p, false); err != nil {
		return nil, err
	}
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pw := secrets.Lookup(st, p.ID, secrets.KindSSHPassword)
		pp := secrets.Lookup(st, p.ID, secrets.KindSSHPassphrase)
		var srv *secrets.AskpassServer
		if pw != "" || pp != "" {
			var err error
			if srv, err = secrets.StartAskpass(pw, pp); err != nil {
				return nil, err
			}
		}
		args, err := sshArgs(p, srv != nil)
		if err != nil {
			return nil, err
		}
		cmd := exec.Command("ssh", append(args, "docker", "system", "dial-stdio")...)
		hideWindow(cmd)
		if srv != nil {
			env, err := srv.Env()
			if err != nil {
				srv.Close()
				return nil, err
			}
			cmd.Env = append(os.Environ(), env...)
		}
		conn, err := startCmdConn(cmd)
		if err != nil {
			if srv != nil {
				srv.Close()
			}
			return nil, err
		}
		if srv != nil {
			conn.onClose = srv.Close
		}
		return conn, nil
	}, nil
}
