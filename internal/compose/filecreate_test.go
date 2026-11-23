package compose

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLocalCreateFile_NeverReplaces(t *testing.T) {
	p := filepath.Join(t.TempDir(), "compose.yaml")
	ctx := context.Background()
	if err := (LocalFS{}).CreateFile(ctx, p, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != "first" {
		t.Fatalf("got %q", got)
	}
	err := (LocalFS{}).CreateFile(ctx, p, []byte("second"))
	if !errors.Is(err, ErrFileExists) {
		t.Fatalf("want ErrFileExists, got %v", err)
	}
	if got, _ := os.ReadFile(p); string(got) != "first" {
		t.Fatalf("the existing file was changed: %q", got)
	}
}

func TestLocalMakeDir_ReportsWhetherItCreated(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dir := filepath.Join(root, "a", "b")
	created, err := (LocalFS{}).MakeDir(ctx, dir)
	if err != nil || !created {
		t.Fatalf("first: created=%v err=%v", created, err)
	}
	if st, serr := os.Stat(dir); serr != nil || !st.IsDir() {
		t.Fatalf("not a folder: %v", serr)
	}
	created, err = (LocalFS{}).MakeDir(ctx, dir)
	if err != nil || created {
		t.Fatalf("second: created=%v err=%v", created, err)
	}
	file := filepath.Join(root, "f")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	if _, err := (LocalFS{}).MakeDir(ctx, file); err == nil {
		t.Fatal("want an error for a path that is a file")
	}
}

func TestLocalRemove_FileAndEmptyDirOnly(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	file := filepath.Join(root, "f")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	if err := (LocalFS{}).Remove(ctx, file); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("the file is still there")
	}
	empty := filepath.Join(root, "empty")
	_ = os.Mkdir(empty, 0o755)
	if err := (LocalFS{}).Remove(ctx, empty); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root, "full")
	_ = os.Mkdir(full, 0o755)
	_ = os.WriteFile(filepath.Join(full, "keep"), []byte("x"), 0o600)
	if err := (LocalFS{}).Remove(ctx, full); err == nil {
		t.Fatal("want an error for a folder that is not empty")
	}
	if _, err := os.Stat(filepath.Join(full, "keep")); err != nil {
		t.Fatal("the folder's content was removed")
	}
}

// absentRunner records commands and fails every `test`, as if nothing existed yet.
type absentRunner struct{ cmds []Command }

func (r *absentRunner) Run(context.Context, Command, func(string)) error { return nil }
func (r *absentRunner) Output(_ context.Context, c Command) (string, error) {
	r.cmds = append(r.cmds, c)
	if contains(c.Args, "test") {
		return "", errors.New("exit status 1")
	}
	return "", nil
}

// scriptRunner records commands and fails the ones fail() picks.
type scriptRunner struct {
	cmds []Command
	fail func(c Command) bool
}

func (r *scriptRunner) Run(context.Context, Command, func(string)) error { return nil }
func (r *scriptRunner) Output(_ context.Context, c Command) (string, error) {
	r.cmds = append(r.cmds, c)
	if r.fail != nil && r.fail(c) {
		return "", errors.New("exit status 1")
	}
	return "", nil
}

func isCmd(c Command, name string) bool { return len(c.Args) > 3 && c.Args[3] == name }

// The file is written under a temp name and then hard-linked into place: a link never replaces
// an existing file, and a failed write leaves nothing at the real path.
func TestWSLCreateFile_WritesTempThenLinks(t *testing.T) {
	r := &scriptRunner{fail: func(c Command) bool { return isCmd(c, "test") }} // nothing exists yet
	data := []byte("services: {}\n")
	if err := (WSLFS{Distro: "Ubuntu", Runner: r}).CreateFile(context.Background(), "/p/compose.yaml", data); err != nil {
		t.Fatal(err)
	}
	if len(r.cmds) != 4 {
		t.Fatalf("got %v", r.cmds)
	}
	pre := []string{"wsl.exe", "-d", "Ubuntu", "--exec"}
	dd := wslArgs(r.cmds[1])
	tmp := strings.TrimPrefix(dd[len(dd)-2], "of=")
	if !reflect.DeepEqual(dd[:5], append(append([]string{}, pre...), "dd")) || dd[len(dd)-1] != "status=none" || !bytes.Equal(r.cmds[1].Stdin, data) {
		t.Fatalf("dd: %v", dd)
	}
	if !strings.HasPrefix(tmp, "/p/.compose.yaml.") || !strings.HasSuffix(tmp, ".tmp") {
		t.Fatalf("tmp %q", tmp)
	}
	if want := append(append([]string{}, pre...), "ln", "--", tmp, "/p/compose.yaml"); !reflect.DeepEqual(wslArgs(r.cmds[2]), want) {
		t.Fatalf("ln: %v", wslArgs(r.cmds[2]))
	}
	if want := append(append([]string{}, pre...), "rm", "-f", "--", tmp); !reflect.DeepEqual(wslArgs(r.cmds[3]), want) {
		t.Fatalf("rm: %v", wslArgs(r.cmds[3]))
	}
	for _, c := range r.cmds {
		if contains(c.Args, "sh") {
			t.Fatalf("no shell may be involved: %v", c.Args)
		}
	}
}

