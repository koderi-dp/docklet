package services

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/registryauth"
)

const shopContainer = `[{"Id":"abc","Names":["/shop-web-1"],"Image":"x","State":"running",
 "Labels":{"com.docker.compose.project":"shop","com.docker.compose.service":"web"}}]`

// newCloneSvc has one saved project "shop" on connection c1: a folder with compose.yml, a .env and
// a nested source file, plus an env file outside the folder. containers is the Engine's answer
// to every container list ("[]" for a project that was never started).
func newCloneSvc(t *testing.T, containers string) (s *ComposeService, runner *fakeRunner, m *ops.Manager, st *fakeStore, dir, outside string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "shop")
	if err := os.MkdirAll(filepath.Join(dir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := writeFile(t, dir, "compose.yml")
	writeFile(t, dir, ".env")
	writeFile(t, filepath.Join(dir, "web"), "app.js")
	outside = writeFile(t, t.TempDir(), "shared.env")
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/containers/json") && strings.Contains(r.URL.RawQuery, "shop") && !strings.Contains(r.URL.RawQuery, "shop-copy") && !strings.Contains(r.URL.RawQuery, "store") {
			_, _ = w.Write([]byte(containers))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}), "1.45")
	runner = &fakeRunner{}
	m = ops.New(nil)
	st = &fakeStore{recs: []appdb.ComposeProjectRecord{
		{ConnectionID: "c1", Name: "shop", WorkingDir: dir, ConfigFiles: []string{cfg}, EnvFiles: []string{outside}},
		{ConnectionID: "c1", Name: "taken", WorkingDir: dir, ConfigFiles: []string{cfg}},
	}}
	s = &ComposeService{
		Engines: fakeProvider{cli}, Events: &recEmitter{}, Ops: m, runner: runner, Projects: st,
		Profiles: fakeProfiles{"c1": {ID: "c1", Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:2375"}},
		Registry: &registryauth.Resolver{ConfigDir: t.TempDir()},
	}
	return
}

func TestRenameProject_MovesTheSavedEntryToTheNewName(t *testing.T) {
	s, _, _, st, dir, _ := newCloneSvc(t, `[]`)
	p, err := s.RenameProject(bg, "c1", "shop", "store")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "store" || p.WorkingDir != dir || !p.Managed {
		t.Errorf("project: %+v", p)
	}
	last := st.recs[len(st.recs)-1]
	if last.Name != "store" || last.WorkingDir != dir || len(last.ConfigFiles) != 1 || len(last.EnvFiles) != 1 {
		t.Errorf("saved: %+v", last)
	}
	if !slices.Contains(st.deleted, "c1/shop") {
		t.Errorf("old entry not removed: %v", st.deleted)
	}
}

func TestRenameProject_RefusesWhatWouldOrphanOrOverwrite(t *testing.T) {
	for _, tc := range []struct{ name, containers, from, to, want string }{
		{"project has containers", shopContainer, "shop", "store", "has containers"},
		{"name already saved", `[]`, "shop", "taken", "already"},
		{"same name", `[]`, "shop", "shop", "same name"},
		{"invalid name", `[]`, "shop", "My Shop", "invalid Compose project name"},
		{"unknown project", `[]`, "nope", "store", "not found"},
	} {
		s, _, _, st, _, _ := newCloneSvc(t, tc.containers)
		before := len(st.recs)
		if _, err := s.RenameProject(bg, "c1", tc.from, tc.to); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
		if len(st.recs) != before || len(st.deleted) != 0 {
			t.Errorf("%s: the store changed", tc.name)
		}
	}
}

func TestCloneProject_CopiesTheFolderAndSavesTheCopyUnderTheNewName(t *testing.T) {
	// The original may be running: a clone never touches it.
	s, _, m, st, dir, outside := newCloneSvc(t, shopContainer)
	dst := filepath.Join(filepath.Dir(dir), "shop-copy")
	op, err := s.CloneProject(bg, "c1", "shop", "shop-copy", dst)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded {
		t.Fatalf("operation: %+v", done)
	}
	for _, rel := range []string{"compose.yml", ".env", filepath.Join("web", "app.js")} {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Errorf("%s not copied: %v", rel, err)
		}
	}
	last := st.recs[len(st.recs)-1]
	if last.Name != "shop-copy" || last.WorkingDir != dst || len(last.ConfigFiles) != 1 || last.ConfigFiles[0] != filepath.Join(dst, "compose.yml") {
		t.Errorf("saved: %+v", last)
	}
	// A file outside the folder is not copied: the clone keeps using the original.
	if len(last.EnvFiles) != 1 || last.EnvFiles[0] != outside {
		t.Errorf("env files: %v", last.EnvFiles)
	}
	if len(st.deleted) != 0 {
		t.Errorf("the original was touched: %v", st.deleted)
	}
}

