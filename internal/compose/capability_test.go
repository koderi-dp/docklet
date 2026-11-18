package compose

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"docklet/internal/model"
)

type fakeFS map[string]bool

func (f fakeFS) FileExists(_ context.Context, p string) bool { return f[p] }

func (f fakeFS) DirExists(_ context.Context, p string) bool { return f[p] }

func TestAssess_ExplainsWhyAProjectCannotRun(t *testing.T) {
	ctx := context.Background()
	local := SelectEnv(model.ConnectionProfile{Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/docker_engine"})
	cliOK := func() CLIStatus { return CLIStatus{OK: true} }
	cliBad := func() CLIStatus { return CLIStatus{Detail: "docker compose is not installed"} }

	// Linux paths on a Windows host: not operable, reason says why.
	c := Assess(ctx, AssessInput{Env: local, HostOS: "windows", ConfigFiles: []string{"/home/u/app/compose.yml"}, WorkingDir: "/home/u/app", FS: fakeFS{}, CLI: cliOK})
	if c.CanRun || !strings.Contains(c.Reason, "do not exist on this machine") || c.ExecutionLocation != "This machine" {
		t.Errorf("foreign path: %+v", c)
	}
	// Files exist, CLI ok.
	c = Assess(ctx, AssessInput{Env: local, HostOS: "windows", ConfigFiles: []string{`C:\a\c.yml`}, WorkingDir: `C:\a`, FS: fakeFS{`C:\a\c.yml`: true, `C:\a`: true}, CLI: cliOK})
	if !c.CanRun || c.Reason != "" {
		t.Errorf("should run: %+v", c)
	}
	// CLI missing.
	c = Assess(ctx, AssessInput{Env: local, HostOS: "windows", ConfigFiles: []string{`C:\a\c.yml`}, FS: fakeFS{`C:\a\c.yml`: true}, CLI: cliBad})
	if c.CanRun || c.Reason != "docker compose is not installed" {
		t.Errorf("cli: %+v", c)
	}
	// Missing env file blocks.
	c = Assess(ctx, AssessInput{Env: local, HostOS: "linux", ConfigFiles: []string{"/a/c.yml"}, EnvFiles: []string{"/a/.env"}, FS: fakeFS{"/a/c.yml": true}, CLI: cliOK})
	if c.CanRun || !strings.Contains(c.Reason, "/a/.env") {
		t.Errorf("env file: %+v", c)
	}
	// No config label.
	if c = Assess(ctx, AssessInput{Env: local, HostOS: "linux", FS: fakeFS{}, CLI: cliOK}); c.CanRun || c.Reason == "" {
		t.Errorf("no files: %+v", c)
	}
	// Missing working dir.
	c = Assess(ctx, AssessInput{Env: local, HostOS: "linux", ConfigFiles: []string{"/a/c.yml"}, WorkingDir: "/a", FS: fakeFS{"/a/c.yml": true}, CLI: cliOK})
	if c.CanRun || !strings.Contains(c.Reason, "working directory") {
		t.Errorf("workdir: %+v", c)
	}
	// WSL location and missing file wording.
	wsl := SelectEnv(model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "Ubuntu"})
	c = Assess(ctx, AssessInput{Env: wsl, HostOS: "windows", ConfigFiles: []string{"/home/u/c.yml"}, FS: fakeFS{}, CLI: cliOK})
	if c.CanRun || c.ExecutionLocation != "WSL: Ubuntu" || !strings.Contains(c.Reason, "WSL: Ubuntu") {
		t.Errorf("wsl: %+v", c)
	}
	// ssh: local files only; the warning about remote bind mounts is always present.
	sshEnv := SelectEnv(model.ConnectionProfile{Type: model.ConnSSH, Endpoint: "ssh://u@h"})
	c = Assess(ctx, AssessInput{Env: sshEnv, HostOS: "linux", ConfigFiles: []string{"/a"}, FS: fakeFS{"/a": true}, CLI: cliOK})
	if !c.CanRun || !strings.Contains(c.Warning, "REMOTE host") || !strings.Contains(c.Warning, "build context") {
		t.Errorf("ssh: %+v", c)
	}
	c = Assess(ctx, AssessInput{Env: sshEnv, HostOS: "linux", ConfigFiles: []string{"/remote/only.yml"}, FS: fakeFS{}, CLI: cliOK})
	if c.CanRun {
		t.Errorf("ssh project with files absent locally must not run: %+v", c)
	}
	// TCP warning survives.
	tcp := SelectEnv(model.ConnectionProfile{Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:2375"})
	c = Assess(ctx, AssessInput{Env: tcp, HostOS: "linux", ConfigFiles: []string{"/a/c.yml"}, FS: fakeFS{"/a/c.yml": true}, CLI: cliOK})
	if !c.CanRun || !strings.Contains(c.Warning, "remote Engine host") {
		t.Errorf("tcp: %+v", c)
	}
}

type fakeRunner struct {
	calls []Command
	fail  func(Command) bool
}

func (f *fakeRunner) Run(_ context.Context, c Command, on func(string)) error {
	f.calls = append(f.calls, c)
	on("ok")
	return nil
}

func (f *fakeRunner) Output(_ context.Context, c Command) (string, error) {
	f.calls = append(f.calls, c)
	if f.fail != nil && f.fail(c) {
		return "", context.DeadlineExceeded
	}
	return "", nil
}

func TestWSLFS_UsesArgvAndRefusesUnsafePaths(t *testing.T) {
	fr := &fakeRunner{fail: func(c Command) bool { return c.Args[len(c.Args)-1] == "/missing" }}
	fs := WSLFS{Runner: fr, Distro: "Ubuntu"}
	ctx := context.Background()
	if !fs.FileExists(ctx, "/ok") || fs.FileExists(ctx, "/missing") || fs.FileExists(ctx, "rel") || fs.FileExists(ctx, "--x") {
		t.Fatal("unexpected FileExists results")
	}
	if got := fr.calls[0]; got.Name != "wsl.exe" || !reflect.DeepEqual(got.Args, []string{"-d", "Ubuntu", "--exec", "test", "-f", "/ok"}) {
		t.Fatalf("%+v", got)
	}
	if len(fr.calls) != 2 {
		t.Fatalf("invalid paths must not spawn: %d", len(fr.calls))
	}
	if (WSLFS{Runner: fr, Distro: "a;b"}).DirExists(ctx, "/x") {
		t.Fatal("bad distro must not run")
	}
	st := CheckCLI(ctx, fr, SelectEnv(model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "Ubuntu"}))
	if !st.OK {
		t.Fatal("cli should be ok")
	}
}
