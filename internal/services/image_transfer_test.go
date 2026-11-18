package services

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteArchive_KeepsExistingFileAndRemovesPartialOnFailureOrCancel(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.tar")
	if _, err := writeArchive(context.Background(), strings.NewReader("tar-bytes"), dest, &sink{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "tar-bytes" {
		t.Errorf("content %q", b)
	}
	// Failure mid-stream: existing dest stays, no .part left.
	failing := io.MultiReader(strings.NewReader("abc"), errReader{})
	if _, err := writeArchive(context.Background(), failing, dest, &sink{}); err == nil {
		t.Fatal("expected error")
	}
	if b, _ := os.ReadFile(dest); string(b) != "tar-bytes" {
		t.Errorf("existing destination must be kept, got %q", b)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error("partial file must be removed")
	}
	// Cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dest2 := filepath.Join(dir, "c.tar")
	if _, err := writeArchive(ctx, strings.NewReader("abc"), dest2, &sink{}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancel: %v", err)
	}
	for _, p := range []string{dest2, dest2 + ".part"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s must not exist", p)
		}
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func TestConsumeLoad_CollectsLoadedNamesAndSurfacesErrors(t *testing.T) {
	in := `{"stream":"Loaded image: localhost:5000/a:1\n"}` + "\n" + `{"stream":"Loaded image ID: sha256:abc\n"}` + "\n"
	loaded, err := consumeLoad(context.Background(), strings.NewReader(in), &sink{})
	if err != nil || len(loaded) != 2 || loaded[0] != "localhost:5000/a:1" || loaded[1] != "sha256:abc" {
		t.Errorf("%v %v", loaded, err)
	}
	_, err = consumeLoad(context.Background(), strings.NewReader(`{"errorDetail":{"message":"invalid tar header"}}`), &sink{})
	if err == nil || !strings.Contains(err.Error(), "invalid tar header") {
		t.Errorf("%v", err)
	}
}

func TestValidateSavePath(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		path string
		ok   bool
	}{
		{"relative path", "rel.tar", false},
		{"existing directory", dir, false},
		{"missing parent folder", filepath.Join(dir, "nodir", "x.tar"), false},
		{"new file in existing folder", filepath.Join(dir, "x.tar"), true},
	}
	for _, c := range cases {
		if _, err := validateSavePath(c.path); (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestValidateLoadPath(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := validateLoadPath(filepath.Join(dir, "none.tar")); err == nil {
		t.Error("missing file must be rejected")
	}
	f := filepath.Join(dir, "a.tar")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, size, err := validateLoadPath(f); err != nil || size != 1 {
		t.Errorf("size=%d err=%v", size, err)
	}
}
