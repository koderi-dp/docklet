package services

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklet/internal/model"
	"docklet/internal/ops"
)

// slSetup builds an ImageService against a fake Engine handler with a real ops manager.
func slSetup(t *testing.T, h http.HandlerFunc) (*ImageService, *ops.Manager, *recEmitter) {
	t.Helper()
	em := &recEmitter{}
	m := ops.New(nil)
	cli := dockerClient(t, h, "1.45")
	return &ImageService{Engines: fakeProvider{cli}, Events: em, Ops: m}, m, em
}

// slWait blocks until the operation left the running state and returns its final snapshot.
func slWait(t *testing.T, m *ops.Manager, id string) model.Operation {
	t.Helper()
	var final model.Operation
	eventually(t, "operation "+id+" to finish", func() bool {
		for _, o := range m.List() {
			if o.ID == id && o.Status != model.OpRunning {
				final = o
				return true
			}
		}
		return false
	})
	return final
}

func slLogs(t *testing.T, m *ops.Manager, id string) string {
	t.Helper()
	l, err := m.Logs(id)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(l, "\n")
}

func slNoFile(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s must not exist (err=%v)", p, err)
		}
	}
}

func TestSave_WritesArchiveDedupesRefsAndSetsOperationMetadata(t *testing.T) {
	var names []string
	s, m, _ := slSetup(t, func(w http.ResponseWriter, r *http.Request) {
		names = r.URL.Query()["names"]
		_, _ = w.Write([]byte("TAR-DATA"))
	})
	dest := filepath.Join(t.TempDir(), "out.tar")
	op, err := s.Save(context.Background(), "c1", []string{"nginx:1", "nginx:1", "redis:7"}, dest)
	if err != nil {
		t.Fatal(err)
	}
	if op.Kind != "image.save" || op.Title != "Save 2 images" || op.Resource != "out.tar" || !op.Cancellable || op.ConnectionID != "c1" {
		t.Errorf("operation = %+v", op)
	}
	fin := slWait(t, m, op.ID)
	if fin.Status != model.OpSucceeded {
		t.Fatalf("status %s: %s", fin.Status, fin.Error)
	}
	if b, _ := os.ReadFile(dest); string(b) != "TAR-DATA" {
		t.Errorf("archive = %q", b)
	}
	slNoFile(t, dest+".part")
	if len(names) != 2 || names[0] != "nginx:1" || names[1] != "redis:7" {
		t.Errorf("duplicate refs must be sent once, in order: %v", names)
	}
	if !strings.Contains(slLogs(t, m, op.ID), "Saved 2 image(s), 8 B") {
		t.Errorf("logs: %s", slLogs(t, m, op.ID))
	}
}

func TestSave_SingleImageTitleUsesItsName(t *testing.T) {
	s, m, _ := slSetup(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) })
	op, err := s.Save(context.Background(), "c1", []string{"nginx:1"}, filepath.Join(t.TempDir(), "o.tar"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(op.Title, "Save ") || !strings.Contains(op.Title, "nginx") {
		t.Errorf("title %q", op.Title)
	}
	slWait(t, m, op.ID)
}

func TestSave_RejectsBadInputWithoutStartingAnOperation(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "o.tar")
	cases := []struct {
		name    string
		conn    string
		refs    []string
		dest    string
		noOps   bool
		wantErr string
	}{
		{"no images", "c1", nil, good, false, "at least one image"},
		{"invalid ref", "c1", []string{"bad ref"}, good, false, ""},
		{"relative destination", "c1", []string{"a:1"}, "o.tar", false, "absolute"},
		{"unknown connection", "nope", []string{"a:1"}, good, false, "unknown connection"},
		{"operations unavailable", "c1", []string{"a:1"}, good, true, "not available"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			called := false
			s, m, _ := slSetup(t, func(w http.ResponseWriter, r *http.Request) { called = true })
			if c.noOps {
				s.Ops = nil
			}
			_, err := s.Save(context.Background(), c.conn, c.refs, c.dest)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, c.wantErr)
			}
			if called || len(m.List()) != 0 {
				t.Error("nothing may be started or sent to the Engine")
			}
			slNoFile(t, good, good+".part")
		})
	}
}

func TestSave_EngineErrorFailsOperationAndKeepsExistingDestination(t *testing.T) {
	s, m, _ := slSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message":"No such image: gone:1"}`))
	})
	dest := filepath.Join(t.TempDir(), "keep.tar")
	if err := os.WriteFile(dest, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	op, err := s.Save(context.Background(), "c1", []string{"gone:1"}, dest)
	if err != nil {
		t.Fatal(err)
	}
	fin := slWait(t, m, op.ID)
	if fin.Status != model.OpFailed || fin.Error == "" {
		t.Fatalf("op = %+v", fin)
	}
	if b, _ := os.ReadFile(dest); string(b) != "OLD" {
		t.Errorf("existing destination was modified: %q", b)
	}
	slNoFile(t, dest+".part")
}

func TestSave_EmptyArchiveFailsAndLeavesNoFile(t *testing.T) {
	s, m, _ := slSetup(t, func(w http.ResponseWriter, r *http.Request) {})
	dest := filepath.Join(t.TempDir(), "empty.tar")
	op, _ := s.Save(context.Background(), "c1", []string{"a:1"}, dest)
	fin := slWait(t, m, op.ID)
	if fin.Status != model.OpFailed || !strings.Contains(fin.Error, "empty archive") {
		t.Fatalf("op = %+v", fin)
	}
	slNoFile(t, dest, dest+".part")
}

