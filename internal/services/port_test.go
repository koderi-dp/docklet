package services

import (
	"runtime"
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestHostSources_OnlyAnEngineOnThisComputerHasHostListeners(t *testing.T) {
	cases := []struct {
		name     string
		p        model.ConnectionProfile
		wantN    int
		unknown  bool
		wantWSLs bool
	}{
		{"named pipe", model.ConnectionProfile{Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/docker_engine"}, 1, false, false},
		{"unix socket", model.ConnectionProfile{Type: model.ConnUnix, Endpoint: "unix:///var/run/docker.sock"}, 1, false, false},
		{"docker context to a pipe", model.ConnectionProfile{Type: model.ConnContext, Endpoint: "npipe:////./pipe/dockerDesktopLinuxEngine"}, 1, false, false},
		{"tcp", model.ConnectionProfile{Type: model.ConnTCP, Endpoint: "tcp://10.0.0.5:2376"}, 0, true, false},
		{"ssh", model.ConnectionProfile{Type: model.ConnSSH, Endpoint: "ssh://me@box"}, 0, true, false},
	}
	for _, c := range cases {
		hosts, why := hostSources(c.p)
		if len(hosts) != c.wantN || (why != "") != c.unknown {
			t.Errorf("%s: got %d sources, reason %q", c.name, len(hosts), why)
		}
	}
}

func TestHostSources_WSLEngineChecksTheDistroAndWindows(t *testing.T) {
	hosts, why := hostSources(model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "Ubuntu-24.04"})
	if why != "" || len(hosts) == 0 || hosts[0].Where() != "WSL (Ubuntu-24.04)" {
		t.Fatalf("got %v, %q", hosts, why)
	}
	if runtime.GOOS == "windows" && (len(hosts) != 2 || !strings.EqualFold(hosts[1].Where(), "Windows")) {
		t.Errorf("WSL forwards ports to Windows, so Windows must be checked too: %v", hosts)
	}
}