func TestCloneProject_RemovesTheCopyWhenComposeRejectsIt(t *testing.T) {
	s, runner, m, st, dir, _ := newCloneSvc(t, `[]`)
	runner.out = func(c compose.Command) (string, error) {
		if slices.Contains(c.Args, "-q") && slices.Contains(c.Args, "shop-copy") {
			return "", errors.New("exit status 15: services.web.build.context not found")
		}
		return "", nil
	}
	dst := filepath.Join(filepath.Dir(dir), "shop-copy")
	before := len(st.recs)
	op, err := s.CloneProject(bg, "c1", "shop", "shop-copy", dst)
	if err != nil {
		t.Fatal(err)
	}
	done := waitOp(t, m, op.ID)
	if done.Status != model.OpFailed || !strings.Contains(done.Error, "context not found") {
		t.Errorf("operation: %+v", done)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("the rejected copy was left on disk")
	}
	if len(st.recs) != before {
		t.Error("a rejected copy was saved")
	}
}

func TestCloneProject_RefusesABadNameOrDestinationBeforeCopying(t *testing.T) {
	s, _, m, _, dir, _ := newCloneSvc(t, `[]`)
	full := t.TempDir()
	writeFile(t, full, "keep.txt")
	sibling := filepath.Join(filepath.Dir(dir), "x")
	for _, tc := range []struct{ name, newName, dst, want string }{
		{"taken name", "taken", sibling, "already"},
		{"same name", "shop", sibling, "already"},
		{"invalid name", "Shop Copy", sibling, "invalid Compose project name"},
		{"no folder", "shop-copy", "  ", "choose a folder"},
		{"into itself", "shop-copy", filepath.Join(dir, "copy"), "inside"},
		{"onto itself", "shop-copy", dir, "same folder"},
		{"folder with content", "shop-copy", full, "not empty"},
		{"relative", "shop-copy", "copy", "absolute"},
	} {
		if _, err := s.CloneProject(bg, "c1", "shop", tc.newName, tc.dst); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
	if n := len(m.List()); n != 0 {
		t.Errorf("%d operations started", n)
	}
}

func TestClonePreview_SuggestsANameAndFolderAndReportsWhatWillCollide(t *testing.T) {
	s, runner, _, _, dir, outside := newCloneSvc(t, shopContainer)
	runner.out = func(c compose.Command) (string, error) {
		if slices.Contains(c.Args, "json") {
			return `{"name":"shop","services":{"web":{"image":"x","ports":[{"target":80,"published":"8080"}]}}}`, nil
		}
		return "", nil
	}
	pv, err := s.ClonePreview(bg, "c1", "shop")
	if err != nil {
		t.Fatal(err)
	}
	if pv.SuggestedName != "shop-copy" || pv.SuggestedDir != filepath.Join(filepath.Dir(dir), "shop-copy") || pv.SourceDir != dir {
		t.Errorf("suggestions: %+v", pv)
	}
	if !pv.SizeKnown || pv.Files != 3 || pv.SizeBytes == 0 {
		t.Errorf("size: %+v", pv)
	}
	if !pv.Running {
		t.Error("the running original was not reported")
	}
	if len(pv.Outside) != 1 || pv.Outside[0] != outside {
		t.Errorf("outside: %v", pv.Outside)
	}
	if len(pv.Collisions) != 1 || !strings.Contains(pv.Collisions[0], "8080") {
		t.Errorf("collisions: %v", pv.Collisions)
	}
}

func TestClonePreview_SkipsANameAndFolderThatAreTaken(t *testing.T) {
	s, _, _, st, dir, _ := newCloneSvc(t, `[]`)
	st.recs = append(st.recs, appdb.ComposeProjectRecord{ConnectionID: "c1", Name: "shop-copy", WorkingDir: dir, ConfigFiles: []string{"x"}})
	if err := os.Mkdir(filepath.Join(filepath.Dir(dir), "shop-copy"), 0o755); err != nil {
		t.Fatal(err)
	}
	pv, err := s.ClonePreview(bg, "c1", "shop")
	if err != nil || pv.SuggestedName != "shop-copy-2" || pv.SuggestedDir != filepath.Join(filepath.Dir(dir), "shop-copy-2") {
		t.Errorf("got %+v, %v", pv, err)
	}
}
