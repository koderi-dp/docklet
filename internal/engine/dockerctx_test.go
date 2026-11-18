package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"docklet/internal/model"
)

func writeCtx(t *testing.T, dir, name, host string, tls bool) {
	t.Helper()
	sum := sha256.Sum256([]byte(name))
	h := hex.EncodeToString(sum[:])
	md := filepath.Join(dir, "contexts", "meta", h)
	if err := os.MkdirAll(md, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"Name":"` + name + `","Metadata":{"Description":"d-` + name + `"},"Endpoints":{"docker":{"Host":"` + host + `","SkipTLSVerify":false}}}`
	if err := os.WriteFile(filepath.Join(md, "meta.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if tls {
		if err := os.MkdirAll(filepath.Join(dir, "contexts", "tls", h, "docker"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDockerContext_MapsSupportedSchemesAndNeverImportsInsecure(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"currentContext":"remote"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCtx(t, dir, "pipe", "npipe:////./pipe/dockerDesktopLinuxEngine", false)
	writeCtx(t, dir, "remote", "tcp://docker.example.com:2376", true)
	writeCtx(t, dir, "plain", "tcp://10.0.0.5:2375", false)
	writeCtx(t, dir, "sshy", "ssh://me@host:2222", false)
	writeCtx(t, dir, "weird", "fd://x", false)

	list, err := readDockerContexts(dir)
	if err != nil || len(list) != 5 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	by := map[string]model.DockerContextInfo{}
	for _, c := range list {
		by[c.Name] = c
	}
	if !by["remote"].Current || by["pipe"].Current || by["weird"].Type != "" || by["weird"].Reason == "" {
		t.Fatalf("flags wrong: %+v", by)
	}

	p, err := contextProfile(dir, "pipe")
	if err != nil || p.Type != model.ConnNamedPipe || p.Endpoint != "npipe:////./pipe/dockerDesktopLinuxEngine" {
		t.Fatalf("pipe: %+v %v", p, err)
	}
	p, _ = contextProfile(dir, "sshy")
	if p.Type != model.ConnSSH || p.Endpoint != "ssh://me@host:2222" {
		t.Fatalf("ssh: %+v", p)
	}
	p, _ = contextProfile(dir, "remote")
	if p.Type != model.ConnTCP || !p.TLSVerify || p.TLSCertDir == "" {
		t.Fatalf("tls tcp must reference the context's TLS dir with verification on: %+v", p)
	}
	if _, err := ValidateProfile(p); err != nil {
		t.Fatalf("imported tls profile must validate: %v", err)
	}
	// A plain remote tcp context must not be silently imported as insecure.
	p, _ = contextProfile(dir, "plain")
	if p.AllowInsecure {
		t.Fatal("import must never set AllowInsecure")
	}
	if _, err := ValidateProfile(p); err == nil {
		t.Fatal("plain remote tcp context must be rejected on import")
	}
	if _, err := contextProfile(dir, "weird"); err == nil {
		t.Fatal("unsupported scheme must not import")
	}
	if _, err := contextProfile(dir, "missing"); err == nil {
		t.Fatal("unknown context must error")
	}
	if l, err := readDockerContexts(filepath.Join(dir, "nope")); err != nil || len(l) != 0 {
		t.Fatalf("missing dir must be empty, got %v %v", l, err)
	}
}
