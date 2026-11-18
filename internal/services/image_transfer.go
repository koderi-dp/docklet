package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/moby/api/types/jsonstream"

	"docklet/internal/model"
	"docklet/internal/ops"
)

// validateSavePath checks a destination chosen with the native save dialog.
func validateSavePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("choose where to save the archive")
	}
	if !filepath.IsAbs(p) {
		return "", errors.New("the destination must be an absolute path")
	}
	p = filepath.Clean(p)
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return "", fmt.Errorf("%q is a directory", p)
	}
	if st, err := os.Stat(filepath.Dir(p)); err != nil || !st.IsDir() {
		return "", fmt.Errorf("the folder %q does not exist", filepath.Dir(p))
	}
	return p, nil
}

// validateLoadPath checks a file chosen with the native open dialog.
func validateLoadPath(p string) (string, int64, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", 0, errors.New("choose an image archive to load")
	}
	if !filepath.IsAbs(p) {
		return "", 0, errors.New("the archive must be an absolute path")
	}
	p = filepath.Clean(p)
	st, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, fmt.Errorf("file %q does not exist", p)
		}
		return "", 0, fmt.Errorf("cannot read %q: %w", p, err)
	}
	if !st.Mode().IsRegular() {
		return "", 0, fmt.Errorf("%q is not a file", p)
	}
	if st.Size() == 0 {
		return "", 0, fmt.Errorf("%q is empty", p)
	}
	return p, st.Size(), nil
}

// Save writes one or more images to a tar archive on this computer as a
// background operation. The archive is written to a temporary file next to the
// destination and moved into place on success; a partial file never remains.
func (s *ImageService) Save(ctx context.Context, connectionID string, refs []string, destPath string) (model.Operation, error) {
	if len(refs) == 0 {
		return model.Operation{}, errors.New("select at least one image to save")
	}
	var clean []string
	seen := map[string]bool{}
	for _, r := range refs {
		v, err := validateImageRef(r)
		if err != nil {
			return model.Operation{}, err
		}
		if !seen[v] {
			seen[v] = true
			clean = append(clean, v)
		}
	}
	dest, err := validateSavePath(destPath)
	if err != nil {
		return model.Operation{}, err
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	if s.Ops == nil {
		return model.Operation{}, fmt.Errorf("operations are not available")
	}
	title := "Save " + displayRef(clean[0])
	if len(clean) > 1 {
		title = fmt.Sprintf("Save %d images", len(clean))
	}
	op := s.Ops.Start(ops.Spec{
		Kind: "image.save", Title: title, ConnectionID: connectionID, Resource: filepath.Base(dest), Cancellable: true,
	}, func(octx context.Context, h *ops.Handle) error {
		h.Log(fmt.Sprintf("Saving %s to %s", strings.Join(clean, ", "), dest))
		rc, err := cli.ImageSave(octx, clean)
		if err != nil {
			if octx.Err() != nil {
				return octx.Err()
			}
			return imageError(err)
		}
		defer rc.Close()
		n, err := writeArchive(octx, rc, dest, h)
		if err != nil {
			return err
		}
		h.Log(fmt.Sprintf("Saved %d image(s), %s", len(clean), humanBytes(n)))
		return nil
	})
	return op, nil
}

type countingWriter struct {
	w      io.Writer
	n      int64
	h      pullSink
	last   time.Time
	prefix string
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	if time.Since(c.last) > 200*time.Millisecond {
		c.last = time.Now()
		c.h.Detail(c.prefix + humanBytes(c.n))
	}
	return n, err
}

// writeArchive streams r to dest through a temporary file. On any failure or
// cancellation the temporary file is removed and dest is left untouched.
func writeArchive(ctx context.Context, r io.Reader, dest string, h pullSink) (int64, error) {
	tmp := dest + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("cannot write %q: %w", dest, err)
	}
	cw := &countingWriter{w: f, h: h, prefix: "Written "}
	_, copyErr := io.Copy(cw, &ctxReader{ctx: ctx, r: r})
	closeErr := f.Close()
	if copyErr == nil && ctx.Err() != nil {
		copyErr = ctx.Err()
	}
	if copyErr == nil && cw.n == 0 {
		copyErr = errors.New("the Engine returned an empty archive")
	}
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr == nil {
		if rerr := os.Rename(tmp, dest); rerr != nil {
			// Windows cannot rename over an existing file on some filesystems.
			if os.Remove(dest) == nil {
				rerr = os.Rename(tmp, dest)
			}
			copyErr = rerr
		}
	}
	if copyErr != nil {
		os.Remove(tmp)
		if ctx.Err() != nil {
			return cw.n, ctx.Err()
		}
		return cw.n, imageError(copyErr)
	}
	return cw.n, nil
}

