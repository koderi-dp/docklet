package services

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
)

// createSvc is a service on connection "c1" (local) and "wsl" whose engine lists the given containers.
func createSvc(t *testing.T, r *fakeRunner, st ComposeProjectStore, containersJSON string) *ComposeService {
	t.Helper()
	if containersJSON == "" {
		containersJSON = `[]`
	}
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(containersJSON))
	}), "1.45")
	return &ComposeService{
		Engines: fakeProvider{cli}, Events: &recEmitter{}, runner: r, Projects: st,
		Profiles: fakeProfiles{"c1": localConn, "wsl": {ID: "wsl", Type: model.ConnWSL, WSLDistro: "Ubuntu"}},
	}
}

const newFileText = "name: shop\nservices:\n  app:\n    image: nginx:alpine\n"

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestCreateProject_WritesFileAndSavesRecord(t *testing.T) {
	r, st := &fakeRunner{}, &fakeStore{}
	dir := filepath.Join(t.TempDir(), "apps", "shop")
	res, err := createSvc(t, r, st, "").CreateProject(context.Background(), "c1", dir, " shop ", newFileText)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "compose.yaml")
	if got := readDisk(t, file); got != newFileText {
		t.Fatalf("file content %q", got)
	}
	if len(st.recs) != 1 {
		t.Fatalf("records: %+v", st.recs)
	}
	rec := st.recs[0]
	if rec.ConnectionID != "c1" || rec.Name != "shop" || rec.WorkingDir != dir || len(rec.ConfigFiles) != 1 || rec.ConfigFiles[0] != file {
		t.Fatalf("record: %+v", rec)
	}
	if res.Project == nil || res.Project.Name != "shop" || res.ExistingFile != "" {
		t.Fatalf("result: %+v", res)
	}
	validated := false
	for _, c := range r.cmds {
		args := strings.Join(c.Args, " ")
		if strings.Contains(args, "config") && strings.Contains(args, "-q") && strings.Contains(args, "-p shop") && strings.Contains(args, file) {
			validated = true
		}
		for _, a := range c.Args {
			if a == "up" || a == "start" || a == "create" {
				t.Fatalf("creating must not start anything: %v", c.Args)
			}
		}
	}
	if !validated {
		t.Fatalf("the file was not validated with config -q: %+v", r.cmds)
	}
}

