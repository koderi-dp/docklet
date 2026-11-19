package services

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklet/internal/ops"
)

func tarBodies(t *testing.T, b []byte) map[string]string {
	t.Helper()
	out := map[string]string{}
	tr := tar.NewReader(bytes.NewReader(b))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(tr)
		out[h.Name] = string(body)
	}
}

func uploadErr(t *testing.T, x *xferAPI, dest, src string) (string, error) {
	t.Helper()
	op, err := runOp(t, func(ctx context.Context, h *ops.Handle) error {
		return uploadPath(ctx, h, x, "id", dest, src)
	})
	return op.Detail, err
}

func TestUploadPath_StreamsLocalTreeAsTarAndCountsBytes(t *testing.T) {
	src := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(src, "a.txt"), []byte("AAAA"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("BB"), 0o644)
	x := newXfer()
	detail, err := uploadErr(t, x, "/data", src)
	if err != nil {
		t.Fatal(err)
	}
	got := tarBodies(t, x.toData)
	if got["proj/a.txt"] != "AAAA" || got["proj/sub/b.txt"] != "BB" || len(x.toDirs) != 1 || x.toDirs[0] != "/data" {
		t.Fatalf("tar=%v dirs=%v", got, x.toDirs)
	}
	if detail != "Uploaded 6 B" {
		t.Fatalf("detail = %q (counts file bytes, not tar overhead)", detail)
	}
}

func TestUploadPath_EngineRejectionIsExplainedWithoutDeadlock(t *testing.T) {
	// The sink fails without reading: the tar writer goroutine must be released.
	src := filepath.Join(t.TempDir(), "big.bin")
	if err := os.WriteFile(src, bytes.Repeat([]byte("z"), 1<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	x := newXfer()
	x.toNoRead = true
	x.toErr = errors.New("Error response: read-only file system")
	_, err := uploadErr(t, x, "/data", src)
	if err == nil || !strings.Contains(err.Error(), friendlyROFS) {
		t.Fatalf("err = %v", err)
	}
}

func TestUploadPath_LocalReadFailureSurfacesThroughThePipe(t *testing.T) {
	x := newXfer()
	_, err := uploadErr(t, x, "/data", filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("a source that cannot be read must fail the upload, not send an empty archive")
	}
}

func TestUploadPath_CancelledContextReportsCancellation(t *testing.T) {
	src := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(src, []byte("x"), 0o644)
	x := newXfer()
	x.toNoRead = true
	x.toErr = errors.New("context canceled while copying")
	ctx, cancel := context.WithCancel(bg)
	cancel()
	_, _ = runOp(t, func(_ context.Context, h *ops.Handle) error {
		if err := uploadPath(ctx, h, x, "id", "/d", src); !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v", err)
		}
		return nil
	})
}
