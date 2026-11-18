package compose

import (
	"fmt"
	"strings"
)

// Verb is a supported Compose CLI verb.
type Verb string

const (
	VerbUp      Verb = "up"
	VerbDown    Verb = "down"
	VerbStart   Verb = "start"
	VerbStop    Verb = "stop"
	VerbRestart Verb = "restart"
	VerbPull    Verb = "pull"
	VerbBuild   Verb = "build"
	VerbConfig  Verb = "config"
	VerbWatch   Verb = "watch"
)

// Request is everything needed to build one Compose invocation. Paths come from
// container labels and are never rewritten.
type Request struct {
	Project     string
	ConfigFiles []string
	EnvFiles    []string
	WorkingDir  string
	Verb        Verb

	Build         bool // up only
	RemoveOrphans bool // up, down
	Volumes       bool // down only; deletes named volumes
	NoUp          bool // watch only: do not build and start services first

	// Profiles are passed as global --profile flags (before the verb).
	Profiles []string
	// Services restricts start/stop/restart/pull/build/up to these services.
	Services []string
	// ConfigMode selects the `config` flavour: "" (plain), "quiet" (-q), "json"
	// (--format json) or "services" (--services). Only valid for VerbConfig.
	ConfigMode string
}

// Command is a process invocation (argv, no shell).
type Command struct {
	Name     string
	Args     []string
	EnvSet   []string
	EnvUnset []string
}

// Argv returns the arguments after `docker`: compose -p <project> ... <verb> [flags].
func Argv(r Request) ([]string, error) {
	// Only the name-detection run (`config --format json`) may omit the project name.
	if r.Project != "" || r.Verb != VerbConfig || r.ConfigMode != "json" {
		if err := ValidateProjectName(r.Project); err != nil {
			return nil, err
		}
	}
	if len(r.ConfigFiles) == 0 {
		return nil, fmt.Errorf("no Compose config files known for project %q", r.Project)
	}
	args := []string{"compose"}
	if r.Project != "" {
		args = append(args, "-p", r.Project)
	}
	for _, pr := range r.Profiles {
		if err := ValidateProfile(pr); err != nil {
			return nil, err
		}
		args = append(args, "--profile", pr)
	}
	if r.WorkingDir != "" {
		if err := validatePath("working directory", r.WorkingDir); err != nil {
			return nil, err
		}
		args = append(args, "--project-directory", r.WorkingDir)
	}
	for _, f := range r.ConfigFiles {
		if err := validatePath("config file", f); err != nil {
			return nil, err
		}
		args = append(args, "-f", f)
	}
	for _, f := range r.EnvFiles {
		if err := validatePath("env file", f); err != nil {
			return nil, err
		}
		args = append(args, "--env-file", f)
	}
	if r.Build && r.Verb != VerbUp {
		return nil, fmt.Errorf("--build is only valid for up")
	}
	if r.Volumes && r.Verb != VerbDown {
		return nil, fmt.Errorf("--volumes is only valid for down")
	}
	if r.NoUp && r.Verb != VerbWatch {
		return nil, fmt.Errorf("--no-up is only valid for watch")
	}
	if r.RemoveOrphans && r.Verb != VerbUp && r.Verb != VerbDown {
		return nil, fmt.Errorf("--remove-orphans is only valid for up and down")
	}
	if r.ConfigMode != "" && r.Verb != VerbConfig {
		return nil, fmt.Errorf("config mode is only valid for config")
	}
	if len(r.Services) > 0 && (r.Verb == VerbDown || r.Verb == VerbConfig) {
		return nil, fmt.Errorf("services can not be selected for %s", r.Verb)
	}
	for _, sv := range r.Services {
		if err := ValidateServiceName(sv); err != nil {
			return nil, err
		}
	}
	switch r.Verb {
	case VerbUp:
		args = append(args, "up", "-d")
		if r.Build {
			args = append(args, "--build")
		}
		if r.RemoveOrphans {
			args = append(args, "--remove-orphans")
		}
	case VerbDown:
		args = append(args, "down")
		if r.Volumes {
			args = append(args, "--volumes")
		}
		if r.RemoveOrphans {
			args = append(args, "--remove-orphans")
		}
	case VerbPull:
		// Services with both build and image would otherwise fail with "pull access denied".
		args = append(args, "pull", "--ignore-buildable")
	case VerbWatch:
		args = append(args, "watch")
		if r.NoUp {
			args = append(args, "--no-up")
		}
	case VerbStart, VerbStop, VerbRestart, VerbBuild:
		args = append(args, string(r.Verb))
	case VerbConfig:
		args = append(args, "config")
		switch r.ConfigMode {
		case "":
		case "quiet":
			args = append(args, "-q")
		case "json":
			args = append(args, "--format", "json")
		case "services":
			args = append(args, "--services")
		default:
			return nil, fmt.Errorf("unknown config mode %q", r.ConfigMode)
		}
	default:
		return nil, fmt.Errorf("%w %q", errNoVerb, r.Verb)
	}
	return append(args, r.Services...), nil
}

// BuildCommand wraps Argv for the execution environment.
func BuildCommand(env ExecEnv, r Request) (Command, error) {
	args, err := Argv(r)
	if err != nil {
		return Command{}, err
	}
	switch env.Kind {
	case KindLocal:
		return Command{Name: "docker", Args: args, EnvSet: env.EnvSet, EnvUnset: env.EnvUnset}, nil
	case KindWSL:
		if err := ValidateDistro(env.Distro); err != nil {
			return Command{}, err
		}
		for _, f := range append(append([]string{r.WorkingDir}, r.ConfigFiles...), r.EnvFiles...) {
			if f != "" && !strings.HasPrefix(f, "/") {
				return Command{}, fmt.Errorf("path %q is not an absolute path inside the WSL distribution", f)
			}
		}
		w := []string{"-d", env.Distro}
		if r.WorkingDir != "" {
			w = append(w, "--cd", r.WorkingDir)
		}
		w = append(w, "--exec", "docker")
		return Command{Name: "wsl.exe", Args: append(w, args...)}, nil
	}
	return Command{}, fmt.Errorf("compose cannot run here: %s", env.Reason)
}
