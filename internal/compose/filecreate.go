package compose

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

// CreateFile writes the data under a temp name next to the target and hard-links it into place.
// A link never replaces an existing file, so a file that appears meanwhile is kept, and a write
// that fails or times out leaves nothing at the real path.
func (w WSLFS) CreateFile(ctx context.Context, p string, data []byte) error {
	if err := w.check(p); err != nil {
		return err
	}
	if w.test(ctx, "-e", p) {
		return fmt.Errorf("%s: %w", p, ErrFileExists)
	}
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return err
	}
	tmp := path.Join(path.Dir(p), "."+path.Base(p)+"."+hex.EncodeToString(rnd[:])+".tmp")
	defer w.cleanup(tmp)
	if _, err := w.run(ctx, data, "dd", "of="+tmp, "status=none"); err != nil {
		return err
	}
	if _, err := w.run(ctx, nil, "ln", "--", tmp, p); err != nil {
		if w.test(ctx, "-e", p) {
			return fmt.Errorf("%s: %w", p, ErrFileExists)
		}
		return err
	}
	return nil
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
