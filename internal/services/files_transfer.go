package services

import (
	"archive/tar"
	"context"
	"docklet/internal/model"
	"docklet/internal/ops"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// throttledDetail reports transferred bytes (real progress only when total is known).
type transfer struct {
	h     *ops.Handle
	total int64
	done  int64
	last  time.Time
}

func (t *transfer) add(n int64) {
	t.done += n
	if time.Since(t.last) < 150*time.Millisecond {
		return
	}
	t.last = time.Now()
	if t.total > 0 {
		t.h.Progress(float64(t.done) / float64(t.total))
		t.h.Detail(humanBytes(t.done) + " of " + humanBytes(t.total))
	} else {
		t.h.Detail(humanBytes(t.done))
	}
}

type funcWriter func(p []byte) (int, error)

func (f funcWriter) Write(p []byte) (int, error) { return f(p) }

func (s *FileService) startOp(kind, title, connectionID, resource string, fn ops.Func) (model.Operation, error) {
	if s.Ops == nil {
		return model.Operation{}, errors.New("operations are not available")
	}
	return s.Ops.Start(ops.Spec{Kind: kind, Title: title, ConnectionID: connectionID, Resource: resource, Cancellable: true}, fn), nil
}

// Download saves a file (as-is) or a directory (as .tar) to a local path.
func (s *FileService) Download(ctx context.Context, connectionID, containerID, p, destPath string) (model.Operation, error) {
	c, err := cleanContainerPath(p, true)
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
	return s.startOp(KindFilesDownload, "Download "+path.Base(c), connectionID, c, func(ctx context.Context, h *ops.Handle) error {
		return downloadPath(ctx, h, api, containerID, c, c, destPath)
	})
}

func downloadPath(ctx context.Context, h *ops.Handle, api filesAPI, id, realPath, shown, dest string) error {
	body, st, err := api.CopyFrom(ctx, id, realPath)
	if err != nil {
		return fileError(err)
	}
	defer body.Close()
	tr := &transfer{h: h}
	if st.Mode.IsDir() {
		if !strings.EqualFold(filepath.Ext(dest), ".tar") {
			dest += ".tar"
		}
		h.Log("Saving " + shown + " as tar to " + dest)
		err = writeFileAtomically(dest, func(w io.Writer) error {
			_, e := io.Copy(funcWriter(func(p []byte) (int, error) { tr.add(int64(len(p))); return w.Write(p) }), body)
			return e
		})
	} else {
		h.Log("Saving " + shown + " to " + dest)
		tr.total = st.Size
		err = writeFileAtomically(dest, func(w io.Writer) error {
			r := tar.NewReader(body)
			hd, e := r.Next()
			if e != nil {
				return e
			}
			if hd.Typeflag == tar.TypeSymlink {
				return fmt.Errorf("%s is a symbolic link to %s; download the target instead", shown, hd.Linkname)
			}
			if hd.Typeflag != tar.TypeReg {
				return fmt.Errorf("%s is not a regular file", shown)
			}
			_, e = io.Copy(funcWriter(func(p []byte) (int, error) { tr.add(int64(len(p))); return w.Write(p) }), r)
			return e
		})
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fileError(err)
	}
	h.Progress(-1)
	h.Detail("Saved " + humanBytes(tr.done))
	return nil
}

// Upload copies a local file or directory into destDir of the container.
func (s *FileService) Upload(ctx context.Context, connectionID, containerID, destDir, srcPath string) (model.Operation, error) {
	d, err := cleanContainerPath(destDir, true)
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
	sctx, cancel := context.WithTimeout(ctx, statTimeout)
	defer cancel()
	st, err := api.StatPath(sctx, containerID, d)
	if err != nil {
		return model.Operation{}, fileError(err)
	}
	if !st.Mode.IsDir() && st.Mode&fs.ModeSymlink == 0 {
		return model.Operation{}, fmt.Errorf("%s is not a directory", d)
	}
	return s.startOp(KindFilesUpload, "Upload "+filepath.Base(srcPath), connectionID, d, func(ctx context.Context, h *ops.Handle) error {
		defer s.invalidateIndex(connectionID, containerID)
		return uploadPath(ctx, h, api, containerID, d, srcPath)
	})
}

func uploadPath(ctx context.Context, h *ops.Handle, api filesAPI, id, destDir, src string) error {
	h.Log("Uploading " + src + " to " + destDir)
	tr := &transfer{h: h, total: localSize(src)}
	pr, pw := io.Pipe()
	go func() { pw.CloseWithError(writeLocalTar(pw, src, tr.add)) }()
	err := api.CopyTo(ctx, id, destDir, pr)
	_ = pr.CloseWithError(errors.New("upload finished"))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fileError(err)
	}
	h.Progress(-1)
	h.Detail("Uploaded " + humanBytes(tr.done))
	return nil
}

// ExportFilesystem writes the container filesystem (docker export) to a tar file.
func (s *FileService) ExportFilesystem(ctx context.Context, connectionID, containerID, destPath string) (model.Operation, error) {
	if err := validateLocalPath(destPath); err != nil {
		return model.Operation{}, err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	return s.startOp(KindFilesExport, "Export filesystem "+filepath.Base(destPath), connectionID, containerID, func(ctx context.Context, h *ops.Handle) error {
		return exportTo(ctx, h, destPath, func() (io.ReadCloser, error) { return api.Export(ctx, containerID) })
	})
}

func exportTo(ctx context.Context, h *ops.Handle, dest string, open func() (io.ReadCloser, error)) error {
	rc, err := open()
	if err != nil {
		return fileError(err)
	}
	defer rc.Close()
	tr := &transfer{h: h}
	err = writeFileAtomically(dest, func(w io.Writer) error {
		_, e := io.Copy(funcWriter(func(p []byte) (int, error) { tr.add(int64(len(p))); return w.Write(p) }), rc)
		return e
	})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fileError(err)
	}
	h.Detail("Wrote " + humanBytes(tr.done) + " to " + dest)
	return nil
}
