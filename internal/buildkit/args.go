// Package buildkit runs image builds through the docker CLI (docker buildx build,
// or DOCKER_BUILDKIT=1 docker build) and turns its progress output into a step
// list. Everything here is pure: command construction and output parsing.
package buildkit

import (
	"fmt"
	"regexp"
	"strings"

	"docklet/internal/compose"
)

// Methods of running a BuildKit build.
const (
	MethodBuildx      = "buildx"
	MethodDockerBuild = "docker-build"
	MethodClassic     = "classic" // only used to print the equivalent command of an Engine API build
)

// Arg is one build argument. Values are only ever used to start the process.
type Arg struct {
	Name  string
	Value string
}

// Options is everything the CLI needs. Paths are as the CLI sees them (inside
// WSL for WSL connections).
type Options struct {
	Method     string
	RawJSON    bool // --progress=rawjson instead of plain
	ContextDir string
	Dockerfile string // path to the Dockerfile as the CLI sees it; "" = <context>/Dockerfile
	Tags       []string
	Args       []Arg
	Target     string
	Platform   string
	NoCache    bool
	Pull       bool
}

var (
	secretNameRe = regexp.MustCompile(`(?i)(secret|token|passw|pwd|passphrase|credential|auth|private|api[_-]?key|access[_-]?key|(^|[_.-])key($|[_.-]))`)
	envSafeRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	reservedEnv  = regexp.MustCompile(`(?i)^(path|home|user|shell|wslenv|pwd|oldpwd|lang|term|tmp|temp|systemroot|comspec|appdata|(docker|buildx|buildkit|compose)_.*)$`)
)

// IsSecretName reports whether a build-argument name looks like it holds a
// credential. Such values are masked in the UI and never written to the
// command line, logs or history.
func IsSecretName(name string) bool { return secretNameRe.MatchString(name) }

// envBacked reports whether the value of an argument is handed to the CLI through
// its environment (`--build-arg NAME` with NAME set) instead of argv.
func envBacked(name string) bool {
	return IsSecretName(name) && envSafeRe.MatchString(name) && !reservedEnv.MatchString(name)
}

// Argv returns the arguments after `docker`, plus environment entries (KEY=VALUE)
// that carry the values of secret-looking build arguments so they stay out of the
// process list.
func Argv(o Options) (args, env []string, err error) {
	if o.ContextDir == "" {
		return nil, nil, fmt.Errorf("no build context")
	}
	switch o.Method {
	case MethodBuildx:
		args = []string{"buildx", "build", "--builder", "default", "--load"}
		if o.RawJSON {
			args = append(args, "--progress=rawjson")
		} else {
			args = append(args, "--progress=plain")
		}
	case MethodDockerBuild:
		args = []string{"build", "--progress=plain"}
		env = append(env, "DOCKER_BUILDKIT=1")
	default:
		return nil, nil, fmt.Errorf("unknown build method %q", o.Method)
	}
	args = appendCommon(args, o, func(a Arg) []string {
		if envBacked(a.Name) {
			env = append(env, a.Name+"="+a.Value)
			return []string{"--build-arg", a.Name}
		}
		return []string{"--build-arg", a.Name + "=" + a.Value}
	})
	return append(args, o.ContextDir), env, nil
}

func appendCommon(args []string, o Options, arg func(Arg) []string) []string {
	if o.Dockerfile != "" {
		args = append(args, "-f", o.Dockerfile)
	}
	for _, t := range o.Tags {
		args = append(args, "-t", t)
	}
	for _, a := range o.Args {
		args = append(args, arg(a)...)
	}
	if o.Target != "" {
		args = append(args, "--target", o.Target)
	}
	if o.Platform != "" {
		args = append(args, "--platform", o.Platform)
	}
	if o.NoCache {
		args = append(args, "--no-cache")
	}
	if o.Pull {
		args = append(args, "--pull")
	}
	return args
}

