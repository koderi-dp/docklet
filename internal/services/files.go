package services

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
)

// FileService is bound to the frontend: container file browser (Docker archive API)
// and volume browse/export/import through a temporary helper container.
type FileService struct {
	Engines engine.Provider
	Events  engine.Emitter
	Ops     *ops.Manager

	// newAPI is a test seam; nil uses the Docker SDK client of the connection.
	newAPI func(ctx context.Context, connectionID string) (filesAPI, error)

	active sync.Map // helper container IDs currently in use (never swept)

	idx indexCache // full-tree indexes of non-running containers
}

const (
	KindFilesDownload = "files.download"
	KindFilesUpload   = "files.upload"
	KindFilesExport   = "container.export"
	KindVolumeExport  = "volume.export"
	KindVolumeImport  = "volume.import"

	helperLabel     = "docklet.helper"
	helperMountPath = "/volume"
	helperStaleAge  = 2 * time.Minute

	listTimeout  = 60 * time.Second
	statTimeout  = 20 * time.Second
	execTimeout  = 60 * time.Second
	execListMax  = 8 << 20
	friendlyROFS = "The filesystem is read-only, so files cannot be changed there."
)

func (s *FileService) api(ctx context.Context, connectionID string) (filesAPI, error) {
	if s.newAPI != nil {
		return s.newAPI(ctx, connectionID)
	}
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return nil, err
	}
	return sdkFilesAPI{cli}, nil
}

// fileError turns Engine archive errors into readable messages.
func fileError(err error) error {
	if err == nil {
		return nil
	}
	low := strings.ToLower(err.Error())
	switch {
	case strings.Contains(low, "read-only"), strings.Contains(low, "readonly"):
		return fmt.Errorf("%s (%w)", friendlyROFS, err)
	case strings.Contains(low, "cannot overwrite"):
		return fmt.Errorf("A directory cannot be replaced by a file (or the reverse): %w", err)
	case strings.Contains(low, "no such container"):
		return friendlyError(err)
	case strings.Contains(low, "permission denied"):
		return fmt.Errorf("Permission denied: that path cannot be read inside the container: %w", err)
	case strings.Contains(low, "could not find the file"), strings.Contains(low, "no such file"), cerrdefs.IsNotFound(err):
		return fmt.Errorf("That path does not exist in the container: %w", err)
	case strings.Contains(low, "no such image"):
		return fmt.Errorf("The helper image is no longer available on this engine: %w", err)
	}
	return friendlyError(err)
}

func validateLocalPath(p string) error {
	if strings.TrimSpace(p) == "" {
		return errors.New("no local path was chosen")
	}
	if strings.ContainsRune(p, 0) {
		return errors.New("local path contains a NUL byte")
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("local path %q must be absolute", p)
	}
	return nil
}

func fileEntryFromStat(p string, st container.PathStat) model.FileEntry {
	e := model.FileEntry{
		Name: st.Name, Path: p, IsDir: st.Mode.IsDir(), IsSymlink: st.Mode&fs.ModeSymlink != 0,
		LinkTarget: st.LinkTarget, Size: st.Size, Mode: st.Mode.String(),
	}
	if !st.Mtime.IsZero() {
		e.ModTime = st.Mtime.UnixMilli()
	}
	return e
}

// ---------------------------------------------------------------- container API

// Capabilities reports which file actions are possible for the container and why not.
func (s *FileService) Capabilities(ctx context.Context, connectionID, containerID string) (model.FileCapabilities, error) {
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.FileCapabilities{}, err
	}
	running, name, err := api.Running(ctx, containerID)
	if err != nil {
		return model.FileCapabilities{}, friendlyError(err)
	}
	c := model.FileCapabilities{CanWrite: true, ContainerName: name, Running: running}
	if running {
		c.CanDelete = true
	} else {
		c.DeleteReason = "Deleting files needs a running container (it runs rm inside it). Start the container first."
	}
	return c, nil
}

// Stat describes one path.
func (s *FileService) Stat(ctx context.Context, connectionID, containerID, p string) (model.FileEntry, error) {
	c, err := cleanContainerPath(p, true)
	if err != nil {
		return model.FileEntry{}, err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.FileEntry{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, statTimeout)
	defer cancel()
	if info, ierr := api.Inspect(ctx, containerID); ierr == nil && !info.canExec() {
		if x := s.idx.peek(indexKey(connectionID, containerID), info.stamp()); x != nil {
			return x.stat(c)
		}
	}
	st, err := api.StatPath(ctx, containerID, c)
	if err != nil {
		return model.FileEntry{}, fileError(err)
	}
	return fileEntryFromStat(c, st), nil
}

// ---------------------------------------------------------------- transfers
