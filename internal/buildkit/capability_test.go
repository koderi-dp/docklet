package buildkit

import (
	"strings"
	"testing"
)

func TestParseBuildxVersion(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/docker/buildx v0.38.1 0dbe87f36ed472fe36b6796c3f70b4210a731fc5\n": "0.38.1",
		"github.com/docker/buildx v0.12.1-desktop.4 abc":                              "0.12.1",
		"docker/buildx v0.10.4 c513d34":                                               "0.10.4",
		"github.com/docker/buildx 0.9.1+azure-1":                                      "0.9.1",
	} {
		v, ok := ParseBuildxVersion(in)
		if !ok || v.String() != want {
			t.Errorf("%q -> %v %v, want %s", in, v, ok, want)
		}
	}
	for _, bad := range []string{"", "docker: 'buildx' is not a docker command.", "exec: \"docker\": executable file not found"} {
		if _, ok := ParseBuildxVersion(bad); ok {
			t.Errorf("%q must not parse", bad)
		}
	}
}

func TestParseDockerVersion(t *testing.T) {
	for in, want := range map[string]string{"29.9.3\n": "29.9.3", "20.10.24": "20.10.24", "v24.0.7+azure\nmore": "24.0.7"} {
		if v, ok := ParseDockerVersion(in); !ok || v.String() != want {
			t.Errorf("%q -> %v %v", in, v, ok)
		}
	}
	if _, ok := ParseDockerVersion("Cannot connect to the Docker daemon"); ok {
		t.Error("error text must not parse")
	}
}

func TestDecide(t *testing.T) {
	v := func(a, b, c int) Version { return Version{a, b, c} }
	c := Decide(Probe{HasBuildx: true, Buildx: v(0, 19, 2), HasDocker: true, Docker: v(27, 1, 0)})
	if !c.Available || c.Method != MethodBuildx || !c.RawJSON || c.Summary != "BuildKit via docker buildx 0.19" {
		t.Errorf("%+v", c)
	}
	c = Decide(Probe{HasBuildx: true, Buildx: v(0, 12, 1)})
	if !c.Available || c.RawJSON || !strings.Contains(c.Summary, "plain progress") {
		t.Errorf("old buildx: %+v", c)
	}
	// docker build is only a BuildKit client before 23.0 (later it needs the buildx plugin).
	c = Decide(Probe{HasDocker: true, Docker: v(20, 10, 24)})
	if !c.Available || c.Method != MethodDockerBuild || c.RawJSON {
		t.Errorf("legacy: %+v", c)
	}
	c = Decide(Probe{HasDocker: true, Docker: v(26, 0, 0)})
	if c.Available || !strings.Contains(c.Summary, "Classic builder:") || !strings.Contains(c.Reason, "buildx plugin is missing") {
		t.Errorf("no plugin: %+v", c)
	}
	if c = Decide(Probe{HasDocker: true, Docker: v(17, 5, 0)}); c.Available {
		t.Errorf("too old: %+v", c)
	}
	c = Decide(Probe{})
	if c.Available || c.Summary != "Classic builder: the docker CLI was not found for this connection" {
		t.Errorf("none: %+v", c)
	}
}
