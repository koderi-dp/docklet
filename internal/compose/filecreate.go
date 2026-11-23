package compose

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// ComposeFileNames are the file names Compose looks for in a folder.
var ComposeFileNames = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}

// ErrFileExists is returned by CreateFile when the path is already taken.
var ErrFileExists = errors.New("file already exists")

// FileCreator makes new files where the Compose CLI sees them.
type FileCreator interface {
	// MakeDir creates dir and its missing parents; created is false when dir existed.
	MakeDir(ctx context.Context, dir string) (created bool, err error)
	// CreateFile writes a new file and never replaces one: ErrFileExists when path exists.
	CreateFile(ctx context.Context, path string, data []byte) error
	// Remove deletes one file or one empty folder.
	Remove(ctx context.Context, path string) error
}

// JoinPath joins a folder and a file name for the given execution kind.
func JoinPath(kind Kind, dir, name string) string {
	if kind == KindWSL {
		return path.Join(dir, name)
	}
	return filepath.Join(dir, name)
}

func (LocalFS) MakeDir(_ context.Context, dir string) (bool, error) {
	if st, err := os.Stat(dir); err == nil {
		if !st.IsDir() {
			return false, fmt.Errorf("%s is a file, not a folder", dir)
		}
		return false, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	return true, nil
}

func (LocalFS) CreateFile(_ context.Context, p string, data []byte) error {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s: %w", p, ErrFileExists)
		}
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(p)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(p)
		return err
	}
	return nil
}

// Remove deletes a file or an empty folder; os.Remove refuses a folder with content.
func (LocalFS) Remove(_ context.Context, p string) error { return os.Remove(p) }

func (w WSLFS) MakeDir(ctx context.Context, dir string) (bool, error) {
	if err := w.check(dir); err != nil {
		return false, err
	}
	if w.test(ctx, "-d", dir) {
		return false, nil
	}
	if _, err := w.run(ctx, nil, "mkdir", "-p", "--", dir); err != nil {
		return false, err
	}
	return true, nil
}

// CreateFile writes through a shell with noclobber set, so a file that appears between the
// existence check and the write is still not replaced.
func (w WSLFS) CreateFile(ctx context.Context, p string, data []byte) error {
	if err := w.check(p); err != nil {
		return err
	}
	if w.test(ctx, "-e", p) {
		return fmt.Errorf("%s: %w", p, ErrFileExists)
	}
	_, err := w.run(ctx, data, "sh", "-c", `set -C; cat > "$1"`, "sh", p)
	return err
}

func (w WSLFS) Remove(ctx context.Context, p string) error {
	if err := w.check(p); err != nil {
		return err
	}
	if w.test(ctx, "-d", p) {
		_, err := w.run(ctx, nil, "rmdir", "--", p)
		return err
	}
	_, err := w.run(ctx, nil, "rm", "-f", "--", p)
	return err
}