// Load streams a tar archive from this computer to the Engine as a background
// operation and reports the loaded image names.
func (s *ImageService) Load(ctx context.Context, connectionID, srcPath string) (model.Operation, error) {
	src, size, err := validateLoadPath(srcPath)
	if err != nil {
		return model.Operation{}, err
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	if s.Ops == nil {
		return model.Operation{}, fmt.Errorf("operations are not available")
	}
	op := s.Ops.Start(ops.Spec{
		Kind: "image.load", Title: "Load " + filepath.Base(src), ConnectionID: connectionID, Resource: filepath.Base(src), Cancellable: true,
	}, func(octx context.Context, h *ops.Handle) error {
		f, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("cannot read %q: %w", src, err)
		}
		defer f.Close()
		h.Log(fmt.Sprintf("Uploading %s (%s) to the Engine", src, humanBytes(size)))
		pr := &progressReader{ctx: octx, r: f, total: size, h: h}
		res, err := cli.ImageLoad(octx, pr)
		if err != nil {
			if octx.Err() != nil {
				return octx.Err()
			}
			return imageError(err)
		}
		defer res.Close()
		loaded, err := consumeLoad(octx, res, h)
		s.emitChanged(connectionID)
		if err != nil {
			return err
		}
		if len(loaded) == 0 {
			h.Log("The Engine reported no loaded images")
		} else {
			h.Log("Loaded: " + strings.Join(loaded, ", "))
		}
		return nil
	})
	return op, nil
}

// progressReader reports real upload progress (bytes read of the file size).
type progressReader struct {
	ctx   context.Context
	r     io.Reader
	total int64
	read  int64
	h     pullSink
	last  time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	if err := p.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.total > 0 && time.Since(p.last) > 200*time.Millisecond {
		p.last = time.Now()
		p.h.Progress(float64(p.read) / float64(p.total))
		p.h.Detail(fmt.Sprintf("Uploaded %s / %s", humanBytes(p.read), humanBytes(p.total)))
	}
	return n, err
}

// consumeLoad parses the Engine's load response and returns the loaded names.
func consumeLoad(ctx context.Context, r io.Reader, h pullSink) ([]string, error) {
	dec := json.NewDecoder(r)
	var loaded []string
	for {
		var m jsonstream.Message
		err := dec.Decode(&m)
		if err == io.EOF {
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				return loaded, ctx.Err()
			}
			return loaded, imageError(fmt.Errorf("load output interrupted: %w", err))
		}
		if m.Error != nil {
			return loaded, imageError(errors.New(m.Error.Message))
		}
		text := strings.TrimSpace(m.Stream)
		if text == "" {
			text = strings.TrimSpace(m.Status + " " + m.ID)
		}
		if text == "" {
			continue
		}
		h.Log(text)
		for _, pfx := range []string{"Loaded image: ", "Loaded image ID: "} {
			if strings.HasPrefix(text, pfx) {
				loaded = append(loaded, strings.TrimPrefix(text, pfx))
			}
		}
	}
	if ctx.Err() != nil {
		return loaded, ctx.Err()
	}
	return loaded, nil
}
