package compose

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLocalCopyTree_CopiesEveryFileAndFolderIncludingEmptyOnes(t *testing.T) {
	ctx := context.Background()
	src, dst := filepath.Join(t.TempDir(), "shop"), filepath.Join(t.TempDir(), "new", "shop-copy")
	writeTree(t, src, map[string]string{"compose.yaml": "services: {}\n", ".env": "A=1\n", "web/src/app.js": "x", "web/Dockerfile": "FROM a"})
	if err := os.MkdirAll(filepath.Join(src, "data", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (LocalFS{}).CopyTree(ctx, src, dst); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{"compose.yaml": "services: {}\n", ".env": "A=1\n", "web/src/app.js": "x", "web/Dockerfile": "FROM a"} {
		got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Errorf("%s: %q, %v", rel, got, err)
		}
	}
	if st, err := os.Stat(filepath.Join(dst, "data", "empty")); err != nil || !st.IsDir() {
		t.Errorf("empty folder not copied: %v", err)
	}
	// The original is untouched.
	if got, _ := os.ReadFile(filepath.Join(src, "web", "src", "app.js")); string(got) != "x" {
		t.Error("source changed")
	}
}

func TestLocalCopyTree_RefusesADestinationThatHasContent(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeTree(t, src, map[string]string{"compose.yaml": "a"})
	writeTree(t, dst, map[string]string{"keep.txt": "mine"})
	err := (LocalFS{}).CopyTree(context.Background(), src, dst)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "compose.yaml")); err == nil {
		t.Error("copied into a folder with content")
	}
}

func TestLocalCopyTree_StopsWhenCancelled(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "copy")
	writeTree(t, src, map[string]string{"a": "1", "b": "2"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (LocalFS{}).CopyTree(ctx, src, dst); err == nil {
		t.Error("a cancelled copy must fail")
	}
}

func TestLocalTreeSize_CountsFilesAndBytes(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]string{"a.txt": "12345", "sub/b.txt": "123"})
	n, files, err := (LocalFS{}).TreeSize(context.Background(), src)
	if err != nil || n != 8 || files != 2 {
		t.Errorf("got %d bytes, %d files, %v", n, files, err)
	}
}

func TestLocalDirEmptyAndRemoveTree(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "x", "copy")
	writeTree(t, dir, map[string]string{"sub/a": "1"})
	if (LocalFS{}).DirEmpty(ctx, dir) || !(LocalFS{}).DirEmpty(ctx, t.TempDir()) {
		t.Error("DirEmpty is wrong")
	}
	if err := (LocalFS{}).RemoveTree(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("folder still there")
	}
	// A drive root or a top-level folder is never removed as a tree.
	for _, p := range []string{`C:\`, `/`, `/home`, `C:\Source`, "relative/x"} {
		if err := (LocalFS{}).RemoveTree(ctx, p); err == nil {
			t.Errorf("RemoveTree(%q) must be refused", p)
		}
	}
}

// treeRunner records commands and answers Output from a function.
type treeRunner struct {
	cmds []Command
	out  func(Command) (string, error)
}

func (s *treeRunner) Run(_ context.Context, c Command, _ func(string)) error {
	s.cmds = append(s.cmds, c)
	_, err := s.answer(c)
	return err
}

func (s *treeRunner) Output(_ context.Context, c Command) (string, error) {
	s.cmds = append(s.cmds, c)
	return s.answer(c)
}

func (s *treeRunner) answer(c Command) (string, error) {
	if s.out != nil {
		return s.out(c)
	}
	return "", nil
}

func TestWSLCopyTree_CopiesInsideTheDistributionWithCp(t *testing.T) {
	r := &treeRunner{out: func(c Command) (string, error) {
		if strings.Contains(strings.Join(c.Args, " "), "--exec test") {
			return "", os.ErrNotExist // the destination does not exist yet
		}
		return "", nil
	}}
	w := WSLFS{Runner: r, Distro: "Ubuntu"}
	if err := w.CopyTree(context.Background(), "/home/u/shop", "/home/u/shop-copy"); err != nil {
		t.Fatal(err)
	}
	last := r.cmds[len(r.cmds)-1]
	if got := strings.Join(last.Args, " "); last.Name != "wsl.exe" || got != "-d Ubuntu --exec cp -a -- /home/u/shop/. /home/u/shop-copy" {
		t.Errorf("copy ran as %s %s", last.Name, got)
	}
}

func TestWSLRemoveTree_RefusesShallowPaths(t *testing.T) {
	r := &treeRunner{}
	w := WSLFS{Runner: r, Distro: "Ubuntu"}
	for _, p := range []string{"/", "/home", "/mnt/c", "home/x/y"} {
		if err := w.RemoveTree(context.Background(), p); err == nil {
			t.Errorf("RemoveTree(%q) must be refused", p)
		}
	}
	if len(r.cmds) != 0 {
		t.Errorf("commands ran: %+v", r.cmds)
	}
	if err := w.RemoveTree(context.Background(), "/home/u/shop-copy"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.cmds[0].Args, " "); got != "-d Ubuntu --exec rm -rf -- /home/u/shop-copy" {
		t.Errorf("ran %s", got)
	}
}

func TestWSLTreeSize_ReadsBytesAndFileCount(t *testing.T) {
	r := &treeRunner{out: func(Command) (string, error) { return "4096\n12\n", nil }}
	n, files, err := (WSLFS{Runner: r, Distro: "Ubuntu"}).TreeSize(context.Background(), "/home/u/shop")
	if err != nil || n != 4096 || files != 12 {
		t.Errorf("got %d, %d, %v", n, files, err)
	}
}

const collisionConfig = `{
  "name": "shop",
  "services": {
    "web": {"image": "a", "container_name": "shop-web",
            "ports": [{"mode":"ingress","target":80,"published":"8080","protocol":"tcp"}, {"target":9000,"protocol":"tcp"}],
            "volumes": [{"type":"bind","source":"/srv/shared/conf","target":"/etc/conf"},
                        {"type":"bind","source":"/home/u/shop/web","target":"/app"},
                        {"type":"volume","source":"data","target":"/data"}]},
    "db":  {"image": "b"}
  },
  "volumes": {
    "data":  {"name": "shop_data"},
    "fixed": {"name": "company-db"},
    "ext":   {"name": "shared", "external": true}
  }
}`

func TestParseCloneCollisions_ListsWhatACloneWouldShareOrFightOver(t *testing.T) {
	got, err := ParseCloneCollisions(collisionConfig, "/home/u/shop")
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{"host port 8080", `container_name "shop-web"`, `"company-db"`, `"shared"`, "/srv/shared/conf"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	for _, not := range []string{"9000", "shop_data", "/home/u/shop/web"} {
		if strings.Contains(all, not) {
			t.Errorf("%q is not a collision:\n%s", not, all)
		}
	}
}

func TestParseCloneCollisions_NothingToReportForAPlainProject(t *testing.T) {
	got, err := ParseCloneCollisions(`{"name":"a","services":{"web":{"image":"x","ports":[{"target":80}]}}}`, `C:\apps\a`)
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v", got, err)
	}
}
