package runspec

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
)

func isSecret(k string) bool { return strings.Contains(strings.ToUpper(k), "PASSWORD") }

func TestFromInspectOnlyNonDefaults(t *testing.T) {
	in := container.InspectResponse{
		ID: "0123456789abcdef0123", Name: "/db",
		Config: &container.Config{
			Image: "postgres:16", Hostname: "0123456789ab",
			Env:    []string{"PATH=/bin", "POSTGRES_PASSWORD=" + secretValue, "MODE=x"},
			Cmd:    []string{"postgres"},
			Labels: map[string]string{"com.docker.compose.project": "p", "keep": "me", "from.image": "v"},
			Tty:    true, OpenStdin: true,
		},
		HostConfig: &container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyOnFailure, MaximumRetryCount: 2},
			NetworkMode:   "bridge",
			PortBindings:  network.PortMap{network.MustParsePort("5432/tcp"): {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "5433"}}},
			ShmSize:       64 << 20,
			LogConfig:     container.LogConfig{Type: "json-file"},
		},
		Mounts: []container.MountPoint{
			{Type: mount.TypeVolume, Name: "pgdata", Destination: "/var/lib/postgresql/data", RW: true},
			{Type: mount.TypeVolume, Name: strings.Repeat("ab", 32), Destination: "/image-volume", RW: true},
			{Type: mount.TypeBind, Source: "/h", Destination: "/c", RW: false},
		},
	}
	def := &image.InspectResponse{}
	def = withImageConfig(def, []string{"PATH=/bin"}, []string{"postgres"}, map[string]string{"from.image": "v"}, "/image-volume")

	s := FromInspect(in, def, Hints{IsSecret: isSecret})
	if len(s.Env) != 2 || s.Env[0].Key != "POSTGRES_PASSWORD" || !s.Env[0].Secret || s.Env[1].Key != "MODE" {
		t.Errorf("env %+v (image env must be skipped, secrets flagged)", s.Env)
	}
	if len(s.Labels) != 1 || s.Labels[0].Key != "keep" {
		t.Errorf("labels %+v (image and compose labels must be skipped)", s.Labels)
	}
	if s.CmdSet || s.EntrypointSet || s.Hostname != "" || s.NetworkMode != "" || s.ShmSize != 0 || s.LogDriver != "" {
		t.Errorf("defaults leaked into the spec: %+v", s)
	}
	if s.Restart != "on-failure:2" || !s.Stdin || !s.TTY {
		t.Errorf("restart/tty: %+v", s)
	}
	if len(s.Ports) != 1 || portSpec(s.Ports[0]) != "127.0.0.1:5433:5432" {
		t.Errorf("ports %+v", s.Ports)
	}
	if len(s.Mounts) != 2 || s.Mounts[0].Target != "/c" || !s.Mounts[0].ReadOnly || s.Mounts[1].Source != "pgdata" {
		t.Errorf("mounts %+v (anonymous image volume must be skipped)", s.Mounts)
	}
}

func TestFromInspectCommandRules(t *testing.T) {
	base := container.InspectResponse{ID: "x", Name: "/c"}
	def := withImageConfig(&image.InspectResponse{}, nil, []string{"serve"}, nil, "")
	def.Config.Entrypoint = []string{"/entry"}

	base.Config = &container.Config{Image: "i", Entrypoint: []string{"/entry"}, Cmd: []string{"serve"}}
	if s := FromInspect(base, def, Hints{}); s.CmdSet || s.EntrypointSet {
		t.Errorf("same as image: %+v", s)
	}
	base.Config = &container.Config{Image: "i", Entrypoint: []string{"/entry"}, Cmd: []string{"other", "x y"}}
	if s := FromInspect(base, def, Hints{}); !s.CmdSet || s.EntrypointSet || len(s.Cmd) != 2 {
		t.Errorf("command override: %+v", s)
	}
	base.Config = &container.Config{Image: "i", Entrypoint: []string{"/new", "a"}}
	if s := FromInspect(base, def, Hints{}); !s.EntrypointSet || s.CmdSet || s.Entrypoint[1] != "a" {
		t.Errorf("entrypoint override resets cmd: %+v", s)
	}
	base.Config = &container.Config{Image: "sha256:" + strings.Repeat("a", 64)}
	if s := FromInspect(base, nil, Hints{}); len(s.NotIncluded) == 0 {
		t.Errorf("image ID must be reported")
	}
}

func TestFromInspectNetworksDropEngineAliases(t *testing.T) {
	in := container.InspectResponse{ID: "0123456789abcdef", Name: "/app-1",
		Config:     &container.Config{Image: "i", Labels: map[string]string{"com.docker.compose.service": "app", "com.docker.compose.project": "p"}},
		HostConfig: &container.HostConfig{NetworkMode: "p_default"},
		NetworkSettings: &container.NetworkSettings{Networks: map[string]*network.EndpointSettings{
			"p_default": {Aliases: []string{"app", "0123456789ab", "extra"}},
			"p_back":    {},
		}}}
	s := FromInspect(in, nil, Hints{})
	if s.NetworkMode != "p_default" || len(s.Networks) != 2 {
		t.Fatalf("networks %+v", s.Networks)
	}
	if s.Networks[0].Name != "p_default" || len(s.Networks[0].Aliases) != 1 || s.Networks[0].Aliases[0] != "extra" {
		t.Errorf("primary first, engine aliases dropped: %+v", s.Networks)
	}
}

func TestFromInspectFlagsPasswordsInURLs(t *testing.T) {
	in := container.InspectResponse{ID: "x", Name: "/c", Config: &container.Config{Image: "i", Env: []string{
		"DATABASE_URL=postgres://app:hunter2@db:5432/x", "PLAIN_URL=https://example.com/a", "USER_ONLY=ssh://git@host/repo"}}}
	s := FromInspect(in, nil, Hints{})
	if !s.Env[0].Secret || s.Env[1].Secret || s.Env[2].Secret {
		t.Errorf("%+v", s.Env)
	}
	if strings.Contains(ToDockerRun(s, Options{Shell: Bash}), "hunter2") || strings.Contains(ToCompose(s, Options{}), "hunter2") {
		t.Errorf("URL password leaked")
	}
}
