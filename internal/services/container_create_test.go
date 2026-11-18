package services

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"

	"docklet/internal/model"
)

func TestSplitCommand_HandlesQuotesAndRejectsUnterminated(t *testing.T) {
	tests := []struct {
		in   string
		want []string
		err  bool
	}{
		{"", nil, false},
		{`sleep 600`, []string{"sleep", "600"}, false},
		{`sh -c "echo 'a b'; echo \"x\""`, []string{"sh", "-c", `echo 'a b'; echo "x"`}, false},
		{`echo 'it''s' a\ b`, []string{"echo", "its", "a b"}, false},
		{`echo "unterminated`, nil, true},
		{`echo \`, nil, true},
		{`echo ""`, []string{"echo", ""}, false},
	}
	for _, tc := range tests {
		got, err := splitCommand(tc.in)
		if (err != nil) != tc.err || strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
			t.Errorf("splitCommand(%q) = %q, %v", tc.in, got, err)
		}
	}
}

func TestBuildCreateConfig_MapsSpecToEngineConfig(t *testing.T) {
	cfg, host, err := buildCreateConfig(model.ContainerCreateSpec{
		Name: "web-1", Image: "nginx:1.27", Command: "nginx -g 'daemon off;'",
		Env:           []model.CreateEnvVar{{Key: "A", Value: "1=2"}, {}},
		Ports:         []model.PortMapping{{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 80}, {ContainerPort: 53, Protocol: "udp"}},
		Mounts:        []model.MountSpec{{Type: "volume", Source: "data", Target: "/data"}, {Type: "bind", Source: `C:\src`, Target: "/src", ReadOnly: true}},
		RestartPolicy: "on-failure", MaxRetries: 3, MemoryMiB: 256, CPUs: 1.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cfg.Cmd, "|") != "nginx|-g|daemon off;" || len(cfg.Env) != 1 || cfg.Env[0] != "A=1=2" {
		t.Fatalf("cmd/env: %q %q", cfg.Cmd, cfg.Env)
	}
	p80 := network.MustParsePort("80/tcp")
	b := host.PortBindings[p80]
	if len(b) != 1 || b[0].HostPort != "8080" || b[0].HostIP.String() != "127.0.0.1" {
		t.Fatalf("binding: %+v", b)
	}
	if b := host.PortBindings[network.MustParsePort("53/udp")]; len(b) != 1 || b[0].HostPort != "" {
		t.Fatalf("udp binding: %+v", b)
	}
	if _, ok := cfg.ExposedPorts[p80]; !ok || len(host.Mounts) != 2 || !host.Mounts[1].ReadOnly || host.Mounts[1].Type != "bind" {
		t.Fatalf("exposed/mounts wrong: %+v %+v", cfg.ExposedPorts, host.Mounts)
	}
	if host.RestartPolicy.Name != container.RestartPolicyOnFailure || host.RestartPolicy.MaximumRetryCount != 3 ||
		host.Memory != 256<<20 || host.NanoCPUs != 1_500_000_000 {
		t.Fatalf("limits/restart: %+v mem=%d cpu=%d", host.RestartPolicy, host.Memory, host.NanoCPUs)
	}
}

func TestBuildCreateConfig_RejectsInvalidSpecs(t *testing.T) {
	bad := map[string]model.ContainerCreateSpec{
		"no image":       {},
		"name":           {Image: "x", Name: "-bad name"},
		"port range":     {Image: "x", Ports: []model.PortMapping{{ContainerPort: 70000}}},
		"host port":      {Image: "x", Ports: []model.PortMapping{{ContainerPort: 80, HostPort: 99999}}},
		"proto":          {Image: "x", Ports: []model.PortMapping{{ContainerPort: 80, Protocol: "icmp"}}},
		"host ip":        {Image: "x", Ports: []model.PortMapping{{ContainerPort: 80, HostIP: "nope"}}},
		"relative mount": {Image: "x", Mounts: []model.MountSpec{{Type: "volume", Source: "v", Target: "data"}}},
		"bind no source": {Image: "x", Mounts: []model.MountSpec{{Type: "bind", Target: "/d"}}},
		"dup target":     {Image: "x", Mounts: []model.MountSpec{{Type: "volume", Source: "a", Target: "/d"}, {Type: "volume", Source: "b", Target: "/d"}}},
		"env key":        {Image: "x", Env: []model.CreateEnvVar{{Key: "A B", Value: "1"}}},
		"restart":        {Image: "x", RestartPolicy: "sometimes"},
		"memory":         {Image: "x", MemoryMiB: 2},
		"quote":          {Image: "x", Command: `echo "`},
	}
	for name, spec := range bad {
		if _, _, err := buildCreateConfig(spec); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func newCreateSvc(t *testing.T, haveImage bool) (*ContainerService, *recEmitter, *[]string) {
	t.Helper()
	var mu sync.Mutex
	calls := []string{}
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if i := strings.Index(p, "/images/"); i >= 0 {
			p = p[i:]
		} else if i := strings.Index(p, "/containers/"); i >= 0 {
			p = p[i:]
		}
		mu.Lock()
		calls = append(calls, r.Method+" "+p)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(p, "/images/nginx/json"):
			if !haveImage {
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"message":"No such image: nginx"}`))
				return
			}
			_, _ = w.Write([]byte(`{"Id":"sha256:abc"}`))
		case p == "/containers/create":
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"Id":"newid1234567890","Warnings":[]}`))
		case strings.HasSuffix(p, "/start"):
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	}), "1.45")
	em := &recEmitter{}
	return &ContainerService{Engines: fakeProvider{cli}, Events: em}, em, &calls
}

func TestCreate_DoesNotPullWhenImageIsMissing(t *testing.T) {
	s, em, calls := newCreateSvc(t, false)
	_, err := s.Create(context.Background(), "c1", model.ContainerCreateSpec{Image: "nginx"})
	if err == nil || !strings.Contains(err.Error(), "Pull it first") {
		t.Fatalf("got %v", err)
	}
	for _, c := range *calls {
		if strings.Contains(c, "create") {
			t.Fatalf("must not create or pull: %v", *calls)
		}
	}
	if len(em.find("containers:changed")) != 0 {
		t.Fatal("no event on failure")
	}
}

func TestCreate_StartsWhenAutoStart(t *testing.T) {
	s, em, calls := newCreateSvc(t, true)
	res, err := s.Create(context.Background(), "c1", model.ContainerCreateSpec{Image: "nginx", Name: "w", AutoStart: true})
	if err != nil || !res.OK || res.ID != "newid1234567890" || res.Error != "" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(em.find("containers:changed")) != 1 {
		t.Fatal("want containers:changed")
	}
	if !strings.Contains(strings.Join(*calls, ","), "POST /containers/newid1234567890/start") {
		t.Fatalf("calls: %v", *calls)
	}
}
