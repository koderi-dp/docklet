package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"docklet/internal/model"
	"docklet/internal/ops"
)

// pickHelperImage chooses an image already present on the engine: busybox/alpine if
// available, else the smallest local image. It never pulls.
func pickHelperImage(images []helperImage) (string, error) {
	if len(images) == 0 {
		return "", errors.New("This engine has no local images to use for the temporary helper container. Pull busybox first (Images > Pull image), then try again.")
	}
	isPreferred := func(ref string) bool {
		name := ref
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		return strings.HasPrefix(name, "busybox:") || strings.HasPrefix(name, "alpine:")
	}
	sorted := append([]helperImage(nil), images...)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi, pj := isPreferred(sorted[i].Ref), isPreferred(sorted[j].Ref)
		if pi != pj {
			return pi
		}
		return sorted[i].Size < sorted[j].Size
	})
	return sorted[0].Ref, nil
}

func helperName() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return "docklet-helper-" + hex.EncodeToString(b)
}

// sweepHelpers removes stale helper containers (left behind by a crash) that are not in use.
func (s *FileService) sweepHelpers(ctx context.Context, connectionID string, api filesAPI) {
	hs, err := api.ListHelpers(ctx)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-helperStaleAge).Unix()
	for _, h := range hs {
		if _, busy := s.active.Load(connectionID + "/" + h.ID); busy || h.Created > cutoff {
			continue
		}
		_ = api.RemoveContainer(ctx, h.ID)
	}
}

// withHelper creates a temporary STOPPED helper container that mounts the volume at /volume,
// runs fn and always removes the helper again (also on error or cancellation).
func (s *FileService) withHelper(ctx context.Context, connectionID string, api filesAPI, volume string, readOnly bool, fn func(id string) error) error {
	if !volumeNameRe.MatchString(volume) {
		return fmt.Errorf("invalid volume name %q", volume)
	}
	s.sweepHelpers(ctx, connectionID, api)
	images, err := api.Images(ctx)
	if err != nil {
		return friendlyError(err)
	}
	img, err := pickHelperImage(images)
	if err != nil {
		return err
	}
	id, err := api.CreateHelper(ctx, img, volume, helperName(), readOnly)
	if err != nil {
		return fileError(err)
	}
	key := connectionID + "/" + id
	s.active.Store(key, struct{}{})
	defer func() {
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer cancel()
		_ = api.RemoveContainer(rctx, id)
		s.active.Delete(key)
	}()
	return fn(id)
}

// volPath maps a volume-relative path ("/" = volume root) to the path inside the helper.
func volPath(p string, allowRoot bool) (rel, real string, err error) {
	rel, err = cleanContainerPath(p, allowRoot)
	if err != nil {
		return "", "", err
	}
	if rel == "/" {
		return rel, helperMountPath, nil
	}
	return rel, helperMountPath + rel, nil
}

func unmapEntryPath(p string) string {
	if p == helperMountPath {
		return "/"
	}
	return strings.TrimPrefix(p, helperMountPath)
}

// VolumeBrowse lists a directory of a volume (read-only helper container, removed afterwards).
func (s *FileService) VolumeBrowse(ctx context.Context, connectionID, volume, p string) (model.FileListing, error) {
	rel, real, err := volPath(p, true)
	if err != nil {
		return model.FileListing{}, err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.FileListing{}, err
	}
	var out model.FileListing
	err = s.withHelper(ctx, connectionID, api, volume, true, func(id string) error {
		l, e := listDir(ctx, api, id, real, false)
		if e != nil {
			return e
		}
		l.Path = rel
		for i := range l.Entries {
			l.Entries[i].Path = unmapEntryPath(l.Entries[i].Path)
		}
		out = l
		return nil
	})
	return out, err
}

// VolumeReadText previews a small text file of a volume.
func (s *FileService) VolumeReadText(ctx context.Context, connectionID, volume, p string, maxBytes int) (model.FileText, error) {
	rel, real, err := volPath(p, false)
	if err != nil {
		return model.FileText{}, err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.FileText{}, err
	}
	var out model.FileText
	err = s.withHelper(ctx, connectionID, api, volume, true, func(id string) error {
		var e error
		out, e = readText(ctx, api, id, real, rel, maxBytes)
		return e
	})
	return out, err
}

