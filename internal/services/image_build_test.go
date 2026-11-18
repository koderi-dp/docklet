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

	"docklet/internal/model"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func tarNames(t *testing.T, dir string, entries []string) map[string]string {
	t.Helper()
	var buf bytes.Buffer
	if err := writeBuildContext(context.Background(), &buf, dir, entries); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	tr := tar.NewReader(&buf)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(tr)
		out[h.Name] = string(b)
	}
	return out
}

func TestBuildContext_HonoursDockerignore(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Dockerfile":          "FROM busybox\nCOPY app.txt /app.txt\n",
		"app.txt":             "hello",
		"secret.env":          "TOKEN=1",
		"node_modules/a/b.js": "x",
		"logs/keep.log":       "k",
		"logs/drop.log":       "d",
		"src/main.go":         "package main",
		".dockerignore":       "# comment\nsecret.env\nnode_modules\n**/*.log\n!logs/keep.log\nDockerfile\n",
	})
	entries, err := buildContextEntries(dir, "Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	got := tarNames(t, dir, entries)
	for _, want := range []string{"Dockerfile", ".dockerignore", "app.txt", "src/main.go", "logs/keep.log"} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing %q in %v", want, entries)
		}
	}
	for _, bad := range []string{"secret.env", "node_modules/a/b.js", "node_modules/", "logs/drop.log"} {
		if _, ok := got[bad]; ok {
			t.Errorf("%q must be excluded", bad)
		}
	}
	if got["app.txt"] != "hello" {
		t.Errorf("content: %q", got["app.txt"])
	}
}

func TestBuildContext_PerDockerfileIgnoreWins(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"build/Dockerfile":              "FROM busybox\n",
		"build/Dockerfile.dockerignore": "drop.txt\n",
		"drop.txt":                      "d",
		"a.txt":                         "a",
		".dockerignore":                 "a.txt\n",
	})
	entries, err := buildContextEntries(dir, "build/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	got := tarNames(t, dir, entries)
	if _, ok := got["drop.txt"]; ok {
		t.Error("drop.txt must be excluded by the Dockerfile-specific ignore file")
	}
	if _, ok := got["a.txt"]; !ok {
		t.Error("the plain .dockerignore must not apply when a per-Dockerfile one exists")
	}
	if _, ok := got["build/Dockerfile"]; !ok {
		t.Error("Dockerfile missing")
	}
}

func TestValidateBuildSpec_RejectsEscapingPathsAndBadValues(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"Dockerfile": "FROM busybox", "sub/Dockerfile.dev": "FROM busybox"})
	ok := model.BuildSpec{ContextDir: dir, Tags: []string{"docklet-test:1", " docklet-test:1 ", ""}, BuildArgs: map[string]string{"A": "1", "EMPTY": ""},
		Target: "final", Platform: "linux/arm64/v8"}
	v, err := validateBuildSpec(ok)
	if err != nil {
		t.Fatal(err)
	}
	if v.Dockerfile != "Dockerfile" || len(v.Tags) != 1 || v.Platform == nil || v.Platform.Variant != "v8" || *v.BuildArgs["EMPTY"] != "" {
		t.Errorf("unexpected: %+v", v)
	}
	if v2, err := validateBuildSpec(model.BuildSpec{ContextDir: dir, Dockerfile: "sub/Dockerfile.dev"}); err != nil || v2.Dockerfile != "sub/Dockerfile.dev" {
		t.Errorf("subdir dockerfile: %+v %v", v2, err)
	}
	bad := map[string]model.BuildSpec{
		"empty context":    {},
		"relative context": {ContextDir: "foo"},
		"missing context":  {ContextDir: filepath.Join(dir, "nope")},
		"file as context":  {ContextDir: filepath.Join(dir, "Dockerfile")},
		"escaping df":      {ContextDir: dir, Dockerfile: "../Dockerfile"},
		"escaping df 2":    {ContextDir: dir, Dockerfile: "sub/../../x"},
		"absolute df":      {ContextDir: dir, Dockerfile: filepath.Join(dir, "Dockerfile")},
		"missing df":       {ContextDir: dir, Dockerfile: "Nope"},
		"dir as df":        {ContextDir: dir, Dockerfile: "sub"},
		"bad tag":          {ContextDir: dir, Tags: []string{"Bad Tag"}},
		"digest tag":       {ContextDir: dir, Tags: []string{"busybox@sha256:" + strings.Repeat("a", 64)}},
		"bad arg":          {ContextDir: dir, BuildArgs: map[string]string{"a b": "1"}},
		"bad target":       {ContextDir: dir, Target: "x;y"},
		"bad platform":     {ContextDir: dir, Platform: "linux"},
	}
	for name, spec := range bad {
		if _, err := validateBuildSpec(spec); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestConsumeBuild_ReportsStepsAndSurfacesFailureAndCancel(t *testing.T) {
	okStream := `{"stream":"Step 1/2 : FROM busybox\n"}
{"stream":" ---> abc\nStep 2/2 : COPY a /a\n"}
{"aux":{"ID":"sha256:deadbeef"}}
{"stream":"Successfully built deadbeef\n"}
`
	s := &sink{}
	if err := consumeBuild(context.Background(), strings.NewReader(okStream), s); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(s.logs, "|")
	for _, want := range []string{"Step 1/2 : FROM busybox", "Step 2/2 : COPY a /a", "Successfully built deadbeef", "Built image sha256:deadbeef"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	if len(s.progress) < 3 || s.progress[0] != 0 || s.progress[1] != 0.5 || s.progress[len(s.progress)-1] != 1 {
		t.Errorf("progress: %v", s.progress)
	}

	failStream := `{"stream":"Step 1/1 : RUN false\n"}
{"errorDetail":{"code":1,"message":"The command '/bin/sh -c false' returned a non-zero code: 1"},"error":"The command '/bin/sh -c false' returned a non-zero code: 1"}
`
	s = &sink{}
	err := consumeBuild(context.Background(), strings.NewReader(failStream), s)
	if err == nil || !strings.Contains(err.Error(), "returned a non-zero code: 1") {
		t.Errorf("error detail not surfaced: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := consumeBuild(ctx, strings.NewReader(`{"stream":"x\n"}`+"\n"+`{"str`), &sink{}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancel: %v", err)
	}
}

func TestBaseImageRegistries_SkipsStagesScratchAndArgs(t *testing.T) {
	df := []byte("FROM --platform=linux/amd64 ghcr.io/o/base:1 AS build\nFROM build AS x\nFROM scratch\nFROM busybox\nFROM ${BASE}\nFROM ghcr.io/o/other\n")
	got := baseImageRegistries(df)
	if strings.Join(got, ",") != "ghcr.io,docker.io" {
		t.Errorf("got %v", got)
	}
}
