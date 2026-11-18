// Package compose discovers Compose projects from container labels and plans
// safe Compose CLI executions. It is deliberately independent of the Engine API
// transport: the CLI has its own execution environment and filesystem.
package compose

import (
	"docklet/internal/engine"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// projectNameRe is the Compose project naming rule.
	projectNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// ValidateProjectName enforces the Compose naming rule (lowercase letters, digits, - and _).
func ValidateProjectName(name string) error {
	if len(name) > 255 || !projectNameRe.MatchString(name) {
		return fmt.Errorf("invalid Compose project name %q", name)
	}
	return nil
}

// ValidateDistro validates a WSL distribution name.
func ValidateDistro(name string) error {
	if len(name) > 255 || !engine.ValidWSLDistroName(name) {
		return fmt.Errorf("invalid WSL distribution name %q", name)
	}
	return nil
}

// validatePath rejects values that could be mistaken for options or break argv.
func validatePath(kind, p string) error {
	if strings.TrimSpace(p) == "" {
		return fmt.Errorf("%s is empty", kind)
	}
	if strings.HasPrefix(p, "-") {
		return fmt.Errorf("%s %q must not start with '-'", kind, p)
	}
	if strings.ContainsAny(p, "\x00\r\n") {
		return fmt.Errorf("%s contains control characters", kind)
	}
	return nil
}

var errNoVerb = errors.New("unsupported compose verb")

var tokenRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// ValidateServiceName validates a Compose service name used as a CLI argument.
func ValidateServiceName(name string) error {
	if len(name) > 255 || !tokenRe.MatchString(name) {
		return fmt.Errorf("invalid Compose service name %q", name)
	}
	return nil
}

// ValidateProfile validates a Compose profile name used with --profile.
func ValidateProfile(name string) error {
	if len(name) > 255 || !tokenRe.MatchString(name) {
		return fmt.Errorf("invalid Compose profile name %q", name)
	}
	return nil
}

// NormalizeProjectName applies the Compose rule for deriving a project name from a
// directory name: lower-case, drop everything except [a-z0-9_-], trim leading _ and -.
func NormalizeProjectName(dir string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(dir) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.TrimLeft(b.String(), "_-")
}
