package wsldocker

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"docklet/internal/model"
)

// Step is one unit of the install plan. It is data: nothing runs while it is built.
type Step struct {
	Title   string
	Why     string
	Cmd     Cmd
	Edit    *FileEdit     // when set, the step edits a file in the distribution instead of running Cmd
	Timeout time.Duration // limit for one attempt
	Wait    time.Duration // when > 0, a failing attempt is retried until this much time has passed
}

// FileEdit rewrites a file in the distribution: read it (missing counts as empty),
// transform the text, write it back. Preview is what the user is shown.
type FileEdit struct {
	Path      string
	Transform func(old string) string
	Preview   string
}

// Plan is the inspection result plus the ordered steps that would install Docker Engine.
type Plan struct {
	Facts       Facts
	State       model.WSLInstallState
	Supported   bool
	Reason      string
	DocsURL     string
	Steps       []Step
	Remove      []string // conflicting packages that will be removed
	Notes       []string
	DaemonStart string // "systemd" or "service"
	// SystemdSteps are the extra steps appended when the user opts into enabling systemd.
	SystemdSteps []Step
}

// Options are the user's choices when installing.
type Options struct {
	EnableSystemd bool
}

var unixUser = regexp.MustCompile(`^[a-z_][a-z0-9_.-]{0,31}$`)

// wsl.exe --exec looks commands up in a PATH without the sbin directories, so
// the two commands that live there are given by absolute path.
const (
	usermodBin = "/usr/sbin/usermod"
	serviceBin = "/usr/sbin/service"
)

const (
	keyringPath = "/etc/apt/keyrings/docker.asc"
	sourcesPath = "/etc/apt/sources.list.d/docker.sources"
	aptTimeout  = 10 * time.Minute
	shortTime   = 2 * time.Minute
)

// BuildPlan turns detected facts into the install plan. It is pure.
func BuildPlan(f Facts) Plan {
	p := Plan{Facts: f, State: f.State(), DocsURL: f.OS.DocsURL(), Remove: f.Conflicts()}
	if p.Remove == nil {
		p.Remove = []string{}
	}
	switch p.State {
	case model.WSLStateRunning:
		p.Reason = "Docker Engine " + f.ServerVersion + " is already running in this distribution."
		return p
	case model.WSLStateDockerDesktop:
		p.Reason = "This distribution uses Docker Desktop's WSL integration: the docker command here is a proxy to Docker Desktop. " +
			"Start Docker Desktop to use it, or connect with the Docker Desktop named pipe connection instead. Docklet will not install an engine over it."
		return p
	case model.WSLStateDaemonStopped:
		p.Reason = "Docker Engine is installed in this distribution but its daemon is not running."
		return p
	}
	if reason := unsupportedReason(f); reason != "" {
		p.Reason = reason
		return p
	}
	p.Supported = true
	p.DaemonStart = "service"
	if f.Systemd {
		p.DaemonStart = "systemd"
	}
	p.Steps = installSteps(f, p.Remove)
	p.SystemdSteps = systemdSteps(f)
	p.Notes = notes(f)
	return p
}

// Preflight reports why the plan must not be run as it is, or nil when it may.
func (p Plan) Preflight(removeConflicting bool) error {
	switch {
	case p.State == model.WSLStateRunning, p.State == model.WSLStateDaemonStopped:
		return errors.New("Docker Engine is already installed in this distribution; start it instead of installing again")
	case !p.Supported:
		return errors.New(p.Reason)
	case len(p.Remove) > 0 && !removeConflicting:
		return errors.New("these installed packages conflict with Docker Engine and must be removed first: " + strings.Join(p.Remove, ", ") + ". Confirm their removal to continue")
	}
	return nil
}

// WithOptions returns the steps to run for the user's choices.
func (p Plan) WithOptions(o Options) []Step {
	steps := append([]Step(nil), p.Steps...)
	if o.EnableSystemd && !p.Facts.Systemd {
		steps = append(steps, p.SystemdSteps...)
	}
	return steps
}

func unsupportedReason(f Facts) string {
	name := f.OS.PrettyName
	if name == "" {
		name = f.Distro
	}
	switch {
	case f.OS.ID != "ubuntu" && f.OS.ID != "debian":
		return "Automatic installation supports Ubuntu and Debian. " + name + " needs Docker's own install guide."
	case f.OS.Codename() == "":
		return "The release codename of " + name + " could not be read from /etc/os-release, so the right Docker repository cannot be chosen."
	case f.Arch == "":
		return "The CPU architecture of " + name + " could not be detected."
	}
	return ""
}