// VolumeDownload saves a file (or directory as .tar) of a volume to a local path.
func (s *FileService) VolumeDownload(ctx context.Context, connectionID, volume, p, destPath string) (model.Operation, error) {
	rel, real, err := volPath(p, true)
	if err != nil {
		return model.Operation{}, err
	}
	if err := validateLocalPath(destPath); err != nil {
		return model.Operation{}, err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	return s.startOp(KindFilesDownload, "Download "+volume+":"+rel, connectionID, volume, func(ctx context.Context, h *ops.Handle) error {
		return s.withHelper(ctx, connectionID, api, volume, true, func(id string) error {
			return downloadPath(ctx, h, api, id, real, rel, destPath)
		})
	})
}

// VolumeUpload copies a local file or directory into destDir of a volume (read-write helper).
func (s *FileService) VolumeUpload(ctx context.Context, connectionID, volume, destDir, srcPath string) (model.Operation, error) {
	_, real, err := volPath(destDir, true)
	if err != nil {
		return model.Operation{}, err
	}
	if err := validateLocalPath(srcPath); err != nil {
		return model.Operation{}, err
	}
	if _, err := os.Lstat(srcPath); err != nil {
		return model.Operation{}, fmt.Errorf("cannot read %s: %w", srcPath, err)
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	return s.startOp(KindFilesUpload, "Upload "+filepath.Base(srcPath)+" to "+volume, connectionID, volume, func(ctx context.Context, h *ops.Handle) error {
		return s.withHelper(ctx, connectionID, api, volume, false, func(id string) error {
			return uploadPath(ctx, h, api, id, real, srcPath)
		})
	})
}

// VolumeExport writes the whole volume as a tar file.
func (s *FileService) VolumeExport(ctx context.Context, connectionID, volume, destPath string) (model.Operation, error) {
	if err := validateLocalPath(destPath); err != nil {
		return model.Operation{}, err
	}
	if !volumeNameRe.MatchString(volume) {
		return model.Operation{}, fmt.Errorf("invalid volume name %q", volume)
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	return s.startOp(KindVolumeExport, "Export volume "+volume, connectionID, volume, func(ctx context.Context, h *ops.Handle) error {
		return s.withHelper(ctx, connectionID, api, volume, true, func(id string) error {
			return exportTo(ctx, h, destPath, func() (io.ReadCloser, error) {
				body, _, e := api.CopyFrom(ctx, id, path.Join(helperMountPath)+"/.")
				return body, e
			})
		})
	})
}

// VolumeImport extracts a tar (or tar.gz) into the volume. It can overwrite existing data, so
// confirmOverwrite must be true; volumes used by running containers are refused.
func (s *FileService) VolumeImport(ctx context.Context, connectionID, volume, srcTarPath string, confirmOverwrite bool) (model.Operation, error) {
	if !confirmOverwrite {
		return model.Operation{}, errors.New("Importing can overwrite existing files in the volume. Confirm the overwrite to continue.")
	}
	if err := validateLocalPath(srcTarPath); err != nil {
		return model.Operation{}, err
	}
	fi, err := os.Stat(srcTarPath)
	if err != nil {
		return model.Operation{}, fmt.Errorf("cannot read %s: %w", srcTarPath, err)
	}
	if fi.IsDir() {
		return model.Operation{}, errors.New("choose a .tar file, not a directory")
	}
	if !volumeNameRe.MatchString(volume) {
		return model.Operation{}, fmt.Errorf("invalid volume name %q", volume)
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	users, err := api.VolumeRunningUsers(ctx, volume)
	if err != nil {
		return model.Operation{}, friendlyError(err)
	}
	if len(users) > 0 {
		return model.Operation{}, fmt.Errorf("Volume %s is in use by running containers (%s). Stop them before importing.", volume, strings.Join(users, ", "))
	}
	return s.startOp(KindVolumeImport, "Import into volume "+volume, connectionID, volume, func(ctx context.Context, h *ops.Handle) error {
		return s.withHelper(ctx, connectionID, api, volume, false, func(id string) error {
			f, err := os.Open(srcTarPath)
			if err != nil {
				return err
			}
			defer f.Close()
			h.Log("Importing " + srcTarPath + " into volume " + volume)
			pr, pw := io.Pipe()
			tr := &transfer{h: h, total: fi.Size()}
			go func() {
				pw.CloseWithError(sanitizeTar(&countReader{r: f, fn: tr.add}, pw, nil))
			}()
			err = api.CopyTo(ctx, id, helperMountPath, pr)
			_ = pr.CloseWithError(errors.New("import finished"))
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fileError(err)
			}
			h.Progress(-1)
			h.Detail("Imported " + humanBytes(tr.done) + " of archive data")
			return nil
		})
	})
}
