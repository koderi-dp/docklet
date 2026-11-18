package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"docklet/internal/appdb"
	"docklet/internal/compose"
	"docklet/internal/model"
)

type fakeRunner struct {
	mu    sync.Mutex
	cmds  []compose.Command
	out   func(c compose.Command) (string, error)
	calls int
}

func (f *fakeRunner) Run(_ context.Context, c compose.Command, _ func(string)) error {
	f.mu.Lock()
	f.cmds, f.calls = append(f.cmds, c), f.calls+1
	f.mu.Unlock()
	return nil
}

func (f *fakeRunner) Output(_ context.Context, c compose.Command) (string, error) {
	f.mu.Lock()
	f.cmds, f.calls = append(f.cmds, c), f.calls+1
	f.mu.Unlock()
	if f.out != nil {
		return f.out(c)
	}
	return "", nil
}

type fakeStore struct {
	recs    []appdb.ComposeProjectRecord
	deleted []string
}

func (f *fakeStore) ListComposeProjects(_ context.Context, cid string) ([]appdb.ComposeProjectRecord, error) {
	var out []appdb.ComposeProjectRecord
	for _, r := range f.recs {
		if r.ConnectionID == cid {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeStore) SaveComposeProject(_ context.Context, r appdb.ComposeProjectRecord) (appdb.ComposeProjectRecord, error) {
	f.recs = append(f.recs, r)
	return r, nil
}

func (f *fakeStore) DeleteComposeProject(_ context.Context, cid, name string) error {
	f.deleted = append(f.deleted, cid+"/"+name)
	return nil
}

type fakeProfiles map[string]model.ConnectionProfile

func (f fakeProfiles) Profile(id string) (model.ConnectionProfile, error) {
	p, ok := f[id]
	if !ok {
		return p, errors.New("unknown connection")
	}
	return p, nil
}

func writeFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func newSvc(r *fakeRunner, st ComposeProjectStore) *ComposeService {
	return &ComposeService{
		Projects: st, runner: r,
		Profiles: fakeProfiles{
			"tcp": {ID: "tcp", Type: model.ConnTCP, Endpoint: "tcp://127.0.0.1:23760", TLSVerify: true, TLSCertDir: "C:\\certs"},
			"ssh": {ID: "ssh", Type: model.ConnSSH, Endpoint: "ssh://dev@box:2222"},
			"wsl": {ID: "wsl", Type: model.ConnWSL, WSLDistro: "Ubuntu"},
		},
	}
}

func TestAddProject_RejectsInvalidInputAndSavesNothing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	good := writeFile(t, dir, "compose.yml")
	envf := writeFile(t, dir, ".env")

	// No store: managed features unavailable, nothing runs.
	r := &fakeRunner{}
	if _, err := newSvc(r, nil).AddProject(ctx, "tcp", []string{good}, nil, ""); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Errorf("nil store: %v", err)
	}

	cases := []struct {
		name, conn string
		cfg, env   []string
		pname      string
		want       string
	}{
		{"no files", "tcp", nil, nil, "", "at least one"},
		{"relative path", "tcp", []string{"compose.yml"}, nil, "", "absolute"},
		{"missing file", "tcp", []string{filepath.Join(dir, "nope.yml")}, nil, "", "not found"},
		{"missing env file", "tcp", []string{good}, []string{filepath.Join(dir, "nope.env")}, "", "not found"},
		{"bad project name", "tcp", []string{good}, []string{envf}, "My App", "invalid Compose project name"},
		{"option-like path", "tcp", []string{"-f"}, nil, "", "must not start"},
		{"wsl windows path", "wsl", []string{good}, nil, "", "Windows path"},
		{"unknown connection", "zzz", []string{good}, nil, "", "unknown connection"},
	}
	for _, c := range cases {
		r := &fakeRunner{}
		st := &fakeStore{}
		_, err := newSvc(r, st).AddProject(ctx, c.conn, c.cfg, c.env, c.pname)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v want %q", c.name, err, c.want)
		}
		if len(st.recs) != 0 {
			t.Errorf("%s: record saved despite failure", c.name)
		}
	}

	// docker compose config -q failure surfaces the CLI error and saves nothing.
	r = &fakeRunner{out: func(c compose.Command) (string, error) {
		if len(c.Args) > 1 && c.Args[1] == "version" {
			return "", nil
		}
		for _, a := range c.Args {
			if a == "-q" {
				return "", errors.New("exit status 15: services.web.image must be a string")
			}
		}
		return `{"name":"fromfile"}`, nil
	}}
	st := &fakeStore{}
	if _, err := newSvc(r, st).AddProject(ctx, "tcp", []string{good}, nil, ""); err == nil || !strings.Contains(err.Error(), "must be a string") {
		t.Errorf("config -q failure: %v", err)
	}
	if len(st.recs) != 0 {
		t.Error("invalid project must not be saved")
	}
}