func TestCreateProject_ExistingComposeFileIsReported(t *testing.T) {
	for _, name := range compose.ComposeFileNames {
		r, st := &fakeRunner{}, &fakeStore{}
		dir := t.TempDir()
		existing := filepath.Join(dir, name)
		if err := os.WriteFile(existing, []byte("old content\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		res, err := createSvc(t, r, st, "").CreateProject(context.Background(), "c1", dir, "shop", newFileText)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.ExistingFile != existing || res.Project != nil {
			t.Errorf("%s: result %+v", name, res)
		}
		if len(st.recs) != 0 {
			t.Errorf("%s: a record was saved", name)
		}
		if got := readDisk(t, existing); got != "old content\n" {
			t.Errorf("%s: the existing file changed: %q", name, got)
		}
		if got := dirEntries(t, dir); len(got) != 1 {
			t.Errorf("%s: files were added: %v", name, got)
		}
	}
}

func TestCreateProject_RefusesSavedNameButAllowsDiscovered(t *testing.T) {
	ctx := context.Background()
	st := &fakeStore{recs: []appdb.ComposeProjectRecord{{ConnectionID: "c1", Name: "shop", WorkingDir: "/x", ConfigFiles: []string{"/x/compose.yaml"}}}}
	dir := filepath.Join(t.TempDir(), "shop")
	_, err := createSvc(t, &fakeRunner{}, st, "").CreateProject(ctx, "c1", dir, "shop", newFileText)
	if err == nil || !strings.Contains(err.Error(), "already saved") {
		t.Fatalf("saved name: %v", err)
	}
	if exists(dir) || len(st.recs) != 1 {
		t.Fatal("nothing may be written for a saved name")
	}

	// A project that exists only as running containers may get its file.
	live := `[{"Id":"x1","Names":["/live-web-1"],"Image":"nginx","State":"running","Labels":{"com.docker.compose.project":"live","com.docker.compose.service":"web"}}]`
	st = &fakeStore{}
	dir = filepath.Join(t.TempDir(), "live")
	res, err := createSvc(t, &fakeRunner{}, st, live).CreateProject(ctx, "c1", dir, "live", newFileText)
	if err != nil {
		t.Fatal(err)
	}
	if res.Project == nil || res.Project.Name != "live" || len(st.recs) != 1 {
		t.Fatalf("result %+v records %+v", res, st.recs)
	}
}

func failConfigQuiet(c compose.Command) (string, error) {
	for _, a := range c.Args {
		if a == "-q" {
			return "", errors.New("exit status 15: services.app.image must be a string")
		}
	}
	return "", nil
}

func TestCreateProject_InvalidFileLeavesNothing(t *testing.T) {
	ctx := context.Background()
	st := &fakeStore{}
	root := t.TempDir()
	dir := filepath.Join(root, "new")
	_, err := createSvc(t, &fakeRunner{out: failConfigQuiet}, st, "").CreateProject(ctx, "c1", dir, "shop", newFileText)
	if err == nil || !strings.Contains(err.Error(), "not valid") || !strings.Contains(err.Error(), "must be a string") {
		t.Fatalf("got %v", err)
	}
	if exists(dir) {
		t.Fatal("the folder this call created must be removed")
	}
	if len(st.recs) != 0 {
		t.Fatal("no record may be saved")
	}

	// Every missing parent this call created goes too, not only the last folder.
	nested := filepath.Join(root, "a", "b", "c")
	if _, err := createSvc(t, &fakeRunner{out: failConfigQuiet}, st, "").CreateProject(ctx, "c1", nested, "shop", newFileText); err == nil {
		t.Fatal("want an error")
	}
	if got := dirEntries(t, root); len(got) != 0 {
		t.Fatalf("created parent folders were left behind: %v", got)
	}

	// A folder that was there before stays, without the file.
	if _, err := createSvc(t, &fakeRunner{out: failConfigQuiet}, st, "").CreateProject(ctx, "c1", root, "shop", newFileText); err == nil {
		t.Fatal("want an error")
	}
	if !exists(root) || len(dirEntries(t, root)) != 0 {
		t.Fatalf("existing folder: %v", dirEntries(t, root))
	}
}

type failingStore struct{ fakeStore }

func (f *failingStore) SaveComposeProject(context.Context, appdb.ComposeProjectRecord) (appdb.ComposeProjectRecord, error) {
	return appdb.ComposeProjectRecord{}, errors.New("disk full")
}

func TestCreateProject_SaveFailureLeavesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new")
	_, err := createSvc(t, &fakeRunner{}, &failingStore{}, "").CreateProject(context.Background(), "c1", dir, "shop", newFileText)
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("got %v", err)
	}
	if exists(dir) {
		t.Fatal("the file and the folder must be removed")
	}
}

func TestCreateProject_RejectsBadFolder(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	cases := []struct{ name, conn, dir, want string }{
		{"empty", "c1", "  ", "choose a folder"},
		{"relative", "c1", filepath.Join("relative", "dir"), "absolute"},
		{"a file", "c1", file, "is a file, not a folder"},
		{"windows path on wsl", "wsl", `C:\apps\shop`, "Windows path"},
		{"option-like", "wsl", "-rf", "must not start"},
	}
	for _, c := range cases {
		r, st := &fakeRunner{}, &fakeStore{}
		_, err := createSvc(t, r, st, "").CreateProject(ctx, c.conn, c.dir, "shop", newFileText)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v want %q", c.name, err, c.want)
		}
		if len(st.recs) != 0 {
			t.Errorf("%s: a record was saved", c.name)
		}
		for _, cmd := range r.cmds {
			args := strings.Join(cmd.Args, " ")
			if strings.Contains(args, "mkdir") || strings.Contains(args, "cat >") {
				t.Errorf("%s: something was created: %v", c.name, cmd.Args)
			}
		}
	}
	if got := dirEntries(t, root); len(got) != 1 {
		t.Errorf("files were added: %v", got)
	}
}

