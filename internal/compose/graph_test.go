package compose

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"docklet/internal/model"
)

func loadGraph(t *testing.T) model.ComposeGraph {
	t.Helper()
	raw, err := os.ReadFile("testdata/graph_config.json")
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseGraph(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func svcByName(t *testing.T, g model.ComposeGraph, name string) model.ComposeGraphService {
	t.Helper()
	for _, s := range g.Services {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("service %q missing", name)
	return model.ComposeGraphService{}
}

func TestParseGraph_ObjectForms(t *testing.T) {
	g := loadGraph(t)
	if g.Source != "config" || g.Note != "" {
		t.Errorf("source/note = %q/%q", g.Source, g.Note)
	}
	names := []string{}
	for _, s := range g.Services {
		names = append(names, s.Name)
	}
	if !reflect.DeepEqual(names, []string{"adminer", "api", "migrate", "postgres", "web"}) {
		t.Errorf("services = %v", names)
	}
	if !reflect.DeepEqual(g.Networks, []string{"shop_backend", "shop_default"}) {
		t.Errorf("networks = %v", g.Networks)
	}
	if !reflect.DeepEqual(g.Volumes, []string{"cache", "pgdata"}) {
		t.Errorf("volumes = %v", g.Volumes)
	}

	web := svcByName(t, g, "web")
	wantPorts := []model.ComposeGraphPort{
		{HostIP: "127.0.0.1", Published: "9260", Target: 8080, Protocol: "tcp"},
		{Published: "9443", Target: 443, Protocol: "tcp"},
	}
	if !reflect.DeepEqual(web.Ports, wantPorts) {
		t.Errorf("web ports = %+v", web.Ports)
	}
	wantDeps := []model.ComposeGraphDep{{Service: "api", Condition: "service_started"}, {Service: "postgres", Condition: "service_healthy"}}
	if !reflect.DeepEqual(web.DependsOn, wantDeps) {
		t.Errorf("web deps = %+v", web.DependsOn)
	}
	api := svcByName(t, g, "api")
	if !api.Build || api.Image != "" {
		t.Errorf("api build/image = %v/%q", api.Build, api.Image)
	}
	if !reflect.DeepEqual(api.Volumes, []string{"/var/run/docker.sock"}) {
		t.Errorf("api volumes = %v", api.Volumes)
	}
	if v := svcByName(t, g, "postgres").Volumes; !reflect.DeepEqual(v, []string{"pgdata"}) {
		t.Errorf("postgres volumes = %v", v)
	}
	if len(svcByName(t, g, "postgres").DependsOn) != 0 {
		t.Error("postgres should have no dependencies")
	}
	if p := svcByName(t, g, "adminer").Profiles; !reflect.DeepEqual(p, []string{"debug"}) {
		t.Errorf("adminer profiles = %v", p)
	}
	if v := svcByName(t, g, "migrate").Volumes; len(v) != 0 {
		t.Errorf("tmpfs mount must not be listed: %v", v)
	}
}

func TestParseGraph_ShortForms(t *testing.T) {
	g, err := ParseGraph(`{"services":{
	  "app":{"image":"x","depends_on":["db"],
	    "ports":["8080:80","127.0.0.1:5432:5432/udp","80"],
	    "volumes":["pgdata:/data","./src:/app:ro"]},
	  "db":{"image":"y"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	app := svcByName(t, g, "app")
	if !reflect.DeepEqual(app.DependsOn, []model.ComposeGraphDep{{Service: "db"}}) {
		t.Errorf("deps = %+v", app.DependsOn)
	}
	wantPorts := []model.ComposeGraphPort{
		{Published: "8080", Target: 80, Protocol: "tcp"},
		{HostIP: "127.0.0.1", Published: "5432", Target: 5432, Protocol: "udp"},
		{Published: "", Target: 80, Protocol: "tcp"},
	}
	if !reflect.DeepEqual(app.Ports, wantPorts) {
		t.Errorf("ports = %+v", app.Ports)
	}
	if !reflect.DeepEqual(app.Volumes, []string{"pgdata", "./src"}) {
		t.Errorf("volumes = %v", app.Volumes)
	}
	if g.Networks == nil || g.Volumes == nil || app.Profiles == nil {
		t.Error("slices must be non-nil")
	}
}

func TestParseGraph_NeverLeaksEnvironmentOrCommand(t *testing.T) {
	out, err := json.Marshal(loadGraph(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"s3cr3t-do-not-leak", "daemon-off-secret-command", "run-api-secret-command", "SECRET_TOKEN"} {
		if strings.Contains(string(out), bad) {
			t.Errorf("graph JSON leaks %q", bad)
		}
	}
	if strings.Contains(string(out), "null") {
		t.Errorf("graph JSON has null: %s", out)
	}
}

func TestParseGraph_InvalidJSON(t *testing.T) {
	if _, err := ParseGraph("not json"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestGraphFromContainers(t *testing.T) {
	deps := "postgres:service_healthy:false,redis:service_started:true"
	cs := []Container{
		{Image: "web:1", Labels: map[string]string{LabelService: "web", LabelDependsOn: deps}},
		{Image: "web:1", Labels: map[string]string{LabelService: "web", LabelDependsOn: deps}},
		{Image: "pg:16", Labels: map[string]string{LabelService: "postgres", LabelDependsOn: ""}},
		{Image: "tool", Labels: map[string]string{LabelService: "tool", LabelOneOff: "True"}},
		{Image: "nolabel", Labels: map[string]string{}},
	}
	g := GraphFromContainers(cs)
	if g.Source != "containers" || len(g.Services) != 2 {
		t.Fatalf("graph = %+v", g)
	}
	pg, web := g.Services[0], g.Services[1]
	if pg.Name != "postgres" || web.Name != "web" || web.Image != "web:1" {
		t.Errorf("services = %+v", g.Services)
	}
	want := []model.ComposeGraphDep{{Service: "postgres", Condition: "service_healthy"}, {Service: "redis", Condition: "service_started"}}
	if !reflect.DeepEqual(web.DependsOn, want) {
		t.Errorf("deps = %+v", web.DependsOn)
	}
	if len(pg.DependsOn) != 0 || len(web.Ports) != 0 || len(web.Volumes) != 0 {
		t.Error("expected no deps/ports/volumes")
	}
	out, _ := json.Marshal(g)
	if strings.Contains(string(out), "null") {
		t.Errorf("graph JSON has null: %s", out)
	}
}