// CopyCommand is the equivalent command line for a terminal. Argument values are
// replaced by $NAME, so it can be shared without leaking anything.
func CopyCommand(o Options) string {
	var parts []string
	switch o.Method {
	case MethodDockerBuild:
		parts = []string{"DOCKER_BUILDKIT=1", "docker", "build"}
	case MethodClassic:
		parts = []string{"DOCKER_BUILDKIT=0", "docker", "build"}
	default:
		parts = []string{"docker", "buildx", "build", "--load"}
	}
	var args []string
	args = appendCommon(args, o, func(a Arg) []string {
		return []string{"--build-arg", "\x00" + a.Name + "=$" + a.Name} // \x00: emit unquoted
	})
	for _, a := range args {
		if strings.HasPrefix(a, "\x00") {
			parts = append(parts, a[1:])
		} else {
			parts = append(parts, shellQuote(a))
		}
	}
	parts = append(parts, shellQuote(o.ContextDir))
	return strings.Join(parts, " ")
}

var shellSafeRe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func shellQuote(s string) string {
	if s != "" && shellSafeRe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Command wraps Argv for the execution environment of the connection (the same
// places the Compose CLI runs). wslenv is the current WSLENV of this process.
func Command(env compose.ExecEnv, o Options, wslenv string) (compose.Command, error) {
	args, extra, err := Argv(o)
	if err != nil {
		return compose.Command{}, err
	}
	switch env.Kind {
	case compose.KindLocal:
		return compose.Command{Name: "docker", Args: args, EnvSet: append(append([]string{}, env.EnvSet...), extra...), EnvUnset: env.EnvUnset}, nil
	case compose.KindWSL:
		if err := compose.ValidateDistro(env.Distro); err != nil {
			return compose.Command{}, err
		}
		if !strings.HasPrefix(o.ContextDir, "/") {
			return compose.Command{}, fmt.Errorf("path %q is not an absolute path inside the WSL distribution", o.ContextDir)
		}
		w := []string{"-d", env.Distro, "--cd", o.ContextDir, "--exec", "docker"}
		cmd := compose.Command{Name: "wsl.exe", Args: append(w, args...)}
		if len(extra) > 0 {
			// WSLENV makes wsl.exe pass these variables into the distribution.
			names := make([]string, 0, len(extra)+1)
			if wslenv != "" {
				names = append(names, wslenv)
			}
			for _, kv := range extra {
				k, _, _ := strings.Cut(kv, "=")
				names = append(names, k)
			}
			cmd.EnvSet = append(append([]string{}, extra...), "WSLENV="+strings.Join(names, ":"))
		}
		return cmd, nil
	}
	return compose.Command{}, fmt.Errorf("builds through the docker CLI cannot run here: %s", env.Reason)
}

// Scrubber removes sensitive text from output before it is shown or stored.
type Scrubber struct{ values []string }

// NewScrubber hides the given values (those shorter than minLen are skipped,
// they would garble ordinary text).
func NewScrubber(values []string, minLen int) *Scrubber {
	s := &Scrubber{}
	seen := map[string]bool{}
	for _, v := range values {
		if len(v) >= minLen && !seen[v] {
			seen[v] = true
			s.values = append(s.values, v)
		}
	}
	// Longest first so a value containing another is replaced whole.
	for i := range s.values {
		for j := i + 1; j < len(s.values); j++ {
			if len(s.values[j]) > len(s.values[i]) {
				s.values[i], s.values[j] = s.values[j], s.values[i]
			}
		}
	}
	return s
}

// Apply replaces every occurrence of a hidden value. A nil Scrubber is a no-op.
func (s *Scrubber) Apply(line string) string {
	if s == nil {
		return line
	}
	for _, v := range s.values {
		line = strings.ReplaceAll(line, v, "***")
	}
	return line
}

// SecretValues returns the values of secret-looking arguments.
func SecretValues(args []Arg) []string {
	var out []string
	for _, a := range args {
		if IsSecretName(a.Name) {
			out = append(out, a.Value)
		}
	}
	return out
}

// AllValues returns every argument value.
func AllValues(args []Arg) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, a.Value)
	}
	return out
}
