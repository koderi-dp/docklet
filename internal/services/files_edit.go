package services

import (
	"archive/tar"
	"context"
	"docklet/internal/model"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadText returns up to maxBytes (<=256 KiB) of a text file for inline preview.
func (s *FileService) ReadText(ctx context.Context, connectionID, containerID, p string, maxBytes int) (model.FileText, error) {
	c, err := cleanContainerPath(p, false)
	if err != nil {
		return model.FileText{}, err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.FileText{}, err
	}
	return readText(ctx, api, containerID, c, c, maxBytes)
}

func readText(ctx context.Context, api filesAPI, id, realPath, shownPath string, maxBytes int) (model.FileText, error) {
	if maxBytes <= 0 || maxBytes > maxPreviewBytes {
		maxBytes = maxPreviewBytes
	}
	ctx, cancel := context.WithTimeout(ctx, statTimeout)
	defer cancel()
	body, _, err := api.CopyFrom(ctx, id, realPath)
	if err != nil {
		return model.FileText{}, fileError(err)
	}
	defer body.Close()
	tr := tar.NewReader(body)
	h, err := tr.Next()
	if err != nil {
		return model.FileText{}, fileError(err)
	}
	switch h.Typeflag {
	case tar.TypeReg:
	case tar.TypeDir:
		return model.FileText{}, errors.New("This is a directory, not a file.")
	case tar.TypeSymlink:
		return model.FileText{}, fmt.Errorf("This is a symbolic link to %s. Open the target instead.", h.Linkname)
	default:
		return model.FileText{}, errors.New("This is not a regular file, so it cannot be previewed.")
	}
	buf, err := io.ReadAll(io.LimitReader(tr, int64(maxBytes)))
	if err != nil {
		return model.FileText{}, fileError(err)
	}
	if isBinaryContent(buf) {
		return model.FileText{}, errors.New("This looks like a binary file, so no text preview is shown. Download it instead.")
	}
	truncated := h.Size > int64(len(buf))
	if truncated { // do not cut inside a UTF-8 sequence
		for i := 0; i < 3 && len(buf) > 0 && !validUTF8Tail(buf); i++ {
			buf = buf[:len(buf)-1]
		}
	}
	return model.FileText{Path: shownPath, Content: string(buf), Size: h.Size, Truncated: truncated}, nil
}

func validUTF8Tail(b []byte) bool { return !isBinaryContent(b) }

// Mkdir creates parent/name through the archive API (works on stopped containers).
func (s *FileService) Mkdir(ctx context.Context, connectionID, containerID, parent, name string) error {
	pdir, err := cleanContainerPath(parent, true)
	if err != nil {
		return err
	}
	if err := validNewName(name); err != nil {
		return err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return err
	}
	defer s.invalidateIndex(connectionID, containerID)
	return mkdir(ctx, api, containerID, pdir, name)
}

func mkdir(ctx context.Context, api filesAPI, id, parent, name string) error {
	ctx, cancel := context.WithTimeout(ctx, statTimeout)
	defer cancel()
	if _, err := api.StatPath(ctx, id, path.Join(parent, name)); err == nil {
		return fmt.Errorf("%s already exists", path.Join(parent, name))
	}
	if err := api.CopyTo(ctx, id, parent, mkdirTar(name)); err != nil {
		return fileError(err)
	}
	return nil
}

// Delete removes a file or directory tree. The archive API cannot delete, so this needs a running
// container with rm and runs exactly ["rm","-rf","--",path] (no shell).
func (s *FileService) Delete(ctx context.Context, connectionID, containerID, p string) error {
	argv, err := deleteArgv(p)
	if err != nil {
		return err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return err
	}
	defer s.invalidateIndex(connectionID, containerID)
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	running, _, err := api.Running(ctx, containerID)
	if err != nil {
		return friendlyError(err)
	}
	if !running {
		return errors.New("Deleting is not possible for this container: it is not running. Start it first (deleting runs rm inside the container).")
	}
	_, stderr, code, err := api.Exec(ctx, containerID, argv, 64<<10)
	if err != nil {
		return fileError(err)
	}
	switch {
	case code == 0:
		return nil
	case code == 126 || code == 127 || strings.Contains(stderr, "executable file not found"):
		return errors.New("Deleting is not possible for this container: it has no rm command.")
	}
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		msg = "rm exited with code " + strconv.Itoa(code)
	}
	if strings.Contains(strings.ToLower(msg), "read-only") {
		return errors.New(friendlyROFS + " (" + msg + ")")
	}
	return fmt.Errorf("Could not delete %s: %s", argv[len(argv)-1], msg)
}

func writeFileAtomically(dest string, fill func(w io.Writer) error) (err error) {
	if err := validateLocalPath(dest); err != nil {
		return err
	}
	if fi, e := os.Stat(dest); e == nil && fi.IsDir() {
		return fmt.Errorf("%s is a directory; choose a file name", dest)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".docklet-*.part")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if err = fill(tmp); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}
