package wsldocker

import (
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestBuildPlan_Ubuntu2404WithSystemdFollowsDockersGuide(t *testing.T) {
	p := BuildPlan(ubuntuFacts())
	if !p.Supported || p.State != model.WSLStateNotInstalled || p.DaemonStart != "systemd" {
		t.Fatalf("%+v", p)
	}
	const apt = "env DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=120 "
	want := []string{
		apt + "update",
		apt + "install -y ca-certificates curl",
		"install -m 0755 -d /etc/apt/keyrings",
		"curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc",
		"chmod a+r /etc/apt/keyrings/docker.asc",
		"tee /etc/apt/sources.list.d/docker.sources",
		apt + "update",
		apt + "install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
		"/usr/sbin/usermod -aG docker alice",
		"systemctl enable --now docker",
		"docker version",
		"docker info",
	}
	got := commands(p.Steps)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("steps:\n%s", strings.Join(got, "\n"))
	}
	if len(p.SystemdSteps) != 0 {
		t.Fatalf("systemd already runs, nothing to enable: %v", commands(p.SystemdSteps))
	}
}

func TestBuildPlan_DebianArm64WithoutSystemdUsesServiceAndOffersSystemd(t *testing.T) {
	p := BuildPlan(debianFacts())
	if !p.Supported || p.DaemonStart != "service" {
		t.Fatalf("%+v", p)
	}
	got := commands(p.Steps)
	mustContain(t, got, "/usr/sbin/service docker start")
	mustContain(t, got, "https://download.docker.com/linux/debian/gpg")
	mustNotContain(t, got, "systemctl")
	repo := stepWith(t, p.Steps, "tee")
	if in := repo.Cmd.Stdin; !strings.Contains(in, "Suites: bookworm\n") || !strings.Contains(in, "Architectures: arm64\n") {
		t.Fatalf("repo entry: %s", in)
	}
	mustContain(t, p.Notes, "systemd is not running")
	if len(p.WithOptions(Options{})) != len(p.Steps) {
		t.Fatal("systemd steps must be off by default")
	}
	with := commands(p.WithOptions(Options{EnableSystemd: true}))
	mustContain(t, with, "edit /etc/wsl.conf")
	mustContain(t, with, "wsl.exe --terminate Debian")
	if with[len(with)-1] != "docker version" {
		t.Fatalf("the plan must end by verifying Docker: %v", with)
	}
}

func TestBuildPlan_UnsupportedDistrosGetReasonAndDocsLink(t *testing.T) {
	for text, docs := range map[string]string{
		fedoraOSRelease: "https://docs.docker.com/engine/install/fedora/",
		"ID=alpine\n":   "https://docs.docker.com/engine/install/",
		"ID=linuxmint\nID_LIKE=\"ubuntu debian\"\nUBUNTU_CODENAME=noble\n": "https://docs.docker.com/engine/install/ubuntu/",
	} {
		p := BuildPlan(Facts{Distro: "X", OS: ParseOSRelease(text), Arch: "amd64"})
		if p.Supported || len(p.Steps) != 0 || p.Reason == "" || p.DocsURL != docs {
			t.Errorf("%q: %+v", text, p)
		}
	}
	if p := BuildPlan(Facts{Distro: "D", OS: ParseOSRelease("ID=debian\n"), Arch: "amd64"}); p.Supported {
		t.Fatal("a missing codename cannot pick a repository")
	}
}

func TestBuildPlan_ConflictingPackagesAreListedAndRemovedOnlyWhenPresent(t *testing.T) {
	f := ubuntuFacts()
	clean := BuildPlan(f)
	if len(clean.Remove) != 0 || clean.Preflight(false) != nil {
		t.Fatalf("no conflicts: %+v", clean.Remove)
	}
	mustNotContain(t, commands(clean.Steps), " remove ")

	f.Installed = []string{"docker.io", "containerd", "unrelated-not-listed"}
	p := BuildPlan(f)
	if strings.Join(p.Remove, ",") != "docker.io,containerd" {
		t.Fatalf("remove: %v", p.Remove)
	}
	if first := commands(p.Steps)[0]; !strings.HasSuffix(first, "remove -y docker.io containerd") {
		t.Fatalf("removal must come first and name exactly the installed packages: %s", first)
	}
	if err := p.Preflight(false); err == nil || !strings.Contains(err.Error(), "docker.io, containerd") {
		t.Fatalf("removal must be confirmed: %v", err)
	}
	if err := p.Preflight(true); err != nil {
		t.Fatal(err)
	}
}

