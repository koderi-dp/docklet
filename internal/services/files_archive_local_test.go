package services

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type tarEntry struct {
	hdr  tar.Header
	body string
}

func readTar(t *testing.T, b []byte) []tarEntry {
	t.Helper()
	var out []tarEntry
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
		out = append(out, tarEntry{*h, string(body)})
	}
}

func entryMap(es []tarEntry) map[string]tarEntry {
	m := map[string]tarEntry{}
	for _, e := range es {
		m[e.hdr.Name] = e
	}
	return m
}

func buildTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "proj")
	for _, d := range []string{"sub/deep", "empty"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"a.txt": "AAAA", "sub/b.txt": "BB", "sub/deep/c.txt": "", "sp ace.txt": "S"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestWriteLocalTar_DirectoryRoundTrip(t *testing.T) {
	root := buildTree(t)
	var buf bytes.Buffer
	var progress int64
	if err := writeLocalTar(&buf, root, func(n int64) { progress += n }); err != nil {
		t.Fatal(err)
	}
	m := entryMap(readTar(t, buf.Bytes()))
	want := map[string]string{"proj/a.txt": "AAAA", "proj/sub/b.txt": "BB", "proj/sub/deep/c.txt": "", "proj/sp ace.txt": "S"}
	for name, body := range want {
		e, ok := m[name]
		if !ok || e.body != body || e.hdr.Typeflag != tar.TypeReg || e.hdr.Size != int64(len(body)) {
			t.Errorf("%s: %+v ok=%v", name, e, ok)
		}
	}
	for _, d := range []string{"proj/", "proj/sub/", "proj/sub/deep/", "proj/empty/"} {
		if e, ok := m[d]; !ok || e.hdr.Typeflag != tar.TypeDir {
			t.Errorf("directory entry %q missing (empty dirs must survive): %+v", d, e)
		}
	}
	for name := range m {
		if strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			t.Errorf("unsafe or non-portable entry name %q", name)
		}
	}
	if progress != 7 || progress != localSize(root) {
		t.Errorf("progress = %d, localSize = %d, want 7", progress, localSize(root))
	}
}

func TestWriteLocalTar_ParentsPrecedeChildren(t *testing.T) {
	// Extractors need the directory entry before its files.
	var buf bytes.Buffer
	if err := writeLocalTar(&buf, buildTree(t), nil); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range readTar(t, buf.Bytes()) {
		n := strings.TrimSuffix(e.hdr.Name, "/")
		if i := strings.LastIndex(n, "/"); i >= 0 && !seen[n[:i]] {
			t.Errorf("%q appears before its parent directory", e.hdr.Name)
		}
		seen[n] = true
	}
}

func TestWriteLocalTar_SingleFileUsesBaseNameOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "deep", "note.txt")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte("hi"), 0o644)
	var buf bytes.Buffer
	if err := writeLocalTar(&buf, p, nil); err != nil {
		t.Fatal(err)
	}
	es := readTar(t, buf.Bytes())
	if len(es) != 1 || es[0].hdr.Name != "note.txt" || es[0].body != "hi" {
		t.Fatalf("%+v", es)
	}
}