func apt(args ...string) []string {
	return append([]string{"env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "-o", "DPkg::Lock::Timeout=120"}, args...)
}

func root(argv ...string) Cmd { return Cmd{Argv: argv, Root: true} }

func installSteps(f Facts, remove []string) []Step {
	var s []Step
	if len(remove) > 0 {
		s = append(s, Step{
			Title: "Remove conflicting packages",
			Why:   "Distribution packages that conflict with Docker's own (" + strings.Join(remove, ", ") + ") would break docker-ce, as Docker's install guide says.",
			Cmd:   root(apt(append([]string{"remove", "-y"}, remove...)...)...), Timeout: aptTimeout,
		})
	}
	s = append(s,
		Step{Title: "Refresh package lists", Why: "Gets current package information before installing.",
			Cmd: root(apt("update")...), Timeout: aptTimeout},
		Step{Title: "Install ca-certificates and curl", Why: "Needed to download Docker's signing key over HTTPS.",
			Cmd: root(apt("install", "-y", "ca-certificates", "curl")...), Timeout: aptTimeout},
		Step{Title: "Create the apt keyring directory", Why: "Docker's key is stored in /etc/apt/keyrings, the location apt recommends.",
			Cmd: root("install", "-m", "0755", "-d", "/etc/apt/keyrings"), Timeout: shortTime},
		Step{Title: "Download Docker's signing key", Why: "Lets apt verify packages from Docker's repository.",
			Cmd: root("curl", "-fsSL", "https://download.docker.com/linux/"+f.OS.ID+"/gpg", "-o", keyringPath), Timeout: shortTime},
		Step{Title: "Make the key readable", Why: "apt reads the key as an unprivileged user.",
			Cmd: root("chmod", "a+r", keyringPath), Timeout: shortTime},
		repoStep(f),
		Step{Title: "Refresh package lists from Docker's repository", Why: "Makes Docker's packages available to install.",
			Cmd: root(apt("update")...), Timeout: aptTimeout},
		Step{Title: "Install Docker Engine", Why: "Docker CE, the CLI, containerd, and the Buildx and Compose plugins from Docker's official repository.",
			Cmd:     root(apt("install", "-y", "docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin")...),
			Timeout: 20 * time.Minute},
	)
	if f.User != "" && f.User != "root" && unixUser.MatchString(f.User) {
		s = append(s, Step{Title: "Add " + f.User + " to the docker group",
			Why: "Lets " + f.User + " use Docker without sudo. It applies the next time the distribution restarts.",
			Cmd: root(usermodBin, "-aG", "docker", f.User), Timeout: shortTime})
	}
	s = append(s, startStep(f))
	s = append(s, verifySteps()...)
	return s
}

// RepoSources is the deb822 apt source for Docker's repository (the format Docker's guide uses).
func RepoSources(distroID, codename, arch string) string {
	return "Types: deb\n" +
		"URIs: https://download.docker.com/linux/" + distroID + "\n" +
		"Suites: " + codename + "\n" +
		"Components: stable\n" +
		"Architectures: " + arch + "\n" +
		"Signed-By: " + keyringPath + "\n"
}

func repoStep(f Facts) Step {
	return Step{
		Title: "Add Docker's apt repository",
		Why:   "Points apt at Docker's repository for " + f.OS.Codename() + " (" + f.Arch + "). The file content is written by Docklet through tee.",
		Cmd:   Cmd{Argv: []string{"tee", sourcesPath}, Root: true, Stdin: RepoSources(f.OS.ID, f.OS.Codename(), f.Arch)}, Timeout: shortTime,
	}
}

func startStep(f Facts) Step {
	if f.Systemd {
		return Step{Title: "Enable and start the Docker service", Why: "systemd is running here, so it starts Docker now and at every boot.",
			Cmd: root("systemctl", "enable", "--now", "docker"), Timeout: shortTime}
	}
	return Step{Title: "Start the Docker service", Why: "systemd is not running in this distribution, so the daemon is started with the init script.",
		Cmd: root(serviceBin, "docker", "start"), Timeout: shortTime}
}

// StartCommand is the single command that starts an installed daemon.
func StartCommand(systemd bool) Cmd {
	if systemd {
		return root("systemctl", "start", "docker")
	}
	return root(serviceBin, "docker", "start")
}

func verifySteps() []Step {
	return []Step{
		{Title: "Check Docker's version", Why: "Confirms the client and the daemon answer. Nothing is pulled.",
			Cmd: root("docker", "version"), Timeout: 30 * time.Second, Wait: 20 * time.Second},
		{Title: "Check Docker's info", Why: "Shows the daemon's configuration as a final check.",
			Cmd: root("docker", "info"), Timeout: 30 * time.Second},
	}
}

func systemdSteps(f Facts) []Step {
	if f.Systemd {
		return nil
	}
	return []Step{
		{Title: "Enable systemd in /etc/wsl.conf", Why: "Adds systemd=true to the [boot] section and keeps the rest of the file as it is.",
			Edit:    &FileEdit{Path: "/etc/wsl.conf", Transform: EnableSystemd, Preview: "[boot]\nsystemd=true\n"},
			Timeout: shortTime},
		{Title: "Restart the distribution", Why: "wsl.conf is only read at start. This stops everything running in " + f.Distro + ".",
			Cmd: Cmd{Argv: []string{"--terminate", f.Distro}, Host: true}, Timeout: time.Minute},
		{Title: "Start Docker under systemd", Why: "Starting a command boots the distribution again, now with systemd.",
			Cmd: root("systemctl", "enable", "--now", "docker"), Timeout: 2 * time.Minute, Wait: 60 * time.Second},
		{Title: "Check Docker's version again", Why: "Confirms Docker came back after the restart.",
			Cmd: root("docker", "version"), Timeout: 30 * time.Second, Wait: 20 * time.Second},
	}
}

func notes(f Facts) []string {
	var n []string
	if !f.Systemd {
		n = append(n, "systemd is not running in this distribution, so Docker will not start by itself after WSL restarts. Use Start Docker Engine in Docklet after a restart, or enable systemd.")
	}
	if f.User == "root" {
		n = append(n, "The default user is root, which can already use Docker: no group change is needed.")
	} else if f.User != "" && !unixUser.MatchString(f.User) {
		n = append(n, "The default user name is unusual, so it was not added to the docker group automatically.")
	}
	return n
}
