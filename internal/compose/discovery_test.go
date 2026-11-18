package compose

import (
	"reflect"
	"testing"

	"docklet/internal/model"
)

func TestDiscover_GroupsByComposeLabelOnly(t *testing.T) {
	lab := func(proj, svc, cfg string, extra ...string) map[string]string {
		m := map[string]string{LabelProject: proj, LabelService: svc, LabelConfigFile: cfg, LabelWorkingDir: "/srv/" + proj}
		for i := 0; i+1 < len(extra); i += 2 {
			m[extra[i]] = extra[i+1]
		}
		return m
	}
	cs := []Container{
		{ID: "1", Name: "catalo-web-1", Image: "web", State: "running", Created: 10, Labels: lab("catalo", "web", "/srv/a.yml,/srv/b.yml")},
		{ID: "2", Name: "catalo-db-1", Image: "pg", State: "exited", Created: 11, Labels: lab("catalo", "db", "/srv/a.yml,/srv/b.yml", LabelEnvFile, "/srv/.env")},
		{ID: "3", Name: "catalo-run-1", Image: "web", State: "running", Created: 99, Labels: lab("catalo", "web", "/other.yml", LabelOneOff, "True")},
		{ID: "4", Name: "catalo-extra", Image: "x", State: "running", Labels: map[string]string{}}, // name prefix must not group
		{ID: "5", Name: "other-1", Image: "o", State: "running", Labels: lab("other", "o", "")},
	}
	ps := Discover("c1", cs)
	if len(ps) != 2 || ps[0].Name != "catalo" || ps[1].Name != "other" {
		t.Fatalf("%+v", ps)
	}
	p := ps[0]
	if p.Total != 3 || p.Running != 2 || p.Stopped != 1 || p.Source != model.ComposeSourceLabels {
		t.Errorf("counts %+v", p)
	}
	if !reflect.DeepEqual(p.ConfigFiles, []string{"/srv/a.yml", "/srv/b.yml"}) || p.WorkingDir != "/srv/catalo" || !reflect.DeepEqual(p.EnvFiles, []string{"/srv/.env"}) {
		t.Errorf("labels not taken from regular container: %+v", p)
	}
	if len(p.Services) != 2 || p.Services[1].Name != "web" || p.Services[1].Total != 2 || !p.Services[1].Containers[0].OneOff {
		t.Errorf("services %+v", p.Services)
	}
	if len(ps[1].ConfigFiles) != 0 {
		t.Errorf("empty label should give no files: %+v", ps[1])
	}
}

func TestDiscover_OneShotContainersAreCompletedNotStopped(t *testing.T) {
	zero, two := 0, 2
	l := func(extra ...string) map[string]string {
		m := map[string]string{LabelProject: "catalo"}
		for i := 0; i+1 < len(extra); i += 2 {
			m[extra[i]] = extra[i+1]
		}
		return m
	}
	cs := []Container{
		{ID: "1", State: "running", Labels: l()},
		{ID: "2", State: "exited", ExitCode: &zero, RestartPolicy: "no", Labels: l()},     // migrate: completed
		{ID: "3", State: "exited", ExitCode: &zero, RestartPolicy: "always", Labels: l()}, // service that stopped: stopped
		{ID: "4", State: "exited", ExitCode: &two, RestartPolicy: "no", Labels: l()},      // failed
		{ID: "5", State: "exited", ExitCode: &zero, Labels: l(LabelOneOff, "True")},       // compose run: completed
		{ID: "6", State: "exited", ExitCode: nil, RestartPolicy: "no", Labels: l()},       // unknown code: stopped
	}
	p := Discover("c", cs)[0]
	if p.Running != 1 || p.Completed != 2 || p.Failed != 1 || p.Stopped != 3 {
		t.Fatalf("running=%d completed=%d failed=%d stopped=%d", p.Running, p.Completed, p.Failed, p.Stopped)
	}
}
