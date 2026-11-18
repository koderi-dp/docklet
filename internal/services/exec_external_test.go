package services

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestBuildTermCommand_PerConnectionType(t *testing.T) {
	exec := []string{"docker", "exec", "-it", "abc123", "/bin/sh"}
	cases := map[string]struct {
		p        model.ConnectionProfile
		argv     []string
		set      []string
		needs    string
		distro   string
		unsetHas string
	}{
		"npipe": {
			p:    model.ConnectionProfile{Name: "n", Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/docker_engine"},
			argv: exec, set: []string{"DOCKER_HOST=npipe:////./pipe/docker_engine"}, needs: "docker", unsetHas: "DOCKER_CONTEXT",
		},
		"unix": {
			p:    model.ConnectionProfile{Name: "u", Type: model.ConnUnix, Endpoint: "unix:///var/run/docker.sock"},
			argv: exec, set: []string{"DOCKER_HOST=unix:///var/run/docker.sock"}, needs: "docker", unsetHas: "DOCKER_TLS_VERIFY",
		},
		"tcp tls": {
			p:    model.ConnectionProfile{Name: "t", Type: model.ConnTCP, Endpoint: "tcp://10.0.0.2:2376", TLSVerify: true, TLSCertDir: os.TempDir()},
			argv: exec, set: []string{"DOCKER_HOST=tcp://10.0.0.2:2376", "DOCKER_TLS_VERIFY=1", "DOCKER_CERT_PATH=" + os.TempDir()}, needs: "docker",
		},
		"wsl": {
			p:    model.ConnectionProfile{Name: "w", Type: model.ConnWSL, WSLDistro: "Ubuntu-24.04"},
			argv: append([]string{"wsl.exe", "-d", "Ubuntu-24.04", "--"}, exec...), distro: "Ubuntu-24.04",
		},
		"ssh": {
			p:    model.ConnectionProfile{Name: "s", Type: model.ConnSSH, Endpoint: "ssh://me@build.example.com:2222"},
			argv: append([]string{"ssh", "-t", "-p", "2222", "--", "me@build.example.com"}, exec...), needs: "ssh",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			tc, err := buildTermCommand(c.p, "abc123", "")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tc.Argv, c.argv) {
				t.Errorf("argv %v want %v", tc.Argv, c.argv)
			}
			if len(c.set) > 0 && !reflect.DeepEqual(tc.Set, c.set) {
				t.Errorf("env %v want %v", tc.Set, c.set)
			}
			if tc.NeedsBinary != c.needs || tc.Distro != c.distro {
				t.Errorf("needs %q distro %q", tc.NeedsBinary, tc.Distro)
			}
			if c.unsetHas != "" && !strings.Contains(strings.Join(tc.Unset, ","), c.unsetHas) {
				t.Errorf("a stale %s from the user's environment must be unset: %v", c.unsetHas, tc.Unset)
			}
		})
	}
}

