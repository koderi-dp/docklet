package wsldocker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"docklet/internal/model"
)

func probeOutput(parts ...string) []string {
	return strings.Split(strings.Join(parts, "\n"), "\n")
}

var probeKey = Display(Cmd{Argv: []string{"sh", "-c", factsScript}})

func TestParseFacts_ReadsEveryProbeSection(t *testing.T) {
	out := probeOutput("@@os", ubuntuOSRelease, "@@arch", "arm64", "@@init", "systemd", "@@user", "alice",
		"@@pkgs", "docker.io", "runc", "@@cli", "/usr/bin/docker", "/usr/bin/docker.io", "@@dockerd", "/usr/sbin/dockerd", "@@server", "", "@@end")
	f := parseFacts("U", out)
	if f.OS.ID != "ubuntu" || f.Arch != "arm64" || !f.Systemd || f.User != "alice" || !f.Dockerd || f.CLIPath != "/usr/bin/docker" {
		t.Fatalf("%+v", f)
	}
	if strings.Join(f.Conflicts(), ",") != "docker.io,runc" || f.State() != model.WSLStateDaemonStopped {
		t.Fatalf("conflicts/state: %v %v", f.Conflicts(), f.State())
	}
}

func TestParseFacts_DetectsDockerDesktopProxyAndIgnoresJunkServerOutput(t *testing.T) {
	out := probeOutput("@@os", debianOSRelease, "@@arch", "amd64", "@@init", "init", "@@user", "bob", "@@pkgs",
		"@@cli", "/usr/bin/docker", "/mnt/wsl/docker-desktop/cli-tools/usr/bin/docker", "@@dockerd", "@@server",
		"The command 'docker' could not be found in this WSL 2 distro.", "@@end")
	f := parseFacts("D", out)
	if f.State() != model.WSLStateDockerDesktop || f.ServerVersion != "" || f.Systemd || f.Dockerd || len(f.Installed) != 0 {
		t.Fatalf("%+v state=%v", f, f.State())
	}
}

func TestParseFacts_MissingDockerIsNotInstalledAndAnswerFromDaemonIsRunning(t *testing.T) {
	f := parseFacts("D", probeOutput("@@os", debianOSRelease, "@@arch", "amd64", "@@cli", "@@pkgs", "@@dockerd", "@@server", "@@end"))
	if f.State() != model.WSLStateNotInstalled || f.CLIPath != "" {
		t.Fatalf("%+v", f)
	}
	if parseFacts("D", probeOutput("@@server", "27.3.1", "@@end")).State() != model.WSLStateRunning {
		t.Fatal("a daemon answering with a version is running")
	}
}

func TestStart_UsesTheInitSystemOfTheDistributionAndWaitsForTheDaemon(t *testing.T) {
	for init, want := range map[string]string{"systemd": "systemctl start docker", "init": "/usr/sbin/service docker start"} {
		stopped := probeOutput("@@os", ubuntuOSRelease, "@@init", init, "@@pkgs", "docker-ce", "@@dockerd", "/usr/sbin/dockerd", "@@server", "", "@@end")
		answering := probeOutput("@@os", ubuntuOSRelease, "@@init", init, "@@pkgs", "docker-ce", "@@dockerd", "/usr/sbin/dockerd", "@@server", "27.3.1", "@@end")
		ex := &fakeExec{output: map[string][]string{probeKey: stopped}}
		ex.onCall = func(c Cmd) {
			if c.Argv[0] != "sh" { // the start command ran: the daemon answers from now on
				ex.output[probeKey] = answering
			}
		}
		f, err := Start(context.Background(), ex, "U")
		if err != nil || f.ServerVersion != "27.3.1" {
			t.Fatalf("%s: %v %+v", init, err, f)
		}
		mustContain(t, ex.displays(), want)
	}
}

func TestStart_RefusesDockerDesktopAndDistributionsWithoutAnEngine(t *testing.T) {
	cases := map[string][]string{
		"none":    probeOutput("@@os", debianOSRelease, "@@pkgs", "@@server", "@@end"),
		"desktop": probeOutput("@@cli", "/usr/bin/docker", "/mnt/wsl/docker-desktop/x/docker", "@@server", "@@end"),
	}
	for name, out := range cases {
		ex := &fakeExec{output: map[string][]string{probeKey: out}}
		if _, err := Start(context.Background(), ex, "D"); err == nil || len(ex.calls) != 1 {
			t.Errorf("%s: nothing may be started (%v, %v)", name, err, ex.displays())
		}
	}
}

func TestDetect_SurfacesProbeFailure(t *testing.T) {
	ex := &fakeExec{fail: map[string]error{probeKey: errors.New("exit status 1")}}
	if _, err := Detect(context.Background(), ex, "D"); err == nil {
		t.Fatal("detection failure must surface")
	}
}
