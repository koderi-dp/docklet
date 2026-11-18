package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf16"

	"docklet/internal/model"
)

func utf16le(s string, bom bool) []byte {
	var b []byte
	if bom {
		b = append(b, 0xFF, 0xFE)
	}
	for _, u := range utf16.Encode([]rune(s)) {
		b = append(b, byte(u), byte(u>>8))
	}
	return b
}

func TestParseWSLList_HandlesUTF16BOMUTF8AndEmpty(t *testing.T) {
	text := "  NAME              STATE           VERSION\r\n* Ubuntu-24.04      Running         2\r\n  Debian            Stopped         2\r\n  old-one           Stopped         1\r\n"
	for _, bom := range []bool{true, false} {
		got := parseWSLList(utf16le(text, bom))
		want := []model.WSLDistro{
			{Name: "Ubuntu-24.04", State: "Running", Version: 2, Default: true},
			{Name: "Debian", State: "Stopped", Version: 2},
			{Name: "old-one", State: "Stopped", Version: 1},
		}
		if len(got) != len(want) {
			t.Fatalf("bom=%v got %+v", bom, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("bom=%v [%d] = %+v want %+v", bom, i, got[i], want[i])
			}
		}
	}
	// UTF-8 output (WSL_UTF8=1) and garbage/empty input must not panic or invent distros.
	if got := parseWSLList([]byte(text)); len(got) != 3 {
		t.Fatalf("utf8: %+v", got)
	}
	if got := parseWSLList(utf16le("Windows Subsystem for Linux has no installed distributions.\r\n", true)); len(got) != 0 {
		t.Fatalf("no distros: %+v", got)
	}
	if got := parseWSLList(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil input must give empty non-nil list: %#v", got)
	}
}

func TestWSLDockerInfo_NeverExecutesInStoppedDistro(t *testing.T) {
	oldRun, oldSup := runCommand, wslSupported
	defer func() { runCommand, wslSupported = oldRun, oldSup }()
	wslSupported = true
	var calls [][]string
	running := ""
	runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		if len(args) > 0 && args[0] == "--list" {
			return utf16le(running, true), nil
		}
		return []byte("27.1.1|27.1.1\n"), nil
	}
	m := NewManager(&memStore{}, nil)
	info, err := m.WSLDockerInfo(context.Background(), "Debian")
	if err != nil || info.Running || info.CLIFound {
		t.Fatalf("stopped distro: %+v %v", info, err)
	}
	for _, c := range calls {
		if strings.Contains(strings.Join(c, " "), "docker") {
			t.Fatalf("stopped distro must not be executed in: %v", c)
		}
	}
	running = "Debian\r\n"
	info, _ = m.WSLDockerInfo(context.Background(), "Debian")
	if !info.Running || !info.CLIFound || !info.DaemonReachable || info.ServerVersion != "27.1.1" {
		t.Fatalf("running distro: %+v", info)
	}
	if _, err := m.WSLDockerInfo(context.Background(), "-e sh"); err == nil {
		t.Fatal("option-like distro name must be rejected")
	}
}

func TestClassifyWSLDocker_SeparatesMissingCLIFromUnreachableDaemon(t *testing.T) {
	cases := []struct {
		out        string
		err        error
		cli, alive bool
	}{
		{"27.1.1|27.1.1\n", nil, true, true},
		{"27.1.1|\nCannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?", errors.New("exit 1"), true, false},
		{"wsl: execvpe(docker) failed: No such file or directory", errors.New("exit 1"), false, false},
		{"failed to connect to the docker API at unix:///var/run/docker.sock; check if the path is correct and if the daemon is running: dial unix /var/run/docker.sock: connect: no such file or directory", errors.New("exit 1"), true, false},
		{"permission denied while trying to connect to the Docker daemon socket", errors.New("exit 1"), true, false},
	}
	for i, c := range cases {
		got := classifyWSLDocker("U", c.out, c.err)
		if got.CLIFound != c.cli || got.DaemonReachable != c.alive || got.Detail == "" {
			t.Errorf("case %d: %+v", i, got)
		}
	}
}
