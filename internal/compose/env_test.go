package compose

import (
	"reflect"
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestSelectEnv_PerConnectionType(t *testing.T) {
	cases := []struct {
		name     string
		p        model.ConnectionProfile
		kind     Kind
		location string
		env      []string
		warn     bool
	}{
		{"npipe", model.ConnectionProfile{Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/docker_engine"}, KindLocal, "This machine", []string{"DOCKER_HOST=npipe:////./pipe/docker_engine"}, false},
		{"unix", model.ConnectionProfile{Type: model.ConnUnix, Endpoint: "unix:///var/run/docker.sock"}, KindLocal, "This machine", []string{"DOCKER_HOST=unix:///var/run/docker.sock"}, false},
		{"tcp plain", model.ConnectionProfile{Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:2375"}, KindLocal, "This machine", []string{"DOCKER_HOST=tcp://127.0.0.1:2375"}, true},
		{"tcp tls", model.ConnectionProfile{Type: model.ConnTCP, Endpoint: "tcp://h:2376", TLSVerify: true, TLSCertDir: "C:\\certs"}, KindLocal, "This machine",
			[]string{"DOCKER_HOST=tcp://h:2376", "DOCKER_TLS_VERIFY=1", "DOCKER_CERT_PATH=C:\\certs"}, true},
		{"wsl", model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "Ubuntu"}, KindWSL, "WSL: Ubuntu", nil, false},
		{"wsl bad", model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "a;b"}, KindNone, "WSL", nil, false},
		{"ssh", model.ConnectionProfile{Type: model.ConnSSH, Endpoint: "ssh://u@h:2222"}, KindLocal, "This machine", []string{"DOCKER_HOST=ssh://u@h:2222"}, true},
		{"ssh invalid host", model.ConnectionProfile{Type: model.ConnSSH, Endpoint: "ssh://u@h;rm"}, KindNone, "Not available", nil, false},
		{"ssh password", model.ConnectionProfile{Type: model.ConnSSH, Endpoint: "ssh://u:pw@h"}, KindNone, "Not available", nil, false},
		{"ssh path", model.ConnectionProfile{Type: model.ConnSSH, Endpoint: "ssh://u@h/x"}, KindNone, "Not available", nil, false},
		{"context ssh", model.ConnectionProfile{Type: model.ConnContext, Endpoint: "ssh://dev@box"}, KindLocal, "This machine", []string{"DOCKER_HOST=ssh://dev@box"}, true},
		{"context other", model.ConnectionProfile{Type: model.ConnContext, Endpoint: "tcp://h:1"}, KindNone, "Not available", nil, false},
		{"context empty", model.ConnectionProfile{Type: model.ConnContext}, KindNone, "Not available", nil, false},
	}
	for _, c := range cases {
		e := SelectEnv(c.p)
		if e.Kind != c.kind || e.Location != c.location || !reflect.DeepEqual(e.EnvSet, c.env) || (e.Warning != "") != c.warn {
			t.Errorf("%s: got %+v", c.name, e)
		}
		if c.kind == KindNone && e.Reason == "" {
			t.Errorf("%s: missing reason", c.name)
		}
	}
}

func TestMergeEnv_OverridesInheritedDockerSettings(t *testing.T) {
	e := SelectEnv(model.ConnectionProfile{Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/docker_engine"})
	got := MergeEnv([]string{"PATH=x", "docker_host=tcp://other", "DOCKER_CONTEXT=remote", "DOCKER_TLS_VERIFY=1"}, e.EnvSet, e.EnvUnset)
	want := []string{"PATH=x", "DOCKER_HOST=npipe:////./pipe/docker_engine"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestBuildCommand_LocalWSLAndSSH(t *testing.T) {
	c, err := BuildCommand(SelectEnv(model.ConnectionProfile{Type: model.ConnUnix, Endpoint: "unix:///x.sock"}), req(VerbRestart))
	if err != nil || c.Name != "docker" || c.Args[0] != "compose" || c.EnvSet[0] != "DOCKER_HOST=unix:///x.sock" {
		t.Fatalf("%+v %v", c, err)
	}
	c, err = BuildCommand(SelectEnv(model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "Ubuntu"}), req(VerbUp))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-d", "Ubuntu", "--cd", "/srv", "--exec", "docker", "compose", "-p", "catalo"}
	if c.Name != "wsl.exe" || !reflect.DeepEqual(c.Args[:len(want)], want) {
		t.Fatalf("%+v", c)
	}
	r := req(VerbUp)
	r.ConfigFiles = []string{`C:\proj\compose.yml`}
	if _, err := BuildCommand(SelectEnv(model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "Ubuntu"}), r); err == nil {
		t.Error("windows path accepted for WSL")
	}
	if _, err := BuildCommand(SelectEnv(model.ConnectionProfile{Type: model.ConnSSH, Endpoint: "ssh://u@h;x"}), req(VerbUp)); err == nil {
		t.Error("invalid ssh endpoint must not build a command")
	}
	c, err = BuildCommand(SelectEnv(model.ConnectionProfile{Type: model.ConnSSH, Endpoint: "ssh://u@h"}), req(VerbUp))
	if err != nil || c.Name != "docker" || c.EnvSet[0] != "DOCKER_HOST=ssh://u@h" || !strings.Contains(strings.Join(c.EnvUnset, " "), "DOCKER_CONTEXT") {
		t.Errorf("ssh command: %+v %v", c, err)
	}
}