func TestBuildPlan_ExistingEngineDockerDesktopAndStoppedDaemonAreNotInstalledOver(t *testing.T) {
	running := ubuntuFacts()
	running.Installed, running.ServerVersion = []string{"docker-ce"}, "27.1.1"
	stopped := ubuntuFacts()
	stopped.Installed = []string{"docker-ce"}
	desktop := ubuntuFacts()
	desktop.CLIPath, desktop.DesktopStub = "/usr/bin/docker", true
	for name, c := range map[string]struct {
		f     Facts
		state model.WSLInstallState
	}{
		"running": {running, model.WSLStateRunning}, "stopped daemon": {stopped, model.WSLStateDaemonStopped},
		"docker desktop": {desktop, model.WSLStateDockerDesktop},
	} {
		p := BuildPlan(c.f)
		if p.State != c.state || p.Supported || len(p.Steps) != 0 || p.Preflight(true) == nil || p.Reason == "" {
			t.Errorf("%s: %+v", name, p)
		}
	}
	if !strings.Contains(BuildPlan(desktop).Reason, "Docker Desktop") {
		t.Fatal("the Docker Desktop case must be explained")
	}
}

func TestBuildPlan_VariablePartsStaySeparateArgvElements(t *testing.T) {
	f := ubuntuFacts()
	f.User = "al$(touch x)ice" // not a valid unix name: must never reach a command
	for _, s := range BuildPlan(f).Steps {
		for _, a := range s.Cmd.Argv {
			if strings.Contains(a, "$(") {
				t.Fatalf("unsafe value reached argv: %v", s.Cmd.Argv)
			}
		}
	}
	f.User = "alice"
	steps := BuildPlan(f).Steps
	if s := stepWith(t, steps, "/usr/sbin/usermod"); strings.Join(s.Cmd.Argv, "|") != "/usr/sbin/usermod|-aG|docker|alice" {
		t.Fatalf("user must be its own argv element: %v", s.Cmd.Argv)
	}
	for _, s := range steps {
		if a := s.Cmd.Argv[0]; a == "sh" || a == "bash" {
			t.Fatalf("the install plan must not use a shell: %v", s.Cmd.Argv)
		}
	}
}

func TestBuildPlan_RootDefaultUserNeedsNoGroupStep(t *testing.T) {
	f := debianFacts()
	f.User = "root"
	p := BuildPlan(f)
	mustNotContain(t, commands(p.Steps), "usermod")
	if p.DTO().User != "" {
		t.Fatal("root must not be reported as a user to add")
	}
}

func TestRepoSources_UsesDockersDeb822Format(t *testing.T) {
	want := "Types: deb\nURIs: https://download.docker.com/linux/ubuntu\nSuites: noble\nComponents: stable\nArchitectures: amd64\nSigned-By: /etc/apt/keyrings/docker.asc\n"
	if got := RepoSources("ubuntu", "noble", "amd64"); got != want {
		t.Fatalf("got %q", got)
	}
}

func stepWith(t *testing.T, steps []Step, argv0 string) Step {
	t.Helper()
	for _, s := range steps {
		if len(s.Cmd.Argv) > 0 && s.Cmd.Argv[0] == argv0 {
			return s
		}
	}
	t.Fatalf("no %s step", argv0)
	return Step{}
}

func mustNotContain(t *testing.T, haystack []string, bad string) {
	t.Helper()
	for _, h := range haystack {
		if strings.Contains(h, bad) {
			t.Fatalf("unexpected %q in %q", bad, h)
		}
	}
}
