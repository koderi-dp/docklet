package engine

import (
	"testing"

	"github.com/moby/moby/api/types/system"

	"docklet/internal/model"
)

// Captured from `podman machine list --format json` (Podman 6.1.4, Windows, WSL machine).
const machineListJSON = `[
    {
        "Name": "podman-machine-default",
        "Default": false,
        "Created": "2026-11-19T00:05:23.436458+01:00",
        "Running": true,
        "Starting": false,
        "LastUp": "0001-01-01T00:00:00Z",
        "Stream": "default",
        "VMType": "wsl",
        "CPUs": 10,
        "Memory": "2147483648",
        "Swap": "0",
        "DiskSize": "107374182400",
        "Port": 54923,
        "RemoteUsername": "user",
        "IdentityPath": "C:\\Users\\Administrator\\.local\\share\\containers\\podman\\machine\\machine",
        "UserModeNetworking": false
    }
]`

// Captured from `podman machine inspect` (trimmed to the parts Docklet reads).
const machineInspectJSON = `[{"ConnectionInfo":{"PodmanSocket":{"Path":"C:\\Users\\ADMINI~1\\AppData\\Local\\Temp\\podman\\podman-machine-default-api.sock"},"PodmanPipe":{"Path":"\\\\.\\pipe\\podman-machine-default"}},"Name":"podman-machine-default","State":"running","Rootful":true}]`

func TestParseMachineList_RealOutput(t *testing.T) {
	got, err := parseMachineList([]byte(machineListJSON))
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].Name != "podman-machine-default" || !got[0].Running || got[0].VMType != "wsl" {
		t.Fatalf("unexpected machine: %+v", got[0])
	}
	if got, err := parseMachineList([]byte("  \n")); err != nil || got != nil {
		t.Fatalf("empty output means no machines: %+v, %v", got, err)
	}
	if _, err := parseMachineList([]byte("not json")); err == nil {
		t.Fatal("garbage must be an error, not an empty list")
	}
}

func TestParseMachineInspect_RealOutput(t *testing.T) {
	got, err := parseMachineInspect([]byte(machineInspectJSON))
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].ConnectionInfo.PodmanPipe.Path != `\\.\pipe\podman-machine-default` ||
		got[0].ConnectionInfo.PodmanSocket.Path == "" {
		t.Fatalf("connection info not read: %+v", got[0].ConnectionInfo)
	}
}

func TestPodmanPipeProfiles_FindsEveryMachineNotOnlyDefault(t *testing.T) {
	got := podmanPipeProfiles([]string{"docker_engine", "dockerDesktopLinuxEngine", "podman-machine-default", "podman-dev", "podmanx"})
	if len(got) != 2 {
		t.Fatalf("want 2 podman pipes, got %+v", got)
	}
	if got[0].Name != "Podman (default)" || got[0].Endpoint != "npipe:////./pipe/podman-machine-default" ||
		got[0].Type != model.ConnNamedPipe || !got[0].Discovered {
		t.Errorf("unexpected profile: %+v", got[0])
	}
	if got[1].Name != "Podman (dev)" || got[1].ID != "local-podman-podman-dev" {
		t.Errorf("unexpected profile: %+v", got[1])
	}
}

func TestPodmanMachineInfos_StoppedMachineGetsTheStartHint(t *testing.T) {
	machines := []podmanMachine{{Name: "podman-machine-default", Running: true}, {Name: "work", Running: false}}
	got := podmanMachineInfos(machines, []string{"podman-machine-default"})
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if !got[0].Running || got[0].Hint != "" || got[0].Profile.Endpoint != "npipe:////./pipe/podman-machine-default" {
		t.Errorf("running machine: %+v", got[0])
	}
	if got[1].Running || got[1].Name != "work" || got[1].Hint != "Start it with: podman machine start work" ||
		got[1].Profile.Endpoint != "npipe:////./pipe/podman-work" {
		t.Errorf("stopped machine: %+v", got[1])
	}
}

