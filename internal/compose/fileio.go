package compose

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// MaxEditableFile is the largest file the editor opens.
const MaxEditableFile = 1 << 20

// FileIO reads and writes project files where the Compose CLI sees them.
type FileIO interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, data []byte) error
}

func tooLarge(p string) error {
	return fmt.Errorf("%s is larger than %d KB and cannot be edited here", p, MaxEditableFile/1024)
}

// LocalFS (declared in capability.go) reads and writes on this machine.

func (LocalFS) ReadFile(_ context.Context, p string) ([]byte, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("%s is a folder, not a file", p)
	}
	if st.Size() > MaxEditableFile {
		return nil, tooLarge(p)
	}
	return os.ReadFile(p)
}

// WriteFile replaces the file through a temp file in the same directory and a rename. A
// symlink is followed so the linked file is written. On any failure the original stays
// as it was and no temp file is left behind.
func (LocalFS) WriteFile(_ context.Context, p string, data []byte) (err error) {
	target, err := filepath.EvalSymlinks(p)
	if err != nil {
		return err
	}
	st, err := os.Stat(target)
	if err != nil {
		return err
	}
	dir, base := filepath.Split(target)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(name, st.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(name, target)
}

// WSLFS (declared in capability.go) reaches a WSL distribution with wsl.exe.

const wslFileTimeout = 15 * time.Second

func (w WSLFS) check(p string) error {
	if err := ValidateDistro(w.Distro); err != nil {
		return err
	}
	if err := validatePath("path", p); err != nil {
		return err
	}
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("path %q is not an absolute Linux path", p)
	}
	return nil
}

func (w WSLFS) run(ctx context.Context, stdin []byte, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, wslFileTimeout)
	defer cancel()
	a := append([]string{"-d", w.Distro, "--exec"}, args...)
	return w.Runner.Output(ctx, Command{Name: "wsl.exe", Args: a, Stdin: stdin})
}

func (w WSLFS) ReadFile(ctx context.Context, p string) ([]byte, error) {
	if err := w.check(p); err != nil {
		return nil, err
	}
	out, err := w.run(ctx, nil, "cat", "--", p)
	if err != nil {
		return nil, err
	}
	if len(out) > MaxEditableFile {
		return nil, tooLarge(p)
	}
	return []byte(out), nil
}

func (w WSLFS) WriteFile(ctx context.Context, p string, data []byte) error {
	if err := w.check(p); err != nil {
		return err
	}
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return err
	}
	tmp := path.Join(path.Dir(p), "."+path.Base(p)+"."+hex.EncodeToString(rnd[:])+".tmp")
	if _, err := w.run(ctx, data, "dd", "of="+tmp, "status=none"); err != nil {
		w.cleanup(tmp)
		return err
	}
	_, _ = w.run(ctx, nil, "chmod", "--reference="+p, tmp)
	if _, err := w.run(ctx, nil, "mv", "-f", "--", tmp, p); err != nil {
		w.cleanup(tmp)
		return err
	}
	return nil
}

// cleanup removes the temp file; it must run even when ctx is already cancelled.
func (w WSLFS) cleanup(tmp string) {
	_, _ = w.run(context.Background(), nil, "rm", "-f", "--", tmp)
}
