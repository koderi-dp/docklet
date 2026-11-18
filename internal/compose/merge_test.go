package compose

import (
	"testing"

	"docklet/internal/model"
)

func TestMerge_CombinesManagedRecordsWithDiscoveredProjects(t *testing.T) {
	env := SelectEnv(model.ConnectionProfile{Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/docker_engine"})
	disc := Discover("c1", []Container{
		{ID: "1", Name: "a-web-1", State: "running", Labels: map[string]string{LabelProject: "a", LabelService: "web", LabelConfigFile: "/old/a.yml"}},
		{ID: "2", Name: "b-web-1", State: "exited", Labels: map[string]string{LabelProject: "b", LabelService: "web", LabelConfigFile: "/b.yml"}},
	})
	recs := []ManagedRecord{
		{Name: "a", WorkingDir: `C:\a`, ConfigFiles: []string{`C:\a\compose.yml`}},
		{Name: "z", WorkingDir: `C:\z`, ConfigFiles: []string{`C:\z\compose.yml`, `C:\z\o.yml`}, EnvFiles: []string{`C:\z\.env`}},
	}
	got := Merge("c1", disc, recs, env)
	if len(got) != 3 || got[0].Name != "a" || got[1].Name != "b" || got[2].Name != "z" {
		t.Fatalf("%+v", got)
	}
	a, b, z := got[0], got[1], got[2]
	if !a.Managed || a.Source != model.ComposeSourceManagedLocal || a.NotCreated || a.Running != 1 || a.ConfigFiles[0] != `C:\a\compose.yml` || a.WorkingDir != `C:\a` {
		t.Errorf("managed+running: %+v", a)
	}
	if b.Managed || b.Source != model.ComposeSourceLabels || b.ConfigFiles[0] != "/b.yml" {
		t.Errorf("discovered only: %+v", b)
	}
	if !z.Managed || !z.NotCreated || z.Total != 0 || len(z.ConfigFiles) != 2 || z.EnvFiles[0] != `C:\z\.env` || z.Services == nil || z.ContainerIDs == nil {
		t.Errorf("not created: %+v", z)
	}
	wsl := SelectEnv(model.ConnectionProfile{Type: model.ConnWSL, WSLDistro: "Ubuntu"})
	if m := Merge("c1", nil, []ManagedRecord{{Name: "w", ConfigFiles: []string{"/home/c.yml"}}}, wsl); m[0].Source != model.ComposeSourceManagedWSL {
		t.Errorf("wsl source: %+v", m[0])
	}
	if m := Merge("c1", disc, nil, env); len(m) != 2 || m[0].Managed {
		t.Errorf("no records: %+v", m)
	}
}
