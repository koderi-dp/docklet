package runspec

import (
	"testing"

	"github.com/moby/moby/api/types/container"
)

// Podman's inspect output spells out defaults and uses names docker run rejects.
func TestFromInspectPodmanDefaultsAndNamesAreNotFlags(t *testing.T) {
	pids := int64(2048)
	in := container.InspectResponse{
		ID: "0123456789abcdef0123", Name: "/web",
		Config: &container.Config{Image: "docker.io/traefik/whoami:latest"},
		HostConfig: &container.HostConfig{
			ShmSize: 65536000, PidMode: "private", UTSMode: "private", Runtime: "oci",
			Resources: container.Resources{
				PidsLimit: &pids,
				Ulimits:   []*container.Ulimit{{Name: "RLIMIT_NOFILE", Soft: 1048576, Hard: 1048576}, {Name: "RLIMIT_NPROC", Soft: 10, Hard: 20}},
			},
		},
	}
	s := FromInspect(in, nil, Hints{Podman: true})
	if s.PidsLimit != 0 || s.ShmSize != 0 || s.PidMode != "" || s.UTSMode != "" || s.Runtime != "" {
		t.Errorf("Podman defaults leaked: %+v", s)
	}
	if len(s.Ulimits) != 2 || s.Ulimits[0] != "nofile=1048576" || s.Ulimits[1] != "nproc=10:20" {
		t.Errorf("ulimit names must be docker's: %v", s.Ulimits)
	}
	// The same numbers on a Docker engine are the user's own settings and stay.
	d := FromInspect(in, nil, Hints{})
	if d.PidsLimit != 2048 || d.ShmSize != 65536000 {
		t.Errorf("on Docker an explicit pids limit and shm size must be kept: %+v", d)
	}
}

func TestPodmanInjectedEnvIsNotCopied(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	in := container.InspectResponse{
		ID: id, Name: "/web",
		Config: &container.Config{Image: "x", Env: []string{"container=podman", "HOME=/root", "HOSTNAME=0123456789ab", "MODE=prod", "HOSTNAME_X=1"}},
	}
	if got := FromInspect(in, nil, Hints{Podman: true}).Env; len(got) != 2 || got[0].Key != "MODE" || got[1].Key != "HOSTNAME_X" {
		t.Errorf("only the user's variables may remain: %+v", got)
	}
	if got := FromInspect(in, nil, Hints{}).Env; len(got) != 5 {
		t.Errorf("on Docker nothing is dropped: %+v", got)
	}
	if !PodmanInjectedEnv("HOME=", id) || PodmanInjectedEnv("HOME=/srv/app", id) || PodmanInjectedEnv("HOSTNAME=db", id) {
		t.Error("only the injected values count, a user's own HOME or HOSTNAME stays")
	}
}
