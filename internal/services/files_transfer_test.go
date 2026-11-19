package services

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"docklet/internal/model"
	"docklet/internal/ops"

	"github.com/moby/moby/api/types/container"
)

// xferAPI extends fakeFilesAPI for transfer tests: it records the paths the code asks the
// engine for and lets a test script the archive body, upload sink and export stream.
type xferAPI struct {
	*fakeFilesAPI
	xmu       sync.Mutex
	fromPaths []string
	toDirs    []string
	toData    []byte
	toErr     error
	toNoRead  bool // CopyTo fails without consuming the stream
	fromBody  io.ReadCloser
	export    func() (io.ReadCloser, error)
}

func newXfer() *xferAPI {
	return &xferAPI{fakeFilesAPI: &fakeFilesAPI{images: []helperImage{{Ref: "busybox:1", Size: 1}}}}
}

func (x *xferAPI) CopyFrom(ctx context.Context, id, p string) (io.ReadCloser, container.PathStat, error) {
	x.xmu.Lock()
	x.fromPaths = append(x.fromPaths, p)
	body := x.fromBody
	x.xmu.Unlock()
	if body != nil {
		return body, x.copyStat, nil
	}
	return x.fakeFilesAPI.CopyFrom(ctx, id, p)
}

func (x *xferAPI) CopyTo(_ context.Context, _ string, dir string, r io.Reader) error {
	x.xmu.Lock()
	x.toDirs = append(x.toDirs, dir)
	x.xmu.Unlock()
	if !x.toNoRead {
		b, err := io.ReadAll(r)
		x.xmu.Lock()
		x.toData = b
		x.xmu.Unlock()
		if err != nil {
			return err
		}
	}
	return x.toErr
}

func (x *xferAPI) Export(context.Context, string) (io.ReadCloser, error) {
	if x.export == nil {
		return nil, errors.New("unused")
	}
	return x.export()
}

func (x *xferAPI) svc() (*FileService, *ops.Manager) {
	m := ops.New(nil)
	return &FileService{Ops: m, newAPI: func(context.Context, string) (filesAPI, error) { return x, nil }}, m
}

// closeSpy records whether the body was closed.
type closeSpy struct {
	io.Reader
	closed bool
}

func (c *closeSpy) Close() error { c.closed = true; return nil }