func TestSave_CancelMidStreamEndsCancelledAndRemovesPartialFile(t *testing.T) {
	started := make(chan struct{})
	s, m, _ := slSetup(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial-bytes"))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	})
	dest := filepath.Join(t.TempDir(), "c.tar")
	op, _ := s.Save(context.Background(), "c1", []string{"a:1"}, dest)
	<-started
	if err := m.Cancel(op.ID); err != nil {
		t.Fatal(err)
	}
	fin := slWait(t, m, op.ID)
	if fin.Status != model.OpCancelled {
		t.Fatalf("op = %+v", fin)
	}
	slNoFile(t, dest, dest+".part")
}

func TestLoad_UploadsFileReportsLoadedImagesAndEmitsChange(t *testing.T) {
	var got []byte
	s, m, em := slSetup(t, func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"stream":"Loaded image: app:1\n"}` + "\n" + `{"stream":"Loaded image ID: sha256:abc\n"}` + "\n"))
	})
	src := filepath.Join(t.TempDir(), "img.tar")
	if err := os.WriteFile(src, []byte("ARCHIVE-BYTES"), 0o644); err != nil {
		t.Fatal(err)
	}
	op, err := s.Load(context.Background(), "c1", src)
	if err != nil {
		t.Fatal(err)
	}
	if op.Kind != "image.load" || op.Title != "Load img.tar" || op.Resource != "img.tar" || !op.Cancellable {
		t.Errorf("operation = %+v", op)
	}
	fin := slWait(t, m, op.ID)
	if fin.Status != model.OpSucceeded {
		t.Fatalf("op = %+v", fin)
	}
	if string(got) != "ARCHIVE-BYTES" {
		t.Errorf("engine received %q", got)
	}
	if !strings.Contains(slLogs(t, m, op.ID), "Loaded: app:1, sha256:abc") {
		t.Errorf("logs: %s", slLogs(t, m, op.ID))
	}
	if n := em.find(model.EventImagesChanged); len(n) != 1 {
		t.Errorf("want one images:changed, got %d", len(n))
	}
}

func TestLoad_EngineReportedErrorFailsButStillRefreshesImages(t *testing.T) {
	s, m, em := slSetup(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{"errorDetail":{"message":"invalid tar header"},"error":"invalid tar header"}`))
	})
	src := filepath.Join(t.TempDir(), "bad.tar")
	_ = os.WriteFile(src, []byte("junk"), 0o644)
	op, _ := s.Load(context.Background(), "c1", src)
	fin := slWait(t, m, op.ID)
	if fin.Status != model.OpFailed || !strings.Contains(fin.Error, "invalid tar header") {
		t.Fatalf("op = %+v", fin)
	}
	if len(em.find(model.EventImagesChanged)) != 1 {
		t.Error("images may have been partly loaded: the list must refresh")
	}
}

func TestLoad_HTTPErrorFailsWithoutRefreshAndNoLoadedImagesIsLogged(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a.tar")
	_ = os.WriteFile(src, []byte("x"), 0o644)

	s, m, em := slSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})
	op, _ := s.Load(context.Background(), "c1", src)
	if fin := slWait(t, m, op.ID); fin.Status != model.OpFailed || !strings.Contains(fin.Error, "boom") {
		t.Fatalf("op = %+v", fin)
	}
	if len(em.find(model.EventImagesChanged)) != 0 {
		t.Error("a request that never reached the Engine must not emit a change")
	}

	s2, m2, _ := slSetup(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body) })
	op2, _ := s2.Load(context.Background(), "c1", src)
	if fin := slWait(t, m2, op2.ID); fin.Status != model.OpSucceeded {
		t.Fatalf("op = %+v", fin)
	}
	if !strings.Contains(slLogs(t, m2, op2.ID), "reported no loaded images") {
		t.Errorf("logs: %s", slLogs(t, m2, op2.ID))
	}
}

func TestLoad_RejectsBadInputWithoutStartingAnOperation(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.tar")
	_ = os.WriteFile(empty, nil, 0o644)
	ok := filepath.Join(dir, "ok.tar")
	_ = os.WriteFile(ok, []byte("x"), 0o644)
	cases := []struct {
		name, conn, path string
		noOps            bool
	}{
		{"empty path", "c1", "  ", false},
		{"relative path", "c1", "ok.tar", false},
		{"missing file", "c1", filepath.Join(dir, "none.tar"), false},
		{"directory", "c1", dir, false},
		{"empty file", "c1", empty, false},
		{"unknown connection", "nope", ok, false},
		{"operations unavailable", "c1", ok, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, m, _ := slSetup(t, func(w http.ResponseWriter, r *http.Request) { t.Error("Engine must not be called") })
			if c.noOps {
				s.Ops = nil
			}
			if _, err := s.Load(context.Background(), c.conn, c.path); err == nil {
				t.Fatal("expected error")
			}
			if len(m.List()) != 0 {
				t.Error("no operation may start")
			}
		})
	}
}

func TestLoad_CancelWhileEngineIsBusyEndsCancelled(t *testing.T) {
	started := make(chan struct{})
	s, m, _ := slSetup(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	})
	src := filepath.Join(t.TempDir(), "a.tar")
	_ = os.WriteFile(src, []byte("x"), 0o644)
	op, _ := s.Load(context.Background(), "c1", src)
	<-started
	_ = m.Cancel(op.ID)
	if fin := slWait(t, m, op.ID); fin.Status != model.OpCancelled {
		t.Fatalf("op = %+v", fin)
	}
}