func TestBuildTermCommand_RejectsInjection(t *testing.T) {
	local := model.ConnectionProfile{Name: "n", Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/docker_engine"}
	for _, id := range []string{"", "a b", "x;rm -rf /", "$(id)", "-it", "a&b", "a\nb", "../x", "a|b"} {
		if _, err := buildTermCommand(local, id, "sh"); err == nil {
			t.Errorf("container reference %q must be rejected", id)
		}
	}
	for _, sh := range []string{"sh; reboot", "/bin/sh -c x", "../../bin/sh", "bash\nid"} {
		if _, err := buildTermCommand(local, "abc", sh); err == nil {
			t.Errorf("shell %q must be rejected", sh)
		}
	}
	bad := model.ConnectionProfile{Name: "w", Type: model.ConnWSL, WSLDistro: "x; calc"}
	if _, err := buildTermCommand(bad, "abc", ""); err == nil {
		t.Error("an unsafe WSL distribution name must be rejected")
	}
	ssh := model.ConnectionProfile{Name: "s", Type: model.ConnSSH, Endpoint: "ssh://-oProxyCommand=calc@host"}
	if _, err := buildTermCommand(ssh, "abc", ""); err == nil {
		t.Error("an ssh endpoint that looks like an option must be rejected")
	}
}

func TestBuildScript_QuotesPerOS(t *testing.T) {
	tc := termCommand{Argv: []string{"docker", "exec", "-it", "abc", "/bin/sh"}, Set: []string{"DOCKER_HOST=tcp://h:2376", "DOCKER_CERT_PATH=C:\\my certs"}, Unset: []string{"DOCKER_CONTEXT"}}
	win, err := buildScript("windows", tc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"set \"DOCKER_CONTEXT=\"\r\n", "set \"DOCKER_CERT_PATH=C:\\my certs\"\r\n", "docker exec -it abc /bin/sh\r\n", "if errorlevel 1 pause"} {
		if !strings.Contains(win, want) {
			t.Errorf("windows script lacks %q:\n%s", want, win)
		}
	}
	posix, err := buildScript("linux", termCommand{Argv: []string{"docker", "exec"}, Set: []string{"X=it's a trap"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(posix, `export X='it'\''s a trap'`) {
		t.Errorf("single quote not escaped:\n%s", posix)
	}
	for _, bad := range []termCommand{
		{Argv: []string{"docker", "a\nb"}},
		{Argv: []string{"docker"}, Set: []string{"K=v\nrm"}},
		{Argv: []string{"docker"}, Set: []string{"K L=v"}},
	} {
		if _, err := buildScript("linux", bad); err == nil {
			t.Errorf("%+v must be rejected", bad)
		}
	}
	if _, err := buildScript("windows", termCommand{Argv: []string{"docker", `a"&calc`}}); err == nil {
		t.Error("a double quote must be rejected for cmd.exe")
	}
}

func TestTerminalArgv_PerOSAndCustomTemplate(t *testing.T) {
	has := func(names ...string) func(string) (string, error) {
		return func(n string) (string, error) {
			for _, x := range names {
				if x == n {
					return n, nil
				}
			}
			return "", errors.New("not found")
		}
	}
	auto := model.ExternalTerminal{Mode: "auto"}
	check := func(goos string, tm model.ExternalTerminal, look func(string) (string, error), want []string) {
		t.Helper()
		got, err := terminalArgv(goos, tm, "S", look)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v err %v want %v", goos, got, err, want)
		}
	}
	check("windows", auto, has("wt.exe"), []string{"wt.exe", "cmd.exe", "/c", "S"})
	check("windows", auto, has(), []string{"cmd.exe", "/c", "start", "", "cmd.exe", "/c", "S"})
	check("darwin", auto, has(), []string{"open", "-a", "Terminal", "S"})
	check("linux", auto, has("konsole", "xterm"), []string{"konsole", "-e", "/bin/sh", "S"})
	check("linux", auto, has("x-terminal-emulator", "xterm"), []string{"x-terminal-emulator", "-e", "/bin/sh", "S"})
	check("linux", model.ExternalTerminal{Mode: "custom", Template: `"/opt/my term/t" --run {cmd}`}, has(), []string{"/opt/my term/t", "--run", "/bin/sh", "S"})
	if _, err := terminalArgv("linux", auto, "S", has()); err == nil {
		t.Error("no terminal emulator must be a friendly error")
	}
	if _, err := terminalArgv("linux", model.ExternalTerminal{Mode: "custom", Template: "alacritty -e"}, "S", has()); err == nil {
		t.Error("a custom template without {cmd} must be rejected")
	}
}

func TestLaunchExternal_FriendlyErrorWhenCLIIsMissing(t *testing.T) {
	d := extDeps{goos: "linux", tempDir: t.TempDir(),
		lookPath: func(string) (string, error) { return "", errors.New("missing") },
		start:    func([]string) error { t.Fatal("must not start a terminal"); return nil }}
	err := launchExternal(context.Background(), d, termCommand{Argv: []string{"docker"}, NeedsBinary: "docker"}, model.ExternalTerminal{Mode: "auto"})
	if err == nil || !strings.Contains(err.Error(), "docker CLI was not found") {
		t.Fatalf("got %v", err)
	}
	d.dockerInDistro = func(context.Context, string) error { return errors.New("nope") }
	err = launchExternal(context.Background(), d, termCommand{Argv: []string{"wsl.exe"}, Distro: "Ubuntu"}, model.ExternalTerminal{Mode: "auto"})
	if err == nil || !strings.Contains(err.Error(), `"Ubuntu"`) {
		t.Fatalf("got %v", err)
	}
}

func TestLaunchExternal_WritesScriptAndStartsTerminal(t *testing.T) {
	var started []string
	d := extDeps{goos: "linux", tempDir: t.TempDir(),
		lookPath: func(n string) (string, error) { return n, nil },
		start:    func(a []string) error { started = a; return nil }}
	tc := termCommand{Argv: []string{"docker", "exec", "-it", "abc", "/bin/sh"}, Set: []string{"DOCKER_HOST=tcp://h:2376"}, NeedsBinary: "docker"}
	if err := launchExternal(context.Background(), d, tc, model.ExternalTerminal{Mode: "auto"}); err != nil {
		t.Fatal(err)
	}
	if len(started) != 4 || started[0] != "x-terminal-emulator" {
		t.Fatalf("argv %v", started)
	}
	body, err := os.ReadFile(started[3])
	if err != nil || !strings.Contains(string(body), "export DOCKER_HOST='tcp://h:2376'") {
		t.Fatalf("script %q err %v", body, err)
	}
}