// failingReader yields data and then err.
type failingReader struct {
	data []byte
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

// runOp runs fn as a real operation (the only way to obtain an *ops.Handle) and returns the
// finished operation and fn's own error.
func runOp(t *testing.T, fn ops.Func) (model.Operation, error) {
	t.Helper()
	m := ops.New(nil)
	var ferr error
	op := m.Start(ops.Spec{Kind: "test"}, func(ctx context.Context, h *ops.Handle) error {
		ferr = fn(ctx, h)
		return ferr
	})
	done := waitOp(t, m, op.ID)
	return done, ferr
}

func dirNames(t *testing.T, dir string) []string {
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

func fileStat(size int64) container.PathStat {
	return container.PathStat{Name: "f", Size: size, Mode: 0o644}
}

func downloadErr(t *testing.T, api filesAPI, dest string) error {
	t.Helper()
	_, err := runOp(t, func(ctx context.Context, h *ops.Handle) error {
		return downloadPath(ctx, h, api, "id", "/volume/x", "/x", dest)
	})
	return err
}

func TestDownloadPath_FileIsUnwrappedFromTarAndSized(t *testing.T) {
	f := &fakeFilesAPI{copyFrom: makeTar(t, []tarItem{{name: "f.txt", typ: tar.TypeReg, body: "hello world"}}), copyStat: fileStat(11)}
	dest := filepath.Join(t.TempDir(), "out.txt")
	op, err := runOp(t, func(ctx context.Context, h *ops.Handle) error {
		return downloadPath(ctx, h, f, "id", "/volume/f.txt", "/f.txt", dest)
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "hello world" {
		t.Fatalf("content = %q (must not be the raw tar)", b)
	}
	if op.Detail != "Saved 11 B" {
		t.Fatalf("detail = %q", op.Detail)
	}
	if got := dirNames(t, filepath.Dir(dest)); len(got) != 1 {
		t.Fatalf("leftover files: %v", got)
	}
}

func TestDownloadPath_DirectoryKeepsTarAndAddsExtensionOnlyWhenMissing(t *testing.T) {
	raw := makeTar(t, []tarItem{{name: "d/", typ: tar.TypeDir}, {name: "d/a", typ: tar.TypeReg, body: "A"}})
	for _, tc := range []struct{ dest, want string }{
		{"backup", "backup.tar"},
		{"backup.TAR", "backup.TAR"},
		{"backup.tar", "backup.tar"},
		{"backup.tgz", "backup.tgz.tar"},
	} {
		f := &fakeFilesAPI{copyFrom: raw, copyStat: container.PathStat{Name: "d", Mode: fs.ModeDir | 0o755}}
		dir := t.TempDir()
		if err := downloadErr(t, f, filepath.Join(dir, tc.dest)); err != nil {
			t.Fatalf("%s: %v", tc.dest, err)
		}
		got, e := os.ReadFile(filepath.Join(dir, tc.want))
		if e != nil || !bytes.Equal(got, raw) {
			t.Fatalf("%s: expected verbatim tar at %s (err=%v)", tc.dest, tc.want, e)
		}
	}
}

func TestDownloadPath_RejectsNonRegularMembersWithoutLeavingOutput(t *testing.T) {
	cases := []struct {
		name string
		item tarItem
		want string
	}{
		{"symlink", tarItem{name: "l", typ: tar.TypeSymlink, link: "/etc/passwd"}, "symbolic link to /etc/passwd"},
		{"directory entry", tarItem{name: "d/", typ: tar.TypeDir}, "not a regular file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeFilesAPI{copyFrom: makeTar(t, []tarItem{tc.item}), copyStat: fileStat(0)}
			dest := filepath.Join(t.TempDir(), "out")
			err := downloadErr(t, f, dest)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if got := dirNames(t, filepath.Dir(dest)); len(got) != 0 {
				t.Fatalf("partial output left behind: %v", got)
			}
		})
	}
}

func TestDownloadPath_FailureKeepsExistingDestinationAndClosesBody(t *testing.T) {
	// A tar whose header promises more bytes than the stream delivers (connection dropped).
	full := makeTar(t, []tarItem{{name: "f", typ: tar.TypeReg, body: strings.Repeat("x", 4096)}})
	spy := &closeSpy{Reader: bytes.NewReader(full[:1024])}
	x := newXfer()
	x.fromBody = spy
	x.copyStat = fileStat(4096)
	dest := filepath.Join(t.TempDir(), "keep.bin")
	if err := os.WriteFile(dest, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := downloadErr(t, x, dest); err == nil {
		t.Fatal("truncated archive must fail")
	}
	if b, _ := os.ReadFile(dest); string(b) != "old" {
		t.Fatalf("existing file clobbered: %q", b)
	}
	if got := dirNames(t, filepath.Dir(dest)); len(got) != 1 {
		t.Fatalf("temp file not cleaned: %v", got)
	}
	if !spy.closed {
		t.Fatal("engine stream not closed")
	}
}

func TestDownloadPath_EngineAndDestinationErrors(t *testing.T) {
	dir := t.TempDir()
	good := &fakeFilesAPI{copyFrom: makeTar(t, []tarItem{{name: "f", typ: tar.TypeReg, body: "x"}}), copyStat: fileStat(1)}
	cases := []struct {
		name string
		api  filesAPI
		dest string
		want string
	}{
		{"missing path is explained", &fakeFilesAPI{copyFromErr: errors.New("Could not find the file /nope")}, filepath.Join(dir, "a"), "does not exist"},
		{"read-only maps to friendly text", &fakeFilesAPI{copyFromErr: errors.New("read-only file system")}, filepath.Join(dir, "b"), friendlyROFS},
		{"destination is a directory", good, dir, "is a directory"},
		{"relative destination", good, "relative.txt", "must be absolute"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := downloadErr(t, tc.api, tc.dest)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestDownloadPath_CancelledContextReportsCancellationNotIOError(t *testing.T) {
	x := newXfer()
	x.fromBody = &closeSpy{Reader: &failingReader{err: errors.New("connection reset")}}
	x.copyStat = fileStat(10)
	ctx, cancel := context.WithCancel(bg)
	cancel()
	dest := filepath.Join(t.TempDir(), "o")
	_, _ = runOp(t, func(_ context.Context, h *ops.Handle) error {
		err := downloadPath(ctx, h, x, "id", "/p", "/p", dest)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
		return nil
	})
	if got := dirNames(t, filepath.Dir(dest)); len(got) != 0 {
		t.Fatalf("output left after cancel: %v", got)
	}
}

func TestExportTo_WritesStreamAndCleansUpOnFailure(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "fs.tar")

	spy := &closeSpy{Reader: strings.NewReader("TARDATA")}
	op, err := runOp(t, func(ctx context.Context, h *ops.Handle) error {
		return exportTo(ctx, h, dest, func() (io.ReadCloser, error) { return spy, nil })
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "TARDATA" || !spy.closed {
		t.Fatalf("content=%q closed=%v", b, spy.closed)
	}
	if !strings.HasPrefix(op.Detail, "Wrote 7 B to ") {
		t.Fatalf("detail = %q", op.Detail)
	}

	// Mid-stream failure: previous export stays intact, no .part file remains.
	bad := &closeSpy{Reader: &failingReader{data: []byte("half"), err: errors.New("stream broke")}}
	_, err = runOp(t, func(ctx context.Context, h *ops.Handle) error {
		return exportTo(ctx, h, dest, func() (io.ReadCloser, error) { return bad, nil })
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if b, _ := os.ReadFile(dest); string(b) != "TARDATA" {
		t.Fatalf("previous export clobbered: %q", b)
	}
	if got := dirNames(t, dir); len(got) != 1 || !bad.closed {
		t.Fatalf("files=%v closed=%v", got, bad.closed)
	}
}

func TestExportTo_OpenErrorFailsAndCreatesNothing(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "fs.tar")
	_, err := runOp(t, func(ctx context.Context, h *ops.Handle) error {
		return exportTo(ctx, h, dest, func() (io.ReadCloser, error) { return nil, errors.New("No such container: abc") })
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := dirNames(t, filepath.Dir(dest)); len(got) != 0 {
		t.Fatalf("created %v", got)
	}
}

func TestTransfer_AddAccumulatesEvenWhenProgressIsThrottled(t *testing.T) {
	op, _ := runOp(t, func(_ context.Context, h *ops.Handle) error {
		tr := &transfer{h: h, total: 100}
		tr.add(10) // first call reports
		tr.add(90) // within the throttle window: must still count
		if tr.done != 100 {
			t.Errorf("done = %d", tr.done)
		}
		return nil
	})
	if op.Progress == nil || *op.Progress != 0.1 || op.Detail != "10 B of 100 B" {
		t.Fatalf("progress=%v detail=%q", op.Progress, op.Detail)
	}
}

func TestTransfer_UnknownTotalShowsBytesOnlyAndNoProgress(t *testing.T) {
	op, _ := runOp(t, func(_ context.Context, h *ops.Handle) error {
		(&transfer{h: h, last: time.Now().Add(-time.Second)}).add(2048)
		return nil
	})
	if op.Progress != nil || op.Detail != "2.0 KB" {
		t.Fatalf("progress=%v detail=%q", op.Progress, op.Detail)
	}
}
