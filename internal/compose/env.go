package compose

import (
	"fmt"
	"strings"

	"docklet/internal/engine"
	"docklet/internal/model"
)

// Kind is where the Compose CLI runs.
type Kind string

const (
	KindLocal Kind = "local" // docker compose on this machine
	KindWSL   Kind = "wsl"   // docker compose inside a WSL distro
	KindNone  Kind = "none"  // not supported for this transport
)

// ExecEnv describes WHERE the CLI runs, independent of where the Engine runs.
type ExecEnv struct {
	Kind     Kind
	Location string // "This machine" | "WSL: Ubuntu"
	Distro   string
	// EnvSet / EnvUnset are applied on top of the app environment (local only).
	// DOCKER_HOST is always set explicitly; the user's docker context is never read or changed.
	EnvSet   []string
	EnvUnset []string
	Warning  string
	Reason   string // set when Kind == KindNone
}

// dockerEnvVars are removed from the inherited environment so the profile endpoint wins
// (DOCKER_CONTEXT together with DOCKER_HOST makes the CLI refuse to run).
var dockerEnvVars = []string{"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH", "DOCKER_TLS"}

// SelectEnv chooses the execution environment for a connection profile.
func SelectEnv(p model.ConnectionProfile) ExecEnv {
	switch p.Type {
	case model.ConnNamedPipe, model.ConnUnix:
		return ExecEnv{Kind: KindLocal, Location: "This machine", EnvUnset: dockerEnvVars,
			EnvSet: []string{"DOCKER_HOST=" + p.Endpoint}}
	case model.ConnTCP:
		set := []string{"DOCKER_HOST=" + p.Endpoint}
		if engine.UsesTLS(p) {
			set = append(set, "DOCKER_TLS_VERIFY=1")
			if d := strings.TrimSpace(p.TLSCertDir); d != "" {
				set = append(set, "DOCKER_CERT_PATH="+d)
			}
		}
		return ExecEnv{Kind: KindLocal, Location: "This machine", EnvUnset: dockerEnvVars, EnvSet: set,
			Warning: "The Compose CLI runs on this machine against a remote Engine: relative bind mounts and build contexts resolve on the remote Engine host, not here."}
	case model.ConnWSL:
		if err := ValidateDistro(p.WSLDistro); err != nil {
			return ExecEnv{Kind: KindNone, Location: "WSL", Reason: err.Error()}
		}
		return ExecEnv{Kind: KindWSL, Location: "WSL: " + p.WSLDistro, Distro: p.WSLDistro}
	case model.ConnSSH:
		return sshEnv(p.Endpoint)
	case model.ConnContext:
		// An imported context normally becomes a tcp/ssh/npipe profile; only ssh:// can run here.
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.Endpoint)), "ssh://") {
			return sshEnv(p.Endpoint)
		}
		return ExecEnv{Kind: KindNone, Location: "Not available",
			Reason: "Compose is only supported for Docker contexts that use an ssh:// endpoint; import other contexts as tcp, unix or npipe connections."}
	}
	return ExecEnv{Kind: KindNone, Location: "Not available", Reason: fmt.Sprintf("Compose is not supported for connection type %q.", p.Type)}
}

// MergeEnv applies unset then set on top of base (KEY=VALUE lists). Keys compare
// case-insensitively (Windows environments are case-insensitive).
func MergeEnv(base, set, unset []string) []string {
	drop := map[string]bool{}
	for _, k := range unset {
		drop[strings.ToUpper(k)] = true
	}
	for _, kv := range set {
		k, _, _ := strings.Cut(kv, "=")
		drop[strings.ToUpper(k)] = true
	}
	out := make([]string, 0, len(base)+len(set))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if !drop[strings.ToUpper(k)] {
			out = append(out, kv)
		}
	}
	return append(out, set...)
}

// SSHWarning is shown for every Compose project operated over ssh://.
const SSHWarning = "The Compose CLI runs on this machine and reaches the remote Engine through ssh (DOCKER_HOST=ssh://). " +
	"Compose files are read from this machine and a build context is sent from this machine; " +
	"relative bind-mount paths resolve on the REMOTE host and are neither copied nor rewritten."

// sshEnv validates the endpoint with the same rules the engine package applies to
// ssh profiles and then runs the local CLI with DOCKER_HOST=ssh://user@host[:port].
func sshEnv(endpoint string) ExecEnv {
	prof, err := engine.ValidateProfile(model.ConnectionProfile{Name: "ssh", Type: model.ConnSSH, Endpoint: endpoint})
	if err != nil {
		return ExecEnv{Kind: KindNone, Location: "Not available", Reason: "Invalid ssh endpoint: " + err.Error()}
	}
	return ExecEnv{Kind: KindLocal, Location: "This machine", EnvUnset: dockerEnvVars,
		EnvSet: []string{"DOCKER_HOST=" + prof.Endpoint}, Warning: SSHWarning}
}
