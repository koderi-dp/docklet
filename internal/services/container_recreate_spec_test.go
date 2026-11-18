package services

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"

	"docklet/internal/model"
)

func recreateInfo(t *testing.T, f *dockerFake) model.ContainerRecreateInfo {
	t.Helper()
	s, _ := newRecreateSvc(t, f)
	info, err := s.RecreateSpec(context.Background(), "c1", oldContainerID)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestRecreateSpec_OmitsImageDefaultsAndMasksSecrets_WhenMappingContainer(t *testing.T) {
	info := recreateInfo(t, &dockerFake{})
	sp := info.Spec
	if sp.Image != "nginx:1.27-alpine" || sp.Name != "web" || !sp.AutoStart || !info.Running {
		t.Fatalf("basics: %+v", sp)
	}
	if sp.Command != "" || sp.Entrypoint != "" {
		t.Errorf("command equal to the image's must be omitted: %q %q", sp.Command, sp.Entrypoint)
	}
	var keys []string
	for _, e := range sp.Env {
		keys = append(keys, e.Key)
	}
	if !slices.Equal(keys, []string{"TZ", "DB_PASSWORD", "MODE"}) {
		t.Errorf("image env PATH must be dropped, got %v", keys)
	}
	if sp.Hostname != "" {
		t.Errorf("default hostname must be omitted: %q", sp.Hostname)
	}
	if len(sp.Ports) != 1 || sp.Ports[0].HostPort != 8080 || sp.Ports[0].ContainerPort != 80 {
		t.Errorf("ports: %+v", sp.Ports)
	}
	if sp.RestartPolicy != "always" || sp.NetworkMode != "" {
		t.Errorf("restart/network: %q %q", sp.RestartPolicy, sp.NetworkMode)
	}
}

func TestRecreateSpec_NeverReturnsSecretValue(t *testing.T) {
	info := recreateInfo(t, &dockerFake{})
	b, _ := json.Marshal(info)
	if strings.Contains(string(b), "hunter2") {
		t.Fatal("secret value leaked to the UI payload")
	}
	for _, e := range info.Spec.Env {
		if e.Key == "DB_PASSWORD" && (!e.Keep || e.Value != "") {
			t.Errorf("secret must be masked with Keep: %+v", e)
		}
		if e.Key == "MODE" && (e.Keep || e.Value != "blue") {
			t.Errorf("plain env keeps its value: %+v", e)
		}
	}
}

func TestRecreateSpec_SplitsNamedAndAnonymousVolumes_WhenMappingMounts(t *testing.T) {
	info := recreateInfo(t, &dockerFake{})
	if len(info.Spec.Mounts) != 2 {
		t.Fatalf("mounts: %+v", info.Spec.Mounts)
	}
	byTarget := map[string]model.MountSpec{}
	for _, m := range info.Spec.Mounts {
		byTarget[m.Target] = m
	}
	if m := byTarget["/data"]; m.Type != "volume" || m.Source != "data" || m.ReadOnly {
		t.Errorf("named volume: %+v", m)
	}
	if m := byTarget["/c"]; m.Type != "bind" || m.Source != "/h" || !m.ReadOnly {
		t.Errorf("read-only bind: %+v", m)
	}
	if len(info.AnonymousVolumes) != 1 || info.AnonymousVolumes[0].Target != "/anon" || len(info.Warnings) == 0 {
		t.Errorf("anonymous volume must be reported with a warning: %+v %v", info.AnonymousVolumes, info.Warnings)
	}
}

func TestRecreateSpec_FlagsComposeAndDropsComposeLabels_WhenContainerIsComposeManaged(t *testing.T) {
	info := recreateInfo(t, &dockerFake{compose: "shop"})
	if !info.ComposeManaged || info.ComposeProject != "shop" {
		t.Fatalf("compose: %+v", info)
	}
	for k := range info.Spec.Labels {
		if strings.HasPrefix(k, "com.docker.compose.") {
			t.Errorf("compose label %s must not be in the editable spec", k)
		}
	}
	if info.Spec.Labels["owner"] != "me" {
		t.Errorf("user labels kept: %v", info.Spec.Labels)
	}
	if plain := recreateInfo(t, &dockerFake{}); plain.ComposeManaged {
		t.Error("standalone container flagged as compose")
	}
}

func TestQuoteArgv_RoundTripsThroughSplitCommand(t *testing.T) {
	argv := []string{"sh", "-c", "echo 'hi there' && exit 0", "", "a\"b"}
	got, err := splitCommand(quoteArgv(argv))
	if err != nil || !slices.Equal(got, argv) {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestBuildRecreateInfo_KeepsCommandOnlyWhenItDiffersFromImage(t *testing.T) {
	in := containerWithConfig(`{"Image":"x","Entrypoint":["/e"],"Cmd":["a","b c"]}`)
	img := imageWithConfig(`{"Entrypoint":["/e"],"Cmd":["a","b c"]}`)
	if info := buildRecreateInfo(in, img); info.Spec.Command != "" || info.Spec.Entrypoint != "" {
		t.Fatalf("defaults must be omitted: %+v", info.Spec)
	}
	img = imageWithConfig(`{"Entrypoint":["/e"],"Cmd":["z"]}`)
	if info := buildRecreateInfo(in, img); info.Spec.Command != "a 'b c'" || info.Spec.Entrypoint != "" {
		t.Fatalf("cmd override: %q / %q", info.Spec.Entrypoint, info.Spec.Command)
	}
	img = imageWithConfig(`{"Entrypoint":["/other"],"Cmd":["a","b c"]}`)
	if info := buildRecreateInfo(in, img); info.Spec.Entrypoint != "/e" || info.Spec.Command != "a 'b c'" {
		t.Fatalf("an entrypoint override keeps the command explicit: %q / %q", info.Spec.Entrypoint, info.Spec.Command)
	}
}

func containerWithConfig(cfg string) container.InspectResponse {
	var in container.InspectResponse
	_ = json.Unmarshal([]byte(`{"Id":"abc","Name":"/n","Config":`+cfg+`}`), &in)
	return in
}

func imageWithConfig(cfg string) *image.InspectResponse {
	var img image.InspectResponse
	_ = json.Unmarshal([]byte(`{"Config":`+cfg+`}`), &img)
	return &img
}
