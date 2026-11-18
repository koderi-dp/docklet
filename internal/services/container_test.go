package services

import (
	"context"
	"encoding/binary"
	"net/http"
	"strings"
	"sync"
	"testing"

	"docklet/internal/model"
)

// newTestService serves a minimal Docker API: ids "run" (running), "stop" (exited),
// "boom" (start fails), anything else 404.
func newTestService(t *testing.T) (*ContainerService, *recEmitter, *[]string) {
	t.Helper()
	var mu sync.Mutex
	calls := []string{}
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if i := strings.Index(path, "/containers/"); i > 0 {
			path = path[i:] // strip version prefix
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case path == "/containers/json":
			_, _ = w.Write([]byte(`[
 {"Id":"a1","Names":["/web"],"Image":"nginx","State":"running","Status":"Up 2 hours (healthy)","Created":100,
  "Ports":[{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":8080,"Type":"tcp"},{"PrivatePort":443,"Type":"tcp"}],
  "Labels":{"com.docker.compose.project":"shop","com.docker.compose.service":"web","com.docker.compose.project.working_dir":"/srv/shop","com.docker.compose.project.config_files":"/srv/shop/compose.yml"}},
 {"Id":"b2","Names":["/db"],"Image":"pg","State":"running","Status":"Up 1 hour","Created":50,"Ports":[],"Labels":{}},
 {"Id":"c3","Names":["/bad"],"Image":"x","State":"running","Status":"Up 1 minute (unhealthy)","Created":60,"Ports":[],"Labels":{}}]`))
		case strings.HasSuffix(path, "/logs"):
			var b []byte
			for _, f := range []struct {
				t byte
				s string
			}{{1, "2024-01-01T00:00:00.123456789Z out one\n"}, {2, "2024-01-01T00:00:01Z err one\n"}} {
				h := make([]byte, 8)
				h[0] = f.t
				binary.BigEndian.PutUint32(h[4:], uint32(len(f.s)))
				b = append(append(b, h...), f.s...)
			}
			w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
			_, _ = w.Write(b)
		case strings.HasSuffix(path, "/json"):
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/containers/"), "/json")
			switch id {
			case "run", "boom", "stop":
				running := id == "run"
				status := "exited"
				if running {
					status = "running"
				}
				body := `{"Id":"` + id + `","Name":"/` + id + `","Created":"2024-01-01T00:00:00Z",
 "State":{"Status":"` + status + `","Running":` + map[bool]string{true: "true", false: "false"}[running] + `,"Health":{"Status":"healthy"}},
 "Config":{"Image":"img","Cmd":["a","b"],"Entrypoint":["/e"],"Tty":false,
  "Env":["PATH=/bin","DB_PASSWORD=hunter2","api_token=abc","MONKEY=1","PLAIN=ok"],"Labels":{"k":"v"}},
 "HostConfig":{"RestartPolicy":{"Name":"always"}},
 "Mounts":[{"Type":"bind","Source":"/h","Destination":"/c","RW":false}],
 "NetworkSettings":{"Networks":{"bridge":{}}}}`
				_, _ = w.Write([]byte(body))
			default:
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"message":"No such container: ` + id + `"}`))
			}
		case r.Method == http.MethodPost || r.Method == http.MethodDelete:
			mu.Lock()
			calls = append(calls, r.Method+" "+path)
			mu.Unlock()
			if strings.Contains(path, "/boom/") {
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"message":"driver failed: port is already allocated"}`))
				return
			}
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	}), "1.45")
	em := &recEmitter{}
	return &ContainerService{Engines: fakeProvider{cli}, Events: em}, em, &calls
}