func TestWriteLocalTar_TrailingSeparatorStillNamesTopEntry(t *testing.T) {
	root := buildTree(t)
	var buf bytes.Buffer
	if err := writeLocalTar(&buf, root+string(filepath.Separator), nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := entryMap(readTar(t, buf.Bytes()))["proj/a.txt"]; !ok {
		t.Fatal("trailing separator changed entry names")
	}
}

func TestWriteLocalTar_EmptyDirectoryAndMissingSource(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "void")
	_ = os.Mkdir(empty, 0o755)
	var buf bytes.Buffer
	if err := writeLocalTar(&buf, empty, nil); err != nil {
		t.Fatal(err)
	}
	if es := readTar(t, buf.Bytes()); len(es) != 1 || es[0].hdr.Name != "void/" {
		t.Fatalf("%+v", es)
	}
	buf.Reset()
	if err := writeLocalTar(&buf, filepath.Join(t.TempDir(), "nope"), nil); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteLocalTar_ModesAreNormalisedPerPlatform(t *testing.T) {
	root := filepath.Join(t.TempDir(), "m")
	_ = os.Mkdir(root, 0o755)
	script := filepath.Join(root, "run.sh")
	secret := filepath.Join(root, "secret")
	_ = os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755)
	_ = os.WriteFile(secret, []byte("s"), 0o600)
	_ = os.Chmod(script, 0o755) // defeat umask
	_ = os.Chmod(secret, 0o600)
	var buf bytes.Buffer
	if err := writeLocalTar(&buf, root, nil); err != nil {
		t.Fatal(err)
	}
	m := entryMap(readTar(t, buf.Bytes()))
	wantScript, wantSecret := int64(0o755), int64(0o600)
	if runtime.GOOS == "windows" { // no permission bits: files get a neutral 0644
		wantScript, wantSecret = 0o644, 0o644
	}
	if m["m/run.sh"].hdr.Mode != wantScript || m["m/secret"].hdr.Mode != wantSecret || m["m/"].hdr.Mode != 0o755 {
		t.Fatalf("modes: run.sh=%o secret=%o dir=%o", m["m/run.sh"].hdr.Mode, m["m/secret"].hdr.Mode, m["m/"].hdr.Mode)
	}
}

func TestWriteLocalTar_SymlinksAreStoredNotFollowed(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.txt")
	_ = os.WriteFile(outside, []byte("TOP SECRET"), 0o644)
	root := filepath.Join(t.TempDir(), "d")
	_ = os.Mkdir(root, 0o755)
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	var buf bytes.Buffer
	if err := writeLocalTar(&buf, root, nil); err != nil {
		t.Fatal(err)
	}
	e := entryMap(readTar(t, buf.Bytes()))["d/link"]
	if e.hdr.Typeflag != tar.TypeSymlink || e.hdr.Linkname != filepath.ToSlash(outside) || e.body != "" || e.hdr.Size != 0 {
		t.Fatalf("link entry: %+v", e)
	}
	if got := localSize(root); got != 0 {
		t.Fatalf("localSize followed the symlink: %d", got)
	}
}

func TestWriteLocalTar_OutputPassesImportSanitizer(t *testing.T) {
	var buf, out bytes.Buffer
	if err := writeLocalTar(&buf, buildTree(t), nil); err != nil {
		t.Fatal(err)
	}
	if err := sanitizeTar(&buf, &out, nil); err != nil {
		t.Fatalf("our own archives must be importable: %v", err)
	}
}

type errWriter struct{ after int }

func (w *errWriter) Write(p []byte) (int, error) {
	if w.after <= 0 {
		return 0, errors.New("sink closed")
	}
	w.after -= len(p)
	return len(p), nil
}

func TestWriteLocalTar_SinkFailureIsReturned(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big")
	_ = os.WriteFile(p, bytes.Repeat([]byte("x"), 64<<10), 0o644)
	if err := writeLocalTar(&errWriter{after: 1024}, p, nil); err == nil {
		t.Fatal("write errors must not be swallowed")
	}
}

func TestLocalSize_SumsRegularFilesAndToleratesMissingPaths(t *testing.T) {
	root := buildTree(t)
	if got := localSize(root); got != 7 {
		t.Fatalf("dir = %d", got)
	}
	if got := localSize(filepath.Join(root, "a.txt")); got != 4 {
		t.Fatalf("file = %d", got)
	}
	if got := localSize(filepath.Join(root, "missing")); got != 0 {
		t.Fatalf("missing = %d", got)
	}
}

func TestCountReader_ReportsEveryByteReadAndPassesErrors(t *testing.T) {
	var n int64
	cr := &countReader{r: strings.NewReader("abcdef"), fn: func(k int64) { n += k }}
	b, err := io.ReadAll(cr)
	if err != nil || string(b) != "abcdef" || n != 6 {
		t.Fatalf("b=%q err=%v n=%d", b, err, n)
	}
	boom := errors.New("boom")
	cr = &countReader{r: &failingReader{data: []byte("ab"), err: boom}, fn: func(k int64) { n += k }}
	if _, err := io.ReadAll(cr); !errors.Is(err, boom) || n != 8 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}
