package services

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// sanitizeTar copies a tar (optionally gzip-compressed) to dst, rejecting entries that
// would escape the destination (absolute names, "..", links pointing outside, devices).
func sanitizeTar(src io.Reader, dst io.Writer, onBytes func(n int64)) error {
	br := bufio.NewReaderSize(src, 4096)
	var in io.Reader = br
	if magic, _ := br.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return fmt.Errorf("invalid gzip stream: %w", err)
		}
		defer gz.Close()
		in = gz
	}
	tr := tar.NewReader(in)
	tw := tar.NewWriter(dst)
	n := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("not a valid tar archive: %w", err)
		}
		n++
		if strings.HasPrefix(h.Name, "/") || strings.HasPrefix(h.Name, "\\") {
			return fmt.Errorf("unsafe entry %q: absolute path in archive", h.Name)
		}
		if _, ok := tarRel(h.Name); !ok {
			return fmt.Errorf("unsafe entry %q: path traversal in archive", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeSymlink:
			if h.Linkname == "" {
				return fmt.Errorf("unsafe entry %q: empty link target", h.Name)
			}
		case tar.TypeLink:
			if strings.HasPrefix(h.Linkname, "/") {
				return fmt.Errorf("unsafe entry %q: absolute hard link target", h.Name)
			}
			if _, ok := tarRel(h.Linkname); !ok {
				return fmt.Errorf("unsafe entry %q: hard link escapes the archive", h.Name)
			}
		case tar.TypeChar, tar.TypeBlock:
			return fmt.Errorf("unsafe entry %q: device nodes are not imported", h.Name)
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		c, err := io.Copy(tw, tr)
		if err != nil {
			return err
		}
		if onBytes != nil {
			onBytes(c)
		}
	}
	if n == 0 {
		return errors.New("the archive is empty")
	}
	return tw.Close()
}

type progressFn func(n int64)

// localSize sums the regular file sizes below p (real progress denominator).
func localSize(p string) int64 {
	var total int64
	_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if fi, e := d.Info(); e == nil {
				total += fi.Size()
			}
		}
		return nil
	})
	return total
}

func tarMode(fi fs.FileInfo) int64 {
	if runtime.GOOS == "windows" {
		if fi.IsDir() {
			return 0o755
		}
		return 0o644
	}
	return int64(fi.Mode().Perm())
}

// writeLocalTar tars the local file or directory src (its base name becomes the top entry).
// Symlinks are stored as links (never followed); other irregular files are skipped.
func writeLocalTar(w io.Writer, src string, progress progressFn) error {
	src = filepath.Clean(src)
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(w)
	base := filepath.Base(src)
	if info.IsDir() {
		err = filepath.WalkDir(src, func(p string, d fs.DirEntry, werr error) error {
			if werr != nil {
				return werr
			}
			rel, err := filepath.Rel(src, p)
			if err != nil {
				return err
			}
			name := base
			if rel != "." {
				name = base + "/" + filepath.ToSlash(rel)
			}
			return addLocalEntry(tw, p, name, d, progress)
		})
	} else {
		err = addLocalEntry(tw, src, base, fs.FileInfoToDirEntry(info), progress)
	}
	if err != nil {
		return err
	}
	return tw.Close()
}

func addLocalEntry(tw *tar.Writer, p, name string, d fs.DirEntry, progress progressFn) error {
	fi, err := d.Info()
	if err != nil {
		return err
	}
	switch {
	case fi.IsDir():
		return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: name + "/", Mode: tarMode(fi), ModTime: fi.ModTime()})
	case fi.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(p)
		if err != nil {
			return err
		}
		return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeSymlink, Name: name, Linkname: filepath.ToSlash(target), Mode: 0o777, ModTime: fi.ModTime()})
	case fi.Mode().IsRegular():
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Size: fi.Size(), Mode: tarMode(fi), ModTime: fi.ModTime()}); err != nil {
			return err
		}
		var src io.Reader = f
		if progress != nil {
			src = &countReader{r: f, fn: progress}
		}
		_, err = io.CopyN(tw, src, fi.Size())
		return err
	}
	return nil // sockets, devices, ...
}

type countReader struct {
	r  io.Reader
	fn progressFn
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.fn(int64(n))
	}
	return n, err
}
