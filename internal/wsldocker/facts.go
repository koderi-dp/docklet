package wsldocker

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"docklet/internal/model"
)

// conflictingPackages are the distro packages Docker's guide removes before installing docker-ce.
var conflictingPackages = []string{
	"docker.io", "docker-compose", "docker-compose-v2", "docker-doc", "docker-buildx", "podman-docker", "containerd", "runc",
}

const dockerCE = "docker-ce"

// factsScript is constant: nothing from the user is ever interpolated into it.
// /proc/1/comm is read instead of `ps` because minimal images do not ship procps.
const factsScript = `
echo '@@os'; cat /etc/os-release 2>/dev/null
echo '@@arch'; dpkg --print-architecture 2>/dev/null || uname -m
echo '@@init'; cat /proc/1/comm 2>/dev/null
echo '@@user'; id -un 2>/dev/null
echo '@@pkgs'
for p in docker-ce docker.io docker-compose docker-compose-v2 docker-doc docker-buildx podman-docker containerd runc; do
  dpkg-query -W -f='${Status}' "$p" 2>/dev/null | grep -q 'install ok installed' && echo "$p"
done
echo '@@cli'
# Only a docker inside the distribution counts: Windows programs on PATH (/mnt/...) are not an engine here.
bin=
oldifs=$IFS; IFS=:
for d in $PATH; do
  case "$d" in /mnt/*) continue ;; esac
  if [ -x "$d/docker" ]; then bin="$d/docker"; break; fi
done
IFS=$oldifs
if [ -n "$bin" ]; then
  echo "$bin"; readlink -f "$bin"
  head -c 4000 "$bin" 2>/dev/null | grep -qai 'docker desktop' && echo desktop-stub
fi
echo '@@dockerd'; command -v dockerd
echo '@@server'; [ -n "$bin" ] && "$bin" version --format '{{.Server.Version}}' 2>/dev/null
echo '@@end'
`

// Facts is what was found inside one running distribution.
type Facts struct {
	Distro        string
	OS            OSRelease
	Arch          string
	Systemd       bool // systemd is PID 1
	User          string
	Installed     []string // watched packages that are installed (includes docker-ce when present)
	CLIPath       string   // path of `docker`, empty when absent
	DesktopStub   bool     // docker is Docker Desktop's WSL integration proxy
	Dockerd       bool
	ServerVersion string // set when the daemon answers
}

var serverVersionRe = regexp.MustCompile(`^[0-9]+\.[0-9A-Za-z.+~-]*$`)

// Detect inspects a running distribution with one read-only command. It never
// changes anything and never starts the distribution on its own (callers check).
func Detect(ctx context.Context, ex Exec, distro string) (Facts, error) {
	var lines []string
	err := ex.Run(ctx, distro, Cmd{Argv: []string{"sh", "-c", factsScript}}, func(l string) { lines = append(lines, l) })
	if err != nil {
		return Facts{}, fmt.Errorf("inspecting %s: %w (%s)", distro, err, tail(lines, 3))
	}
	return parseFacts(distro, lines), nil
}

func tail(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

func parseFacts(distro string, lines []string) Facts {
	sections := map[string][]string{}
	cur := ""
	for _, l := range lines {
		if strings.HasPrefix(l, "@@") {
			cur = strings.TrimPrefix(l, "@@")
			continue
		}
		sections[cur] = append(sections[cur], l)
	}
	first := func(k string) string {
		for _, l := range sections[k] {
			if t := strings.TrimSpace(l); t != "" {
				return t
			}
		}
		return ""
	}
	f := Facts{
		Distro:  distro,
		OS:      ParseOSRelease(strings.Join(sections["os"], "\n")),
		Arch:    first("arch"),
		Systemd: first("init") == "systemd",
		User:    first("user"),
		Dockerd: first("dockerd") != "",
	}
	for _, l := range sections["pkgs"] {
		if t := strings.TrimSpace(l); t != "" {
			f.Installed = append(f.Installed, t)
		}
	}
	if cli := sections["cli"]; len(cli) > 0 {
		f.CLIPath = strings.TrimSpace(cli[0])
		for _, l := range cli {
			if l = strings.TrimSpace(l); l == "desktop-stub" || strings.Contains(l, "docker-desktop") {
				f.DesktopStub = true
			}
		}
	}
	if v := first("server"); serverVersionRe.MatchString(v) {
		f.ServerVersion = v
	}
	return f
}

func (f Facts) has(pkg string) bool {
	for _, p := range f.Installed {
		if p == pkg {
			return true
		}
	}
	return false
}

// Conflicts lists the installed packages Docker's guide removes first, in the guide's order.
func (f Facts) Conflicts() []string {
	var out []string
	for _, p := range conflictingPackages {
		if f.has(p) {
			out = append(out, p)
		}
	}
	return out
}

// State classifies Docker's presence; it decides what the UI may offer.
func (f Facts) State() model.WSLInstallState {
	switch {
	case f.ServerVersion != "":
		return model.WSLStateRunning
	case f.DesktopStub:
		return model.WSLStateDockerDesktop
	case f.has(dockerCE) || f.Dockerd:
		return model.WSLStateDaemonStopped
	}
	return model.WSLStateNotInstalled
}
