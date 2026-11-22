package services

import (
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
	"docklet/internal/ops"
)

const applyConfigJSON = `{"services":{"web":{"image":"nginx"},"db":{"image":"postgres"},"debug":{"image":"busybox","profiles":["debug"]}}}`

func scriptApply(r *fakeRunner, hashOut string, hashErr error) {
	r.out = func(c compose.Command) (string, error) {
		switch {
		case slices.Contains(c.Args, "--short"):
			return "5.7.1\n", nil
		case slices.Contains(c.Args, "--hash"):
			return hashOut, hashErr
		case slices.Contains(c.Args, "config"):
			return applyConfigJSON, nil
		}
		return "", nil
	}
}

func TestApplyPlan_EveryServiceIsNewWithoutContainers(t *testing.T) {
	s, r, _ := newVerbSvc(t)
	scriptApply(r, "db h1\nweb h2\n", nil)
	plan, err := s.ApplyPlan(bg, "c1", "shop", []string{"p1"})
	if err != nil || plan.Error != "" || len(plan.Items) != 2 {
		t.Fatalf("%+v %v", plan, err)
	}
	for i, svc := range []string{"db", "web"} {
		if it := plan.Items[i]; it.Service != svc || it.Action != "new" {
			t.Errorf("item %d: %+v", i, it)
		}
	}
	// The hash run carries the chosen profiles; the structure run asks for all of them.
	hash := composeArgs(t, r, "--hash")
	if i := slices.Index(hash, "--profile"); i < 0 || hash[i+1] != "p1" {
		t.Errorf("hash argv: %v", hash)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	found := false
	for _, c := range r.cmds {
		if i := slices.Index(c.Args, "--profile"); i >= 0 && c.Args[i+1] == "*" && slices.Contains(c.Args, "json") {
			found = true
		}
	}
	if !found {
		t.Errorf("no json run with all profiles: %+v", r.cmds)
	}
}

func TestApplyPlan_FailingConfigIsAnErrorFieldNotAGoError(t *testing.T) {
	s, r, _ := newVerbSvc(t)
	scriptApply(r, "", errors.New("service \"web\" refers to undefined volume\nsecond line"))
	plan, err := s.ApplyPlan(bg, "c1", "shop", nil)
	if err != nil {
		t.Fatalf("Go error: %v", err)
	}
	if plan.Error != "service \"web\" refers to undefined volume\nsecond line" || len(plan.Items) != 0 {
		t.Errorf("%+v", plan)
	}
}

func TestPlanError_DropsPrefixAndWarnings(t *testing.T) {
	err := errors.New("exit status 1: time=\"2026-01-01T10:00:00Z\" level=warning msg=\"the attribute `version` is obsolete\"\n" +
		"WARN[0000] The \"X\" variable is not set\n" +
		"service \"web\" refers to undefined volume data: invalid compose project\n")
	if got := planError(err); got != `service "web" refers to undefined volume data: invalid compose project` {
		t.Errorf("got %q", got)
	}
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "line")
	}
	if got := planError(errors.New("exit status 2: " + strings.Join(lines, "\n"))); strings.Count(got, "\n") != 11 {
		t.Errorf("not capped at 12 lines: %q", got)
	}
	if got := planError(errors.New("level=warning only")); got != "level=warning only" {
		t.Errorf("got %q", got)
	}
}

// newEngineSvc is newVerbSvc with a fake Engine: containers and the image ids by platform.
func newEngineSvc(t *testing.T, containers string) (*ComposeService, *fakeRunner) {
	return newEngineSvcAt(t, containers, "1.49", "")
}