func TestPodmanMachineInfos_NoCLIStillListsPipes(t *testing.T) {
	got := podmanMachineInfos(nil, []string{"podman-machine-default"})
	if len(got) != 1 || !got[0].Running {
		t.Fatalf("a pipe without CLI data is a running machine: %+v", got)
	}
}

func TestPodmanMachineInfos_ListedRunningButNoPipeIsNotRunning(t *testing.T) {
	got := podmanMachineInfos([]podmanMachine{{Name: "podman-machine-default", Running: true}}, nil)
	if len(got) != 1 || got[0].Running || got[0].Hint == "" {
		t.Fatalf("no pipe means the API is not reachable: %+v", got)
	}
}

func TestPodmanLinuxCandidates_RootlessAndRootful(t *testing.T) {
	present := map[string]bool{"/run/user/1000/podman/podman.sock": true, "/run/podman/podman.sock": true}
	got := podmanLinuxCandidates("/run/user/1000", 1000, func(p string) bool { return present[p] })
	if len(got) != 2 {
		t.Fatalf("want rootless and rootful once each, got %+v", got)
	}
	if got[0].Name != "Podman (rootless)" || got[0].Endpoint != "unix:///run/user/1000/podman/podman.sock" {
		t.Errorf("rootless: %+v", got[0])
	}
	if got[1].Name != "Podman (rootful)" || got[1].Endpoint != "unix:///run/podman/podman.sock" {
		t.Errorf("rootful: %+v", got[1])
	}
	if got := podmanLinuxCandidates("", 1000, func(string) bool { return false }); len(got) != 0 {
		t.Fatalf("nothing exists, nothing offered: %+v", got)
	}
	// XDG_RUNTIME_DIR unset: fall back to /run/user/<uid>.
	got = podmanLinuxCandidates("", 1000, func(p string) bool { return p == "/run/user/1000/podman/podman.sock" })
	if len(got) != 1 || got[0].ID != "local-podman-rootless" {
		t.Fatalf("uid fallback: %+v", got)
	}
}

func TestPodmanMacCandidates_BothSocketLocations(t *testing.T) {
	glob := func(p string) []string {
		switch p {
		case "/var/folders/ab/T/podman/*-api.sock":
			return []string{"/var/folders/ab/T/podman/podman-machine-default-api.sock"}
		case "/Users/me/.local/share/containers/podman/machine/*/podman.sock":
			return []string{"/Users/me/.local/share/containers/podman/machine/podman-machine-default/podman.sock", "/Users/me/.local/share/containers/podman/machine/dev/podman.sock"}
		}
		return nil
	}
	got := podmanMacCandidates("/Users/me", "/var/folders/ab/T/", glob)
	if len(got) != 2 || got[0].Name != "Podman (default)" || got[1].Name != "Podman (dev)" {
		t.Fatalf("same machine in both places must be listed once: %+v", got)
	}
	if got[0].Endpoint != "unix:///var/folders/ab/T/podman/podman-machine-default-api.sock" {
		t.Errorf("endpoint: %s", got[0].Endpoint)
	}
}

func TestKindFromVersion(t *testing.T) {
	podman := []system.ComponentVersion{{Name: "Podman Engine", Version: "6.1.4"}, {Name: "Conmon"}, {Name: "Engine", Version: "6.1.4"}}
	if KindFromVersion("linux/amd64/fedora-44", podman) != model.EnginePodman {
		t.Error("Podman Engine component means podman")
	}
	if PodmanVersion(podman) != "6.1.4" {
		t.Errorf("version: %q", PodmanVersion(podman))
	}
	if KindFromVersion("Podman Engine", nil) != model.EnginePodman {
		t.Error("platform name counts too")
	}
	docker := []system.ComponentVersion{{Name: "Engine", Version: "29.1.0"}, {Name: "containerd"}, {Name: "runc"}}
	if KindFromVersion("Docker Desktop 4.49.0", docker) != model.EngineDocker || KindFromVersion("", nil) != model.EngineDocker {
		t.Error("docker and unknown stay docker")
	}
}