func TestWSLCreateFile_FailedWriteLeavesNothing(t *testing.T) {
	r := &scriptRunner{fail: func(c Command) bool { return isCmd(c, "test") || isCmd(c, "dd") }}
	err := (WSLFS{Distro: "Ubuntu", Runner: r}).CreateFile(context.Background(), "/p/compose.yaml", []byte("x"))
	if err == nil || errors.Is(err, ErrFileExists) {
		t.Fatalf("want the write error, got %v", err)
	}
	last := wslArgs(r.cmds[len(r.cmds)-1])
	if len(last) < 7 || last[4] != "rm" || !strings.HasSuffix(last[len(last)-1], ".tmp") {
		t.Fatalf("the temp file must be removed: %v", r.cmds)
	}
	for _, c := range r.cmds {
		if isCmd(c, "ln") {
			t.Fatal("nothing may be linked into place after a failed write")
		}
	}
}

func TestWSLCreateFile_FileAppearsDuringWrite(t *testing.T) {
	tests := 0
	r := &scriptRunner{fail: func(c Command) bool {
		if isCmd(c, "test") {
			tests++
			return tests == 1 // absent before the write, present after the link failed
		}
		return isCmd(c, "ln")
	}}
	err := (WSLFS{Distro: "Ubuntu", Runner: r}).CreateFile(context.Background(), "/p/compose.yaml", []byte("x"))
	if !errors.Is(err, ErrFileExists) {
		t.Fatalf("want ErrFileExists, got %v", err)
	}
	last := wslArgs(r.cmds[len(r.cmds)-1])
	if last[4] != "rm" || !strings.HasSuffix(last[len(last)-1], ".tmp") {
		t.Fatalf("the temp file must be removed: %v", r.cmds)
	}
}

func TestWSLCreateFile_ExistingFile(t *testing.T) {
	r := &recRunner{} // test -e succeeds
	err := (WSLFS{Distro: "Ubuntu", Runner: r}).CreateFile(context.Background(), "/p/compose.yaml", []byte("x"))
	if !errors.Is(err, ErrFileExists) {
		t.Fatalf("want ErrFileExists, got %v", err)
	}
	if len(r.cmds) != 1 || !isCmd(r.cmds[0], "test") {
		t.Fatalf("nothing may be written: %v", r.cmds)
	}
}

func TestWSLMakeDirAndRemove(t *testing.T) {
	ctx := context.Background()
	pre := []string{"wsl.exe", "-d", "Ubuntu", "--exec"}
	r := &absentRunner{}
	created, err := (WSLFS{Distro: "Ubuntu", Runner: r}).MakeDir(ctx, "/p")
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if want := append(append([]string{}, pre...), "mkdir", "-p", "--", "/p"); len(r.cmds) != 2 || !reflect.DeepEqual(wslArgs(r.cmds[1]), want) {
		t.Fatalf("mkdir: %v", r.cmds)
	}
	re := &recRunner{}
	if created, err := (WSLFS{Distro: "Ubuntu", Runner: re}).MakeDir(ctx, "/p"); err != nil || created || len(re.cmds) != 1 {
		t.Fatalf("existing folder: created=%v err=%v cmds=%v", created, err, re.cmds)
	}

	rd := &recRunner{} // test -d succeeds: a folder
	if err := (WSLFS{Distro: "Ubuntu", Runner: rd}).Remove(ctx, "/p"); err != nil {
		t.Fatal(err)
	}
	if want := append(append([]string{}, pre...), "rmdir", "--", "/p"); !reflect.DeepEqual(wslArgs(rd.cmds[len(rd.cmds)-1]), want) {
		t.Fatalf("rmdir: %v", rd.cmds)
	}
	rf := &absentRunner{} // not a folder
	if err := (WSLFS{Distro: "Ubuntu", Runner: rf}).Remove(ctx, "/p/compose.yaml"); err != nil {
		t.Fatal(err)
	}
	if want := append(append([]string{}, pre...), "rm", "-f", "--", "/p/compose.yaml"); !reflect.DeepEqual(wslArgs(rf.cmds[len(rf.cmds)-1]), want) {
		t.Fatalf("rm: %v", rf.cmds)
	}
}

func TestWSLCreate_RefusesBadInput(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ distro, path string }{
		{"Ubuntu", "relative/compose.yaml"},
		{"Ubuntu", "-rf"},
		{"bad distro!", "/p/compose.yaml"},
	} {
		r := &recRunner{}
		w := WSLFS{Distro: tc.distro, Runner: r}
		if err := w.CreateFile(ctx, tc.path, []byte("x")); err == nil {
			t.Fatalf("CreateFile(%q, %q): want an error", tc.distro, tc.path)
		}
		if _, err := w.MakeDir(ctx, tc.path); err == nil {
			t.Fatalf("MakeDir(%q, %q): want an error", tc.distro, tc.path)
		}
		if err := w.Remove(ctx, tc.path); err == nil {
			t.Fatalf("Remove(%q, %q): want an error", tc.distro, tc.path)
		}
		if len(r.cmds) != 0 {
			t.Fatalf("the runner was called: %v", r.cmds)
		}
	}
}

func TestJoinPath(t *testing.T) {
	if got := JoinPath(KindWSL, "/a/b", "compose.yaml"); got != "/a/b/compose.yaml" {
		t.Fatalf("wsl: %q", got)
	}
	if got, want := JoinPath(KindLocal, filepath.Join("a", "b"), "compose.yaml"), filepath.Join("a", "b", "compose.yaml"); got != want {
		t.Fatalf("local: %q", got)
	}
}
