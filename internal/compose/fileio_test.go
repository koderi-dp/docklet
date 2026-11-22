package compose

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLocalWriteReplacesContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(p, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := (LocalFS{}).WriteFile(context.Background(), p, []byte("new content")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "new content" {
		t.Fatalf("got %q", got)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(p)
		if st.Mode().Perm() != 0o640 {
			t.Fatalf("mode %v", st.Mode().Perm())
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestLocalWriteMissingDirectory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope", "compose.yaml")
	if err := (LocalFS{}).WriteFile(context.Background(), p, []byte("x")); err == nil {
		t.Fatal("want an error")
	}
}

func TestLocalReadLimits(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big.yaml")
	if err := os.WriteFile(big, make([]byte, MaxEditableFile+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (LocalFS{}).ReadFile(context.Background(), big); err == nil {
		t.Fatal("want an error for a large file")
	}
	if _, err := (LocalFS{}).ReadFile(context.Background(), dir); err == nil {
		t.Fatal("want an error for a directory")
	}
	ok := filepath.Join(dir, "ok.yaml")
	_ = os.WriteFile(ok, []byte("a: 1\n"), 0o600)
	b, err := (LocalFS{}).ReadFile(context.Background(), ok)
	if err != nil || string(b) != "a: 1\n" {
		t.Fatalf("got %q %v", b, err)
	}
}

type recRunner struct {
	cmds   []Command
	failOn string // Args element that makes the command fail
	out    string
}

func (r *recRunner) Run(context.Context, Command, func(string)) error { return nil }
func (r *recRunner) Output(_ context.Context, c Command) (string, error) {
	r.cmds = append(r.cmds, c)
	for _, a := range c.Args {
		if r.failOn != "" && a == r.failOn {
			return "", errors.New("boom")
		}
	}
	return r.out, nil
}

func wslArgs(c Command) []string { return append([]string{c.Name}, c.Args...) }

func TestWSLRead(t *testing.T) {
	r := &recRunner{out: "a: 1\n"}
	b, err := (WSLFS{Distro: "Ubuntu", Runner: r}).ReadFile(context.Background(), "/home/u/compose.yaml")
	if err != nil || string(b) != "a: 1\n" {
		t.Fatalf("got %q %v", b, err)
	}
	want := []string{"wsl.exe", "-d", "Ubuntu", "--exec", "cat", "--", "/home/u/compose.yaml"}
	if len(r.cmds) != 1 || !reflect.DeepEqual(wslArgs(r.cmds[0]), want) {
		t.Fatalf("got %v", r.cmds)
	}
}

func TestWSLWrite(t *testing.T) {
	r := &recRunner{}
	data := []byte("services: {}\n")
	if err := (WSLFS{Distro: "Ubuntu", Runner: r}).WriteFile(context.Background(), "/home/u/compose.yaml", data); err != nil {
		t.Fatal(err)
	}
	if len(r.cmds) != 3 {
		t.Fatalf("got %v", r.cmds)
	}
	dd := wslArgs(r.cmds[0])
	tmp := strings.TrimPrefix(dd[len(dd)-2], "of=")
	if !reflect.DeepEqual(dd[:5], []string{"wsl.exe", "-d", "Ubuntu", "--exec", "dd"}) || dd[len(dd)-1] != "status=none" || !bytes.Equal(r.cmds[0].Stdin, data) {
		t.Fatalf("dd: %v", dd)
	}
	if !strings.HasPrefix(tmp, "/home/u/.") || !strings.HasSuffix(tmp, ".tmp") {
		t.Fatalf("tmp %q", tmp)
	}
	pre := []string{"wsl.exe", "-d", "Ubuntu", "--exec"}
	if want := append(append([]string{}, pre...), "chmod", "--reference=/home/u/compose.yaml", tmp); !reflect.DeepEqual(wslArgs(r.cmds[1]), want) {
		t.Fatalf("chmod: %v", wslArgs(r.cmds[1]))
	}
	if want := append(append([]string{}, pre...), "mv", "-f", "--", tmp, "/home/u/compose.yaml"); !reflect.DeepEqual(wslArgs(r.cmds[2]), want) {
		t.Fatalf("mv: %v", wslArgs(r.cmds[2]))
	}
}

func TestWSLWriteChmodFailureIgnored(t *testing.T) {
	r := &recRunner{failOn: "chmod"}
	if err := (WSLFS{Distro: "Ubuntu", Runner: r}).WriteFile(context.Background(), "/a/b.yaml", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if last := r.cmds[len(r.cmds)-1]; !contains(last.Args, "mv") {
		t.Fatalf("mv not run: %v", r.cmds)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestWSLWriteDdFailureCleansUp(t *testing.T) {
	r := &recRunner{failOn: "dd"}
	if err := (WSLFS{Distro: "Ubuntu", Runner: r}).WriteFile(context.Background(), "/a/b.yaml", []byte("x")); err == nil {
		t.Fatal("want an error")
	}
	if len(r.cmds) != 2 {
		t.Fatalf("got %v", r.cmds)
	}
	rm := r.cmds[1].Args
	if !contains(rm, "rm") || !contains(rm, "-f") || contains(rm, "mv") {
		t.Fatalf("rm: %v", rm)
	}
}

func TestWSLRefusesBadInput(t *testing.T) {
	r := &recRunner{}
	ctx := context.Background()
	if _, err := (WSLFS{Distro: "Ubuntu", Runner: r}).ReadFile(ctx, "relative/x.yaml"); err == nil {
		t.Fatal("relative read allowed")
	}
	if err := (WSLFS{Distro: "Ubuntu", Runner: r}).WriteFile(ctx, "-rf", nil); err == nil {
		t.Fatal("dash path allowed")
	}
	if _, err := (WSLFS{Distro: "bad name;x", Runner: r}).ReadFile(ctx, "/a/b"); err == nil {
		t.Fatal("bad distro allowed")
	}
	if len(r.cmds) != 0 {
		t.Fatalf("commands ran: %v", r.cmds)
	}
}