// newEngineSvcAt is newEngineSvc with the client's API version and the Engine's /info answer
// (empty: no /info route).
func newEngineSvcAt(t *testing.T, containers, api, info string) (*ComposeService, *fakeRunner) {
	t.Helper()
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/version"):
			_, _ = w.Write([]byte(`{"Os":"linux","Arch":"amd64","ApiVersion":"1.49"}`))
		case strings.HasSuffix(r.URL.Path, "/info") && info != "":
			_, _ = w.Write([]byte(info))
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			_, _ = w.Write([]byte(containers))
		case strings.Contains(r.URL.Path, "/images/") && r.URL.Query().Get("platform") != "":
			_, _ = w.Write([]byte(`{"Id":"sha256:platform"}`))
		case strings.Contains(r.URL.Path, "/images/") && api != "1.49":
			_, _ = w.Write([]byte(`{"Id":"sha256:current"}`))
		case strings.Contains(r.URL.Path, "/images/"):
			_, _ = w.Write([]byte(`{"Id":"sha256:index"}`))
		default:
			http.NotFound(w, r)
		}
	}), api)
	runner := &fakeRunner{}
	cfg := writeFile(t, t.TempDir(), "compose.yml")
	s := &ComposeService{
		Engines: fakeProvider{cli}, Events: &recEmitter{}, Ops: ops.New(nil), runner: runner,
		Projects: &fakeStore{recs: []appdb.ComposeProjectRecord{{ConnectionID: "c1", Name: "shop", WorkingDir: filepath.Dir(cfg), ConfigFiles: []string{cfg}}}},
		Profiles: fakeProfiles{"c1": {ID: "c1", Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:2375"}},
	}
	return s, runner
}

func webContainer(id, image string) string {
	return `{"Id":"` + id + `","Names":["/shop-web-` + id + `"],"Image":"nginx","State":"running","Labels":{` +
		`"com.docker.compose.project":"shop","com.docker.compose.service":"web",` +
		`"com.docker.compose.config-hash":"h2","com.docker.compose.image":"` + image + `"}}`
}

func TestApplyPlan_ImageLabelWithPlatformIdIsNotAChange(t *testing.T) {
	s, r := newEngineSvc(t, "["+webContainer("1", "sha256:platform")+"]")
	scriptApply(r, "db h1\nweb h2\n", nil)
	plan, err := s.ApplyPlan(bg, "c1", "shop", nil)
	if err != nil || plan.Error != "" || len(plan.Items) != 2 {
		t.Fatalf("%+v %v", plan, err)
	}
	if it := plan.Items[1]; it.Service != "web" || it.Action != "unchanged" {
		t.Errorf("web: %+v", it)
	}

	s, r = newEngineSvc(t, "["+webContainer("1", "sha256:old")+"]")
	scriptApply(r, "db h1\nweb h2\n", nil)
	plan, _ = s.ApplyPlan(bg, "c1", "shop", nil)
	if it := plan.Items[1]; it.Action != "recreateImage" {
		t.Errorf("old image: %+v", it)
	}
}

func TestApplyPlan_OldEngineChecksTheImageByPlainId(t *testing.T) {
	const classic = `{"DriverStatus":[["Backing Filesystem","extfs"]]}`
	const containerd = `{"DriverStatus":[["driver-type","io.containerd.snapshotter.v1"]]}`
	for name, c := range map[string]struct{ info, want string }{
		"classic store":    {classic, "recreateImage"},
		"containerd store": {containerd, "unchanged"},
		"info unknown":     {"", "unchanged"},
	} {
		s, r := newEngineSvcAt(t, "["+webContainer("1", "sha256:old")+"]", "1.47", c.info)
		scriptApply(r, "db h1\nweb h2\n", nil)
		plan, err := s.ApplyPlan(bg, "c1", "shop", nil)
		if err != nil || len(plan.Items) != 2 || plan.Items[1].Action != c.want {
			t.Errorf("%s: want %s, got %+v %v", name, c.want, plan, err)
		}
	}
}

func TestApplyPlan_OnlyADeclaredOneShotIsFinished(t *testing.T) {
	stopped := strings.Replace(webContainer("1", "sha256:platform"), `"State":"running"`, `"State":"exited","Status":"Exited (0) 1 hour ago"`, 1)
	for name, c := range map[string]struct {
		cfg  string
		shot bool
	}{
		"ordinary service":  {`{"services":{"web":{"image":"nginx"},"db":{"image":"postgres"}}}`, false},
		"declared one-shot": {`{"services":{"web":{"image":"nginx"},"db":{"image":"postgres","depends_on":{"web":{"condition":"service_completed_successfully"}}}}}`, true},
	} {
		s, r := newEngineSvc(t, "["+stopped+"]")
		r.out = func(cmd compose.Command) (string, error) {
			switch {
			case slices.Contains(cmd.Args, "--short"):
				return "5.7.1\n", nil
			case slices.Contains(cmd.Args, "--hash"):
				return "db h1\nweb h2\n", nil
			case slices.Contains(cmd.Args, "config"):
				return c.cfg, nil
			}
			return "", nil
		}
		plan, err := s.ApplyPlan(bg, "c1", "shop", nil)
		if err != nil || len(plan.Items) != 2 || plan.Items[1].Action != "start" || plan.Items[1].OneShot != c.shot {
			t.Errorf("%s: %+v %v", name, plan, err)
		}
	}
}

func TestApplyPlan_ChangedReplicasAreAScale(t *testing.T) {
	s, r := newEngineSvc(t, "["+webContainer("1", "sha256:platform")+"]")
	r.out = func(c compose.Command) (string, error) {
		switch {
		case slices.Contains(c.Args, "--short"):
			return "5.7.1\n", nil
		case slices.Contains(c.Args, "--hash"):
			return "web h2\n", nil
		case slices.Contains(c.Args, "config"):
			return `{"services":{"web":{"image":"nginx","deploy":{"replicas":3}}}}`, nil
		}
		return "", nil
	}
	plan, err := s.ApplyPlan(bg, "c1", "shop", nil)
	if err != nil || len(plan.Items) != 1 || plan.Items[0].Action != "scale" || plan.Items[0].Detail != "1 container now, 3 wanted" {
		t.Errorf("%+v %v", plan, err)
	}
}

func TestApplyPlan_ProjectThatCannotRunReportsWhy(t *testing.T) {
	s, r, _ := newVerbSvc(t)
	r.out = func(c compose.Command) (string, error) { return "", errors.New("docker: not found") }
	capab, err := s.Capability(bg, "c1", "shop")
	if err != nil || capab.CanRun || capab.Reason == "" {
		t.Fatalf("setup: %+v %v", capab, err)
	}
	plan, err := s.ApplyPlan(bg, "c1", "shop", nil)
	if err != nil || plan.Error != capab.Reason || len(plan.Items) != 0 {
		t.Errorf("%+v %v", plan, err)
	}
}