// deniedCreator makes folders for real and then refuses to write the file.
type deniedCreator struct{ compose.LocalFS }

func (deniedCreator) CreateFile(context.Context, string, []byte) error {
	return errors.New("permission denied")
}

func TestCreateProject_UnwritableFolder(t *testing.T) {
	st := &fakeStore{}
	s := createSvc(t, &fakeRunner{}, st, "")
	s.creator = deniedCreator{}
	dir := filepath.Join(t.TempDir(), "locked")
	_, err := s.CreateProject(context.Background(), "c1", dir, "shop", newFileText)
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("got %v", err)
	}
	if exists(dir) {
		t.Fatal("the folder this call created must be removed")
	}
	if len(st.recs) != 0 {
		t.Fatal("no record may be saved")
	}
}

func TestCreateProject_RejectsEmptyAndOversizedText(t *testing.T) {
	ctx := context.Background()
	for _, text := range []string{"", "  \n", strings.Repeat("a", compose.MaxEditableFile+1)} {
		dir := filepath.Join(t.TempDir(), "new")
		if _, err := createSvc(t, &fakeRunner{}, &fakeStore{}, "").CreateProject(ctx, "c1", dir, "shop", text); err == nil {
			t.Errorf("text of %d bytes: want an error", len(text))
		}
		if exists(dir) {
			t.Errorf("text of %d bytes: something was written", len(text))
		}
	}
}

func TestCreateProject_NeedsStoreAndValidName(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "new")
	if _, err := createSvc(t, &fakeRunner{}, nil, "").CreateProject(ctx, "c1", dir, "shop", newFileText); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Errorf("nil store: %v", err)
	}
	if _, err := createSvc(t, &fakeRunner{}, &fakeStore{}, "").CreateProject(ctx, "c1", dir, "My App", newFileText); err == nil || !strings.Contains(err.Error(), "invalid Compose project name") {
		t.Errorf("bad name: %v", err)
	}
	if exists(dir) {
		t.Error("something was written")
	}
}

// halfDirCreator creates the parent folder for real and then fails, as mkdir does when only the
// last part of a path is not allowed.
type halfDirCreator struct{ compose.LocalFS }

func (halfDirCreator) MakeDir(_ context.Context, dir string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return false, err
	}
	return false, errors.New("the folder name is not allowed")
}

func TestCreateProject_FailedFolderCreationLeavesNothing(t *testing.T) {
	s := createSvc(t, &fakeRunner{}, &fakeStore{}, "")
	s.creator = halfDirCreator{}
	root := t.TempDir()
	_, err := s.CreateProject(context.Background(), "c1", filepath.Join(root, "new", "bad"), "shop", newFileText)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("got %v", err)
	}
	if got := dirEntries(t, root); len(got) != 0 {
		t.Fatalf("the parent folder this call created was left behind: %v", got)
	}
}

func TestCreateProject_RejectsAFileNameAsFolder(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"compose.yaml", "docker-compose.yml", "Stack.YAML"} {
		st := &fakeStore{}
		_, err := createSvc(t, &fakeRunner{}, st, "").CreateProject(context.Background(), "c1", filepath.Join(root, "app", name), "shop", newFileText)
		if err == nil || !strings.Contains(err.Error(), "choose the folder") {
			t.Errorf("%s: got %v", name, err)
		}
		if len(st.recs) != 0 || len(dirEntries(t, root)) != 0 {
			t.Errorf("%s: something was written", name)
		}
	}
}