func TestList_MapsComposeLabelsPortsAndHealth(t *testing.T) {
	s, _, _ := newTestService(t)
	got, err := s.List(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
	byName := map[string]model.Container{}
	for _, c := range got {
		byName[c.Name] = c
	}
	web := byName["web"]
	if web.ComposeProject != "shop" || web.ComposeService != "web" || web.ComposeWorkingDir != "/srv/shop" || web.ComposeConfigFiles != "/srv/shop/compose.yml" {
		t.Errorf("compose labels: %+v", web)
	}
	if web.Health != model.HealthHealthy || web.ConnectionID != "c1" {
		t.Errorf("web health/conn: %+v", web)
	}
	if len(web.Ports) != 2 || web.Ports[0].HostPort != 8080 || web.Ports[1].HostPort != 0 || web.Ports[1].ContainerPort != 443 {
		t.Errorf("ports: %+v", web.Ports)
	}
	if byName["db"].Health != model.HealthNone {
		t.Errorf("running without healthcheck must be none, got %q", byName["db"].Health)
	}
	if byName["bad"].Health != model.HealthUnhealthy {
		t.Errorf("bad health: %q", byName["bad"].Health)
	}
}

func TestContainerService_ErrorsForUnknownConnection(t *testing.T) {
	s, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := s.List(ctx, "nope"); err == nil {
		t.Error("List: want error")
	}
	if _, err := s.Start(ctx, "nope", []string{"run"}); err == nil {
		t.Error("Start: want error")
	}
	if _, err := s.Inspect(ctx, "nope", "run"); err == nil {
		t.Error("Inspect: want error")
	}
	if r, err := s.Start(ctx, "nope", nil); err != nil || len(r) != 0 {
		t.Errorf("empty ids: %v %v", r, err)
	}
}

func TestInspect_MasksSecretEnvValues(t *testing.T) {
	s, _, _ := newTestService(t)
	d, err := s.Inspect(context.Background(), "c1", "run")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		masked bool
		value  string
	}{
		"PATH": {false, "/bin"}, "DB_PASSWORD": {true, ""}, "api_token": {true, ""}, "MONKEY": {true, ""}, "PLAIN": {false, "ok"},
	}
	for _, e := range d.Env {
		w := want[e.Key]
		if e.Masked != w.masked || e.Value != w.value {
			t.Errorf("env %s: %+v", e.Key, e)
		}
	}
	if d.Command != "a b" || d.Entrypoint != "/e" || d.RestartPolicy != "always" || len(d.Mounts) != 1 || !d.Mounts[0].ReadOnly || d.Health != model.HealthHealthy {
		t.Errorf("detail: %+v", d)
	}
}

func TestStart_ReportsPerContainerOutcome(t *testing.T) {
	s, em, _ := newTestService(t)
	res, err := s.Start(context.Background(), "c1", []string{"run", "stop", "boom", "ghost"})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].Skipped || res[0].OK || res[0].Error != "" {
		t.Errorf("already running must be skipped: %+v", res[0])
	}
	if !res[1].OK || res[1].Skipped {
		t.Errorf("stopped container must start: %+v", res[1])
	}
	if res[2].OK || res[2].Skipped || !strings.Contains(res[2].Error, "port is already allocated") || !strings.Contains(res[2].Error, "already in use") {
		t.Errorf("engine failure must be friendly and keep the engine message: %+v", res[2])
	}
	if res[3].OK || !strings.Contains(res[3].Error, "not found") {
		t.Errorf("missing container: %+v", res[3])
	}
	if names := em.names(); len(names) != 1 || names[0] != model.EventContainersChanged {
		t.Errorf("one containers:changed event expected, got %v", names)
	}
}

func TestStop_SkipsAlreadyStoppedContainers(t *testing.T) {
	s, _, _ := newTestService(t)
	res, _ := s.Stop(context.Background(), "c1", []string{"stop", "run"})
	if !res[0].Skipped || !res[1].OK {
		t.Errorf("stop: %+v", res)
	}
}

func TestRemove_RefusesRunningContainerWithoutForce(t *testing.T) {
	s, _, calls := newTestService(t)
	res, _ := s.Remove(context.Background(), "c1", []string{"run", "stop"}, false)
	if !res[0].Skipped || res[0].Error == "" || !res[1].OK {
		t.Errorf("remove: %+v", res)
	}
	for _, c := range *calls {
		if strings.Contains(c, "run") {
			t.Errorf("running container must not reach the engine without force: %v", *calls)
		}
	}
}

func TestLogs_SplitsStdoutAndStderrFrames(t *testing.T) {
	s, _, _ := newTestService(t)
	lines, err := s.Logs(context.Background(), "c1", "run", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0].Stream != "stdout" || lines[0].Text != "out one" || lines[0].Timestamp != "2024-01-01T00:00:00.123456789Z" ||
		lines[1].Stream != "stderr" || lines[1].Text != "err one" {
		t.Errorf("lines: %+v", lines)
	}
}

func TestMaskInspectJSON(t *testing.T) {
	out, err := maskInspectJSON([]byte(`{"Config":{"Env":["PATH=/bin","DB_PASSWORD=hunter2"]},"Id":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "hunter2") || !strings.Contains(out, "DB_PASSWORD=********") || !strings.Contains(out, "PATH=/bin") {
		t.Errorf("masking: %s", out)
	}
}

func TestInspectRaw_MasksSecretEnvValues(t *testing.T) {
	s, _, _ := newTestService(t)
	raw, err := s.InspectRaw(context.Background(), "c1", "run")
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"hunter2", "api_token=abc"} {
		if strings.Contains(raw, leak) {
			t.Errorf("raw inspect leaks %q", leak)
		}
	}
	if !strings.Contains(raw, "PLAIN=ok") || !strings.Contains(raw, "DB_PASSWORD=") {
		t.Errorf("non-secret values must stay, secret keys stay visible: %s", raw)
	}
}
