package services

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklet/internal/model"

	"github.com/moby/moby/api/types/container"
)

// helperClean asserts exactly one helper was created and removed again.
func helperClean(t *testing.T, x *xferAPI, wantReadOnly bool) {
	t.Helper()
	if len(x.created) != 1 || len(x.removed) != 1 || x.removed[0] != "helper-"+x.created[0] {
		t.Fatalf("helper lifecycle: created=%v removed=%v", x.created, x.removed)
	}
	if x.readOnly[0] != wantReadOnly {
		t.Fatalf("helper readOnly = %v, want %v", x.readOnly[0], wantReadOnly)
	}
}

func TestUnmapEntryPath(t *testing.T) {
	for in, want := range map[string]string{
		"/volume":         "/",
		"/volume/a":       "/a",
		"/volume/a/b.txt": "/a/b.txt",
		"/volume/":        "/",
	} {
		if got := unmapEntryPath(in); got != want {
			t.Errorf("unmapEntryPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVolumeReadText_ReadsThroughReadOnlyHelperAndShowsVolumeRelativePath(t *testing.T) {
	x := newXfer()
	x.copyFrom = makeTar(t, []tarItem{{name: "app.conf", typ: tar.TypeReg, body: "key=1\n"}})
	s, _ := x.svc()
	got, err := s.VolumeReadText(bg, "c1", "myvol", "/etc/app.conf", 100)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/etc/app.conf" || got.Content != "key=1\n" || got.Size != 6 {
		t.Fatalf("%+v", got)
	}
	if len(x.fromPaths) != 1 || x.fromPaths[0] != "/volume/etc/app.conf" {
		t.Fatalf("engine path = %v (must be inside the helper mount)", x.fromPaths)
	}
	helperClean(t, x, true)
}

func TestVolumeReadText_RejectsBadInputBeforeCreatingAnyHelper(t *testing.T) {
	for _, tc := range []struct{ name, vol, path string }{
		{"root is not a file", "myvol", "/"},
		{"traversal", "myvol", "/../../etc/passwd"},
		{"relative", "myvol", "etc/passwd"},
		{"bad volume name", "my vol;rm", "/a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newXfer()
			s, _ := x.svc()
			if _, err := s.VolumeReadText(bg, "c1", tc.vol, tc.path, 10); err == nil {
				t.Fatal("expected error")
			}
			if len(x.created) != 0 || len(x.fromPaths) != 0 {
				t.Fatalf("engine touched: created=%v from=%v", x.created, x.fromPaths)
			}
		})
	}
}

func TestVolumeReadText_RemovesHelperWhenReadFails(t *testing.T) {
	x := newXfer()
	x.copyFromErr = errors.New("Could not find the file /volume/x")
	s, _ := x.svc()
	_, err := s.VolumeReadText(bg, "c1", "myvol", "/x", 10)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v", err)
	}
	helperClean(t, x, true)
}

func TestVolumeDownload_FetchesFromMountedPathAndSavesFile(t *testing.T) {
	x := newXfer()
	x.copyFrom = makeTar(t, []tarItem{{name: "data.bin", typ: tar.TypeReg, body: "payload"}})
	x.copyStat = fileStat(7)
	s, m := x.svc()
	dest := filepath.Join(t.TempDir(), "data.bin")
	op, err := s.VolumeDownload(bg, "c1", "myvol", "/sub//data.bin", dest)
	if err != nil {
		t.Fatal(err)
	}
	if op.Kind != KindFilesDownload || !strings.Contains(op.Title, "myvol:/sub/data.bin") {
		t.Fatalf("op = %+v", op)
	}
	if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded {
		t.Fatalf("op: %+v", done)
	}
	if b, _ := os.ReadFile(dest); string(b) != "payload" {
		t.Fatalf("content = %q", b)
	}
	if x.fromPaths[0] != "/volume/sub/data.bin" {
		t.Fatalf("engine path = %v", x.fromPaths)
	}
	helperClean(t, x, true)
}

func TestVolumeDownload_RootIsAllowedAndSavedAsTar(t *testing.T) {
	x := newXfer()
	x.copyFrom = makeTar(t, []tarItem{{name: "volume/", typ: tar.TypeDir}})
	x.copyStat = container.PathStat{Name: "volume", Mode: fs.ModeDir | 0o755}
	s, m := x.svc()
	dir := t.TempDir()
	op, err := s.VolumeDownload(bg, "c1", "myvol", "/", filepath.Join(dir, "all"))
	if err != nil {
		t.Fatal(err)
	}
	if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded {
		t.Fatalf("op: %+v", done)
	}
	if _, err := os.Stat(filepath.Join(dir, "all.tar")); err != nil || x.fromPaths[0] != "/volume" {
		t.Fatalf("err=%v from=%v", err, x.fromPaths)
	}
}

func TestVolumeDownload_ValidatesUpFrontAndCleansHelperOnFailure(t *testing.T) {
	x := newXfer()
	s, m := x.svc()
	for _, tc := range []struct{ p, dest string }{
		{"/../x", filepath.Join(t.TempDir(), "o")},
		{"/x", "relative"},
		{"/x", ""},
	} {
		if _, err := s.VolumeDownload(bg, "c1", "myvol", tc.p, tc.dest); err == nil {
			t.Errorf("%q -> %q accepted", tc.p, tc.dest)
		}
	}
	if len(x.created) != 0 {
		t.Fatalf("helper created for invalid input: %v", x.created)
	}

	x.copyFromErr = errors.New("no such file or directory")
	dest := filepath.Join(t.TempDir(), "o")
	op, err := s.VolumeDownload(bg, "c1", "myvol", "/gone", dest)
	if err != nil {
		t.Fatal(err)
	}
	done := waitOp(t, m, op.ID)
	if done.Status != model.OpFailed || !strings.Contains(done.Error, "does not exist") {
		t.Fatalf("op: %+v", done)
	}
	helperClean(t, x, true)
	if got := dirNames(t, filepath.Dir(dest)); len(got) != 0 {
		t.Fatalf("output left: %v", got)
	}
}

