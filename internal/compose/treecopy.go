package compose

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// TreeCopier copies a whole project folder where the Compose CLI sees it.
type TreeCopier interface {
	// TreeSize is the number of bytes and files below dir.
	TreeSize(ctx context.Context, dir string) (bytes int64, files int, err error)
	// DirEmpty reports whether dir is a folder with nothing in it.
	DirEmpty(ctx context.Context, dir string) bool
	// CopyTree copies everything below src into dst. dst is created when missing and must be
	// empty otherwise; src is only read.
	CopyTree(ctx context.Context, src, dst string) error
	// RemoveTree deletes dir with its content. It refuses drive roots and top-level folders.
	RemoveTree(ctx context.Context, dir string) error
}

// ErrNotEmpty is returned by CopyTree when the destination already has content.
var ErrNotEmpty = errors.New("the folder is not empty")

func (LocalFS) TreeSize(ctx context.Context, dir string) (int64, int, error) {
	var total int64
	var files int
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable entry does not stop the count
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.Type().IsRegular() {
			if info, ierr := d.Info(); ierr == nil {
				total += info.Size()
				files++
			}
		}
		return nil
	})
	return total, files, err
}

func (LocalFS) DirEmpty(_ context.Context, dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 0
}

func (l LocalFS) CopyTree(ctx context.Context, src, dst string) error {
	if st, err := os.Stat(dst); err == nil {
		if !st.IsDir() || !l.DirEmpty(ctx, dst) {
			return fmt.Errorf("%s: %w", dst, ErrNotEmpty)
		}
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.Type().IsRegular():
			return copyFile(p, target)
		}
		return nil // sockets, devices and pipes are not copied
	})
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, st.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, st.ModTime(), st.ModTime())
}

func (LocalFS) RemoveTree(_ context.Context, dir string) error {
	clean := filepath.Clean(dir)
	rest := strings.Trim(filepath.ToSlash(strings.TrimPrefix(clean, filepath.VolumeName(clean))), "/")
	need := 2
	if filepath.VolumeName(clean) == "" {
		need = 3 // /home/user/x
	}
	if !filepath.IsAbs(clean) || rest == "" || len(strings.Split(rest, "/")) < need {
		return fmt.Errorf("refusing to remove %s: not a folder inside a project location", dir)
	}
	return os.RemoveAll(clean)
}

var mntDriveRe = regexp.MustCompile(`^/mnt/[a-z]/`)

func (w WSLFS) TreeSize(ctx context.Context, dir string) (int64, int, error) {
	if err := w.check(dir); err != nil {
		return 0, 0, err
	}
	out, err := w.run(ctx, nil, "sh", "-c", `du -sb -- "$1" | cut -f1; find "$1" -type f | wc -l`, "sh", dir)
	if err != nil {
		return 0, 0, err
	}
	lines := strings.Fields(out)
	if len(lines) != 2 {
		return 0, 0, fmt.Errorf("unexpected size answer %q", strings.TrimSpace(out))
	}
	n, err1 := strconv.ParseInt(lines[0], 10, 64)
	files, err2 := strconv.Atoi(lines[1])
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("unexpected size answer %q", strings.TrimSpace(out))
	}
	return n, files, nil
}

func (w WSLFS) DirEmpty(ctx context.Context, dir string) bool {
	if w.check(dir) != nil || !w.test(ctx, "-d", dir) {
		return false
	}
	out, err := w.run(ctx, nil, "find", dir, "-mindepth", "1", "-maxdepth", "1", "-print", "-quit")
	return err == nil && strings.TrimSpace(out) == ""
}

// CopyTree runs `cp -a` inside the distribution: permissions, owners, links and times are kept.
// It has no time limit of its own; cancelling ctx stops it.
func (w WSLFS) CopyTree(ctx context.Context, src, dst string) error {
	if err := w.check(src); err != nil {
		return err
	}
	if err := w.check(dst); err != nil {
		return err
	}
	if w.test(ctx, "-e", dst) && !w.DirEmpty(ctx, dst) {
		return fmt.Errorf("%s: %w", dst, ErrNotEmpty)
	}
	if _, err := w.MakeDir(ctx, dst); err != nil {
		return err
	}
	var last string
	err := w.Runner.Run(ctx, Command{Name: "wsl.exe", Args: []string{"-d", w.Distro, "--exec", "cp", "-a", "--", strings.TrimRight(src, "/") + "/.", dst}},
		func(l string) { last = l })
	if err != nil && last != "" {
		return fmt.Errorf("%s (%w)", last, err)
	}
	return err
}

func (w WSLFS) RemoveTree(ctx context.Context, dir string) error {
	if err := w.check(dir); err != nil {
		return err
	}
	clean := strings.TrimRight(dir, "/")
	need := 3 // /home/user/x
	if mntDriveRe.MatchString(clean + "/") {
		need = 4 // /mnt/c/folder/x
	}
	if len(strings.Split(strings.Trim(clean, "/"), "/")) < need || strings.Contains(clean, "/../") || strings.HasSuffix(clean, "/..") {
		return fmt.Errorf("refusing to remove %s: not a folder inside a project location", dir)
	}
	_, err := w.run(ctx, nil, "rm", "-rf", "--", clean)
	return err
}
