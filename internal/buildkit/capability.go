package buildkit

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"docklet/internal/model"
)

// Version is a parsed dotted version.
type Version struct{ Major, Minor, Patch int }

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// Short is major.minor, used in the one-line summary.
func (v Version) Short() string { return fmt.Sprintf("%d.%d", v.Major, v.Minor) }

// AtLeast compares against major.minor.
func (v Version) AtLeast(major, minor int) bool {
	return v.Major > major || (v.Major == major && v.Minor >= minor)
}

var (
	buildxVersionRe = regexp.MustCompile(`buildx\s+v?(\d+)\.(\d+)(?:\.(\d+))?`)
	dockerVersionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)(?:\.(\d+))?`)
)

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// ParseBuildxVersion reads `docker buildx version` output
// ("github.com/docker/buildx v0.19.2 <hash>").
func ParseBuildxVersion(out string) (Version, bool) {
	m := buildxVersionRe.FindStringSubmatch(out)
	if m == nil {
		return Version{}, false
	}
	return Version{atoi(m[1]), atoi(m[2]), atoi(m[3])}, true
}

// ParseDockerVersion reads the first line of `docker version --format {{.Client.Version}}`.
func ParseDockerVersion(out string) (Version, bool) {
	first, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	m := dockerVersionRe.FindStringSubmatch(strings.TrimSpace(first))
	if m == nil {
		return Version{}, false
	}
	return Version{atoi(m[1]), atoi(m[2]), atoi(m[3])}, true
}

// Probe is what was found in the CLI environment.
type Probe struct {
	Buildx    Version
	HasBuildx bool
	Docker    Version
	HasDocker bool
}

// Decide chooses the BuildKit method for what was found. Summary and Reason
// are the texts shown to the user.
func Decide(p Probe) model.BuildCapability {
	switch {
	case p.HasBuildx:
		raw := p.Buildx.AtLeast(0, 14)
		c := model.BuildCapability{Available: true, Method: MethodBuildx, Version: p.Buildx.String(), RawJSON: raw,
			Summary: "BuildKit via docker buildx " + p.Buildx.Short()}
		if !raw {
			c.Summary += " (plain progress: this buildx is older than 0.14)"
		}
		return c
	case p.HasDocker && p.Docker.AtLeast(18, 9) && !p.Docker.AtLeast(23, 0):
		return model.BuildCapability{Available: true, Method: MethodDockerBuild, Version: p.Docker.String(),
			Summary: "BuildKit via docker build " + p.Docker.Short() + " (plain progress)"}
	case p.HasDocker && p.Docker.AtLeast(23, 0):
		return unavailable(fmt.Sprintf("docker %s is installed, but its buildx plugin is missing", p.Docker.Short()))
	case p.HasDocker:
		return unavailable(fmt.Sprintf("docker %s is too old for BuildKit", p.Docker.Short()))
	}
	return unavailable("the docker CLI was not found for this connection")
}

func unavailable(reason string) model.BuildCapability {
	return model.BuildCapability{Reason: reason, Summary: "Classic builder: " + reason}
}
