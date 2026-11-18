package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"docklet/internal/model"
)

func editedSpec() model.ContainerCreateSpec {
	return model.ContainerCreateSpec{
		Name: "web", Image: "nginx:1.27-alpine", AutoStart: true, RestartPolicy: "always",
		Env: []model.CreateEnvVar{
			{Key: "MODE", Value: "green"},
			{Key: "DB_PASSWORD", Keep: true, Value: "ignored"},
		},
		Ports:  []model.PortMapping{{HostPort: 9090, ContainerPort: 80, Protocol: "tcp"}},
		Mounts: []model.MountSpec{{Type: "volume", Source: "data", Target: "/data"}},
	}
}

func runRecreate(t *testing.T, f *dockerFake, spec model.ContainerCreateSpec, opts model.RecreateOptions) model.Operation {
	t.Helper()
	s, m := newRecreateSvc(t, f)
	op, err := s.Recreate(context.Background(), "c1", oldContainerID, spec, opts)
	if err != nil {
		t.Fatal(err)
	}
	return waitOp(t, m, op.ID)
}

func TestRecreate_SwapsNamesInSafeOrder_WhenEverythingSucceeds(t *testing.T) {
	f := &dockerFake{}
	op := runRecreate(t, f, editedSpec(), model.RecreateOptions{})
	if op.Status != model.OpSucceeded {
		t.Fatalf("op %+v", op)
	}
	steps := []string{
		"POST /containers/create?name=web-docklet-new",
		"POST /containers/" + oldContainerID + "/stop",
		"POST /containers/" + oldContainerID + "/rename?name=web-docklet-old-oldid1234567",
		"POST /containers/newid1234567890abcdef/rename?name=web",
		"POST /containers/newid1234567890abcdef/start",
		"DELETE /containers/" + oldContainerID,
	}
	last := -1
	for _, s := range steps {
		i := f.index(s)
		if i <= last {
			t.Fatalf("step %q out of order or missing (at %d, previous %d): %v", s, i, last, f.sequence())
		}
		last = i
	}
	for _, c := range f.sequence() {
		if strings.Contains(c, "v=1") || strings.Contains(c, "volumes") {
			t.Errorf("old container removal must never remove volumes: %s", c)
		}
	}
}

func TestRecreate_CarriesUnexposedSettingsAndKeptSecret_WhenSpecEditsOtherFields(t *testing.T) {
	f := &dockerFake{}
	runRecreate(t, f, editedSpec(), model.RecreateOptions{})
	var body struct {
		Env        []string
		Labels     map[string]string
		HostConfig struct {
			Privileged   bool
			PortBindings map[string][]struct{ HostPort string }
		}
	}
	if err := json.Unmarshal([]byte(f.createBody), &body); err != nil {
		t.Fatal(err)
	}
	env := strings.Join(body.Env, ",")
	if !strings.Contains(env, "DB_PASSWORD=hunter2") || strings.Contains(env, "ignored") {
		t.Errorf("kept secret must come from the old container: %s", env)
	}
	if !strings.Contains(env, "MODE=green") || strings.Contains(env, "MODE=blue") {
		t.Errorf("typed value must replace: %s", env)
	}
	if !body.HostConfig.Privileged {
		t.Error("settings the form does not expose must be carried over")
	}
	if got := body.HostConfig.PortBindings["80/tcp"]; len(got) != 1 || got[0].HostPort != "9090" {
		t.Errorf("edited port not applied: %+v", got)
	}
}

func TestRecreate_RollsBackToOriginal_WhenStartOfNewContainerFails(t *testing.T) {
	f := &dockerFake{failOn: map[string]string{"POST /containers/newid1234567890abcdef/start": "driver failed: port is already allocated"}}
	op := runRecreate(t, f, editedSpec(), model.RecreateOptions{})
	if op.Status != model.OpFailed || !strings.Contains(op.Error, "already allocated") {
		t.Fatalf("op %+v", op)
	}
	rmNew := f.index("DELETE /containers/newid1234567890abcdef")
	back := f.indexExact("POST /containers/" + oldContainerID + "/rename?name=web")
	restart := f.index("POST /containers/" + oldContainerID + "/start")
	if rmNew < 0 || back < rmNew || restart < back {
		t.Fatalf("want remove new -> rename old back -> restart old, got %v", f.sequence())
	}
	if f.index("DELETE /containers/"+oldContainerID) >= 0 {
		t.Fatal("the original must never be removed after a failed start")
	}
}

func TestRecreate_TouchesNothing_WhenCreateOfNewContainerFails(t *testing.T) {
	f := &dockerFake{failOn: map[string]string{"POST /containers/create": "invalid mount config"}}
	op := runRecreate(t, f, editedSpec(), model.RecreateOptions{})
	if op.Status != model.OpFailed {
		t.Fatalf("op %+v", op)
	}
	for _, c := range f.sequence() {
		if strings.Contains(c, oldContainerID) && !strings.HasPrefix(c, "GET") {
			t.Fatalf("original touched after failed create: %s", c)
		}
	}
}

func TestRecreate_Refuses_WhenComposeContainerIsReplacedInPlace(t *testing.T) {
	f := &dockerFake{compose: "shop"}
	s, _ := newRecreateSvc(t, f)
	_, err := s.Recreate(context.Background(), "c1", oldContainerID, editedSpec(), model.RecreateOptions{})
	if err == nil || !strings.Contains(err.Error(), "Compose") {
		t.Fatalf("want compose refusal, got %v", err)
	}
	if f.index("POST") >= 0 {
		t.Fatal("nothing may be changed")
	}
}

func TestRecreate_LeavesOriginalUntouched_WhenCopyingComposeContainer(t *testing.T) {
	f := &dockerFake{compose: "shop"}
	spec := editedSpec()
	spec.Name = "web-copy"
	op := runRecreate(t, f, spec, model.RecreateOptions{KeepOriginal: true})
	if op.Status != model.OpSucceeded {
		t.Fatalf("op %+v", op)
	}
	for _, c := range f.sequence() {
		if strings.Contains(c, oldContainerID) && !strings.HasPrefix(c, "GET") {
			t.Fatalf("original touched: %s", c)
		}
	}
	if strings.Contains(f.createBody, "com.docker.compose") {
		t.Error("the standalone copy must not carry Compose labels")
	}
}

func TestRecreate_FailsBeforeAnyChange_WhenKeptEnvNoLongerExists(t *testing.T) {
	f := &dockerFake{}
	spec := editedSpec()
	spec.Env = append(spec.Env, model.CreateEnvVar{Key: "GONE_TOKEN", Keep: true})
	s, _ := newRecreateSvc(t, f)
	if _, err := s.Recreate(context.Background(), "c1", oldContainerID, spec, model.RecreateOptions{}); err == nil {
		t.Fatal("want error")
	}
	if f.index("POST") >= 0 {
		t.Fatal("nothing may be changed")
	}
}