func TestVolumeUpload_UsesWritableHelperAndMapsDestinationIntoMount(t *testing.T) {
	src := filepath.Join(t.TempDir(), "note.txt")
	_ = os.WriteFile(src, []byte("hi"), 0o644)
	x := newXfer()
	s, m := x.svc()
	op, err := s.VolumeUpload(bg, "c1", "myvol", "/conf/", src)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded {
		t.Fatalf("op: %+v", done)
	}
	if len(x.toDirs) != 1 || x.toDirs[0] != "/volume/conf" {
		t.Fatalf("dest dir = %v", x.toDirs)
	}
	if got := tarBodies(t, x.toData); got["note.txt"] != "hi" {
		t.Fatalf("tar = %v", got)
	}
	helperClean(t, x, false)
}

func TestVolumeUpload_RejectsBadInputBeforeCreatingAnyHelper(t *testing.T) {
	src := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(src, []byte("x"), 0o644)
	for _, tc := range []struct{ name, dest, src string }{
		{"traversal in destination", "/../etc", src},
		{"relative destination", "conf", src},
		{"relative source", "/conf", "f"},
		{"source does not exist", "/conf", filepath.Join(t.TempDir(), "missing")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newXfer()
			s, _ := x.svc()
			if _, err := s.VolumeUpload(bg, "c1", "myvol", tc.dest, tc.src); err == nil {
				t.Fatal("expected error")
			}
			if len(x.created) != 0 {
				t.Fatalf("helper created: %v", x.created)
			}
		})
	}
}

func TestVolumeUpload_FailureStillRemovesHelperAndReportsReason(t *testing.T) {
	src := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(src, []byte("x"), 0o644)
	x := newXfer()
	x.toErr = errors.New("permission denied")
	s, m := x.svc()
	op, err := s.VolumeUpload(bg, "c1", "myvol", "/", src)
	if err != nil {
		t.Fatal(err)
	}
	done := waitOp(t, m, op.ID)
	if done.Status != model.OpFailed || !strings.Contains(done.Error, "Permission denied") {
		t.Fatalf("op: %+v", done)
	}
	if x.toDirs[0] != "/volume" {
		t.Fatalf("root upload must target the mount itself, got %v", x.toDirs)
	}
	helperClean(t, x, false)
}

func TestVolumeExport_WritesWholeVolumeTarThroughReadOnlyHelper(t *testing.T) {
	x := newXfer()
	raw := makeTar(t, []tarItem{{name: "./", typ: tar.TypeDir}, {name: "./a", typ: tar.TypeReg, body: "A"}})
	x.fromBody = io.NopCloser(strings.NewReader(string(raw)))
	s, m := x.svc()
	dest := filepath.Join(t.TempDir(), "vol.tar")
	op, err := s.VolumeExport(bg, "c1", "myvol", dest)
	if err != nil {
		t.Fatal(err)
	}
	if op.Kind != KindVolumeExport {
		t.Fatalf("kind = %s", op.Kind)
	}
	if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded {
		t.Fatalf("op: %+v", done)
	}
	if b, _ := os.ReadFile(dest); string(b) != string(raw) {
		t.Fatal("exported file is not the engine stream verbatim")
	}
	if x.fromPaths[0] != "/volume/." {
		t.Fatalf("export must copy the mount's contents (/volume/.), got %v", x.fromPaths)
	}
	helperClean(t, x, true)
}

func TestVolumeExport_ValidatesUpFrontAndFailureLeavesNoFile(t *testing.T) {
	x := newXfer()
	s, m := x.svc()
	good := filepath.Join(t.TempDir(), "v.tar")
	for _, tc := range []struct{ vol, dest string }{
		{"x", good}, {"has space", good}, {"-lead", good}, {"myvol", "relative.tar"}, {"myvol", ""},
	} {
		if _, err := s.VolumeExport(bg, "c1", tc.vol, tc.dest); err == nil {
			t.Errorf("accepted vol=%q dest=%q", tc.vol, tc.dest)
		}
	}
	if len(x.created) != 0 {
		t.Fatalf("helper created for invalid input: %v", x.created)
	}

	x.copyFromErr = errors.New("boom")
	op, err := s.VolumeExport(bg, "c1", "myvol", good)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitOp(t, m, op.ID); done.Status != model.OpFailed {
		t.Fatalf("op: %+v", done)
	}
	helperClean(t, x, true)
	if got := dirNames(t, filepath.Dir(good)); len(got) != 0 {
		t.Fatalf("file left after failed export: %v", got)
	}
}

func TestVolumeTransfers_UnavailableOperationsAreReportedNotPanicked(t *testing.T) {
	x := newXfer()
	s := &FileService{newAPI: func(context.Context, string) (filesAPI, error) { return x, nil }}
	if _, err := s.VolumeExport(bg, "c1", "myvol", filepath.Join(t.TempDir(), "v.tar")); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("err = %v", err)
	}
}
