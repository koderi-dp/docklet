package services

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
)

type projFiles struct {
	s       *ComposeService
	cfg     string
	envFile string
}

// newProjFilesSvc builds a managed project whose files are real files in a temp dir.
func newProjFilesSvc(t *testing.T, conn model.ConnectionProfile, withConfig bool, content []byte) projFiles {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(cfg, content, 0o600); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := appdb.ComposeProjectRecord{ConnectionID: "c1", Name: "shop", WorkingDir: dir, EnvFiles: []string{envFile}}
	if withConfig {
		rec.ConfigFiles = []string{cfg}
	}
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}), "1.45")
	conn.ID = "c1"
	s := &ComposeService{
		Engines: fakeProvider{cli}, Events: &recEmitter{}, runner: &fakeRunner{},
		Projects: &fakeStore{recs: []appdb.ComposeProjectRecord{rec}},
		Profiles: fakeProfiles{"c1": conn},
	}
	return projFiles{s: s, cfg: cfg, envFile: envFile}
}

var localConn = model.ConnectionProfile{Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:2375"}

func readDisk(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestProjectFiles_ListsConfigThenEnvAndExplainsWhyNot(t *testing.T) {
	f := newProjFilesSvc(t, localConn, true, []byte("services: {}\n"))
	l, err := f.s.ProjectFiles(bg, "c1", "shop")
	if err != nil || !l.CanEdit || len(l.Files) != 2 ||
		l.Files[0] != (model.ComposeFileRef{Path: f.cfg, Kind: "config"}) ||
		l.Files[1] != (model.ComposeFileRef{Path: f.envFile, Kind: "env"}) {
		t.Fatalf("%+v %v", l, err)
	}

	f = newProjFilesSvc(t, model.ConnectionProfile{Type: model.ConnectionType("bogus")}, true, []byte("a: 1\n"))
	l, err = f.s.ProjectFiles(bg, "c1", "shop")
	if err != nil || l.CanEdit || !strings.Contains(l.Reason, "not supported") {
		t.Errorf("no environment: %+v %v", l, err)
	}

	f = newProjFilesSvc(t, localConn, false, []byte("a: 1\n"))
	l, err = f.s.ProjectFiles(bg, "c1", "shop")
	if err != nil || l.CanEdit || !strings.Contains(l.Reason, "config-file label") {
		t.Errorf("no config files: %+v %v", l, err)
	}
}

func TestReadProjectFile_ReturnsLFTextVersionAndReadOnly(t *testing.T) {
	raw := []byte("\xEF\xBB\xBFservices:\r\n  web: {}\r\n")
	f := newProjFilesSvc(t, localConn, true, raw)
	c, err := f.s.ReadProjectFile(bg, "c1", "shop", f.cfg)
	if err != nil || c.Text != "services:\n  web: {}\n" || c.Version != compose.FileVersion(raw) || c.ReadOnly {
		t.Fatalf("config: %+v %v", c, err)
	}
	c, err = f.s.ReadProjectFile(bg, "c1", "shop", f.envFile)
	if err != nil || c.Text != "A=1\n" || !c.ReadOnly {
		t.Errorf("env: %+v %v", c, err)
	}

	other := filepath.Join(filepath.Dir(f.cfg), "other.yml")
	if err := os.WriteFile(other, []byte("secret: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{other, filepath.Dir(f.cfg) + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(filepath.Dir(f.cfg)) + string(filepath.Separator) + "compose.yml", ""} {
		if c, err := f.s.ReadProjectFile(bg, "c1", "shop", p); err == nil || c.Text != "" {
			t.Errorf("%q: %+v %v", p, c, err)
		}
	}
}

func TestSaveProjectFile_WritesKeepingStyleAndChecksVersion(t *testing.T) {
	raw := []byte("\xEF\xBB\xBFservices:\r\n  web: {}\r\n")
	f := newProjFilesSvc(t, localConn, true, raw)
	read, _ := f.s.ReadProjectFile(bg, "c1", "shop", f.cfg)

	res, err := f.s.SaveProjectFile(bg, "c1", "shop", f.cfg, "services:\n  web: {}\n  db: {}\n", read.Version, false)
	want := "\xEF\xBB\xBFservices:\r\n  web: {}\r\n  db: {}\r\n"
	if err != nil || !res.Saved || res.Conflict || res.Error != nil || readDisk(t, f.cfg) != want {
		t.Fatalf("%+v %v %q", res, err, readDisk(t, f.cfg))
	}
	fresh, _ := f.s.ReadProjectFile(bg, "c1", "shop", f.cfg)
	if res.Version != fresh.Version {
		t.Errorf("version %s, fresh read %s", res.Version, fresh.Version)
	}

	// Invalid YAML: reported with its line, nothing written.
	res, err = f.s.SaveProjectFile(bg, "c1", "shop", f.cfg, "a: 1\nb: [\n", fresh.Version, true)
	if err != nil || res.Saved || res.Error == nil || res.Error.Line < 1 || readDisk(t, f.cfg) != want {
		t.Errorf("invalid yaml: %+v %v", res, err)
	}

	// Changed on disk after the read: conflict, then overwrite wins.
	changed := "services: {x: {}}\n"
	if err := os.WriteFile(f.cfg, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err = f.s.SaveProjectFile(bg, "c1", "shop", f.cfg, "services: {y: {}}\n", fresh.Version, false)
	if err != nil || res.Saved || !res.Conflict || readDisk(t, f.cfg) != changed {
		t.Errorf("conflict: %+v %v", res, err)
	}
	res, err = f.s.SaveProjectFile(bg, "c1", "shop", f.cfg, "services: {y: {}}\n", fresh.Version, true)
	if err != nil || !res.Saved || readDisk(t, f.cfg) != "services: {y: {}}\n" {
		t.Errorf("overwrite: %+v %v", res, err)
	}
}

func TestSaveProjectFile_RefusesEnvAndForeignPaths(t *testing.T) {
	f := newProjFilesSvc(t, localConn, true, []byte("services: {}\n"))
	other := filepath.Join(filepath.Dir(f.cfg), "other.yml")
	if err := os.WriteFile(other, []byte("a: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{f.envFile, other} {
		res, err := f.s.SaveProjectFile(bg, "c1", "shop", p, "a: 2\n", "", true)
		if err == nil || res.Saved {
			t.Errorf("%s: %+v %v", p, res, err)
		}
	}
	if readDisk(t, f.envFile) != "A=1\n" || readDisk(t, other) != "a: 1\n" {
		t.Error("a refused file was written")
	}
}

func TestFileOutline_ListsServices(t *testing.T) {
	o := (&ComposeService{}).FileOutline("services:\n  web: {}\n")
	if o.Error != nil || len(o.Services) != 1 || o.Services[0].Name != "web" || o.Services[0].Line != 2 {
		t.Errorf("%+v", o)
	}
}
