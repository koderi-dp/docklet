package engine

import (
	"errors"
	"net"
	"regexp"
	"strings"
)

var redactors = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?(-----END [A-Z ]*PRIVATE KEY-----|$)`), "[REDACTED PRIVATE KEY]"},
	{regexp.MustCompile(`(?i)(://[^/\s:@]*):[^@\s/]+@`), "$1:***@"},
	{regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]+`), "$1 ***"},
	{regexp.MustCompile(`(?i)(["']?(?:password|passwd|passphrase|pwd|secret|token|api[_-]?key|authorization|x-registry-auth|identitytoken)["']?\s*[=:]\s*)("[^"]*"|'[^']*'|\S+)`), "$1***"},
}

// Redact removes credential-like substrings from diagnostic text.
func Redact(s string) string {
	for _, r := range redactors {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}

// friendlyError translates a connection failure into an actionable message.
// The raw error is kept separately (redacted) in Diagnostics.
func friendlyError(err error, endpoint string) string {
	if err == nil {
		return ""
	}
	raw := strings.ToLower(err.Error())
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(raw, s) {
				return true
			}
		}
		return false
	}
	var netErr net.Error
	switch {
	case has("not supported yet"):
		return err.Error()
	case has("host key verification failed", "remote host identification has changed", "host key is known", "strict checking", "host key for"):
		return "SSH host key verification failed: the host key is unknown or has changed. Docklet never accepts host keys automatically. Verify the fingerprint, then trust the host once from a terminal (ssh user@host) so it is added to your known_hosts file, or fix the changed key with ssh-keygen -R host."
	case has("permission denied (publickey", "permission denied, please try again", "authentication failed", "too many authentication failures"):
		return "SSH authentication failed. Check the user name, the identity file or SSH agent, and any stored password or key passphrase."
	case has("x509", "certificate", "tls:"):
		return "TLS certificate verification failed. Check the CA, client certificate and that the host name matches the server certificate."
	case has("execvpe(docker)"):
		return "Docker Engine was not found in the WSL distribution. Install Docker CE in this distribution or choose another connection."
	case has("no distribution with the supplied name", "wsl_e_distro_not_found"):
		return "The WSL distribution was not found. Check the distribution name (wsl -l -v)."
	case has("docker: command not found", "docker: not found", "executable file not found", "exec: \"docker\""):
		return "Docker is not installed in the target environment (docker command not found)."
	case has("too old", "minimum supported api version"):
		return "The Docker Engine API version is incompatible with this app. Update Docker Engine."
	case has("permission denied", "access is denied"):
		return "Permission denied connecting to Docker. Add your user to the docker group (Linux) or docker-users group (Windows), or check socket permissions."
	case has("cannot find the file specified", "no such file or directory", "connection refused", "actively refused", "is the docker daemon running", "cannot connect to the docker daemon", "no such host", "could not resolve hostname", "name or service not known", "connection timed out", "operation timed out", "connection closed by", "network is unreachable", "no route to host", "eof"):
		return "Docker Engine is not reachable at " + endpoint + ". Make sure it is installed and running."
	case has("deadline exceeded", "timed out", "timeout"), errors.As(err, &netErr) && netErr.Timeout():
		return "Connecting to Docker Engine timed out. Check the address and that the daemon is running."
	}
	return "Could not connect to Docker Engine. See diagnostics for details."
}