func TestAddProject_SavesRecordWithDetectedNameAndConnectionEnv(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := writeFile(t, dir, "compose.yml")
	extra := writeFile(t, dir, "compose.override.yml")
	envf := writeFile(t, dir, "prod.env")
	r := &fakeRunner{out: func(c compose.Command) (string, error) { return `{"name":"fromfile","services":{}}`, nil }}
	st := &fakeStore{}
	// Reading the project back goes through the Engine; with no provider it errors AFTER saving.
	_, _ = newSvc(r, st).AddProject(ctx, "tcp", []string{cfg, extra}, []string{envf}, "")
	if len(st.recs) != 1 {
		t.Fatalf("records: %+v", st.recs)
	}
	rec := st.recs[0]
	if rec.Name != "fromfile" || rec.ConnectionID != "tcp" || rec.WorkingDir != dir || len(rec.ConfigFiles) != 2 || rec.EnvFiles[0] != envf {
		t.Errorf("record: %+v", rec)
	}
	var sawQuiet bool
	for _, c := range r.cmds {
		if c.Name != "docker" {
			t.Errorf("unexpected binary %q", c.Name)
		}
		if strings.Join(c.EnvSet, " ") != "DOCKER_HOST=tcp://127.0.0.1:23760 DOCKER_TLS_VERIFY=1 DOCKER_CERT_PATH=C:\\certs" {
			t.Errorf("env: %v", c.EnvSet)
		}
		if strings.Contains(strings.Join(c.Args, " "), "-p fromfile") && c.Args[len(c.Args)-1] == "-q" {
			sawQuiet = true
		}
	}
	if !sawQuiet {
		t.Errorf("config -q with the detected project name was not run: %+v", r.cmds)
	}
}

func TestAddProject_FallsBackToNormalisedDirectoryName(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "My Cool_App")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := writeFile(t, dir, "compose.yml")
	r := &fakeRunner{out: func(c compose.Command) (string, error) {
		for _, a := range c.Args {
			if a == "json" {
				return "not json", nil
			}
		}
		return "", nil
	}}
	st := &fakeStore{}
	_, _ = newSvc(r, st).AddProject(ctx, "ssh", []string{cfg}, nil, "")
	if len(st.recs) != 1 || st.recs[0].Name != "mycool_app" {
		t.Fatalf("%+v", st.recs)
	}
	// ssh: the local CLI gets DOCKER_HOST=ssh://...
	if got := r.cmds[len(r.cmds)-1].EnvSet; len(got) != 1 || got[0] != "DOCKER_HOST=ssh://dev@box:2222" {
		t.Errorf("ssh env: %v", got)
	}
}

func TestRemoveProject_OnlyForgetsTheRecordAndNeverRunsCompose(t *testing.T) {
	r := &fakeRunner{}
	st := &fakeStore{}
	s := newSvc(r, st)
	if err := s.RemoveProject(context.Background(), "tcp", "web"); err != nil {
		t.Fatal(err)
	}
	if r.calls != 0 {
		t.Fatalf("RemoveProject invoked the runner %d times: %+v", r.calls, r.cmds)
	}
	if len(st.deleted) != 1 || st.deleted[0] != "tcp/web" {
		t.Errorf("deleted: %v", st.deleted)
	}
	if err := newSvc(r, nil).RemoveProject(context.Background(), "tcp", "web"); err == nil {
		t.Error("nil store must fail clearly")
	}
}

func TestLocation_ReportsExecutionEnvironmentAndPathNote(t *testing.T) {
	s := newSvc(&fakeRunner{}, &fakeStore{})
	l, err := s.Location(context.Background(), "wsl")
	if err != nil || !l.Available || l.Kind != "wsl" || l.ExecutionLocation != "WSL: Ubuntu" || !strings.Contains(l.PathNote, `\\wsl$\Ubuntu`) {
		t.Errorf("wsl: %+v %v", l, err)
	}
	l, _ = s.Location(context.Background(), "ssh")
	if !l.Available || !strings.Contains(l.Warning, "REMOTE host") {
		t.Errorf("ssh: %+v", l)
	}
	if l, _ = newSvc(&fakeRunner{}, nil).Location(context.Background(), "tcp"); l.ManagedAvailable {
		t.Error("managed must be unavailable without store")
	}
}
