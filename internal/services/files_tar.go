package services

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"docklet/internal/model"
)

const (
	maxListEntries   = 5000
	maxListScanBytes = 256 << 20 // archive bytes streamed while listing one directory
	maxPreviewBytes  = 256 << 10
)

var errDirTooLarge = errors.New("this directory is too large to list through the archive API (its contents exceed the scan limit). Type a deeper path instead, or start the container to list it quickly")

// cleanContainerPath validates and normalises an absolute POSIX path inside a container.
// Relative paths, NUL bytes and ".." segments are rejected; "/" is allowed only when allowRoot.
func cleanContainerPath(p string, allowRoot bool) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", errors.New("path is empty")
	}
	if strings.ContainsRune(p, 0) {
		return "", errors.New("path contains a NUL byte")
	}
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("path %q must be absolute (start with /)", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", fmt.Errorf("path %q must not contain \"..\"", p)
		}
	}
	c := path.Clean(p)
	if c == "/" && !allowRoot {
		return "", errors.New("the root directory \"/\" is not allowed for this action")
	}
	return c, nil
}

// deleteArgv builds the exec argv used to delete a path: never a shell string.
func deleteArgv(p string) ([]string, error) {
	c, err := cleanContainerPath(p, false)
	if err != nil {
		return nil, err
	}
	return []string{"rm", "-rf", "--", c}, nil
}

// tarRel normalises a tar entry name to a slash path without leading "./" or "/".
// ok=false means the name is unsafe (traversal).
func tarRel(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." {
			return "", false
		}
	}
	c := path.Clean("/" + name)
	return strings.TrimPrefix(c, "/"), true
}

type scanReader struct {
	r     io.Reader
	n     int64
	limit int64
	over  bool
}

func (s *scanReader) Read(p []byte) (int, error) {
	if s.over {
		return 0, errDirTooLarge
	}
	n, err := s.r.Read(p)
	s.n += int64(n)
	if s.n > s.limit {
		s.over = true // sticky: callers like io.ReadFull may ignore an error that comes with data
	}
	return n, err
}

func fileEntryFromHeader(dirPath, rest string, h *tar.Header) model.FileEntry {
	fi := h.FileInfo()
	e := model.FileEntry{
		Name:      rest,
		Path:      path.Join(dirPath, rest),
		IsDir:     h.Typeflag == tar.TypeDir,
		IsSymlink: h.Typeflag == tar.TypeSymlink,
		Size:      h.Size,
		Mode:      fi.Mode().String(),
	}
	if e.IsSymlink {
		e.LinkTarget = h.Linkname
		e.Size = 0
	}
	if !h.ModTime.IsZero() {
		e.ModTime = h.ModTime.UnixMilli()
	}
	return e
}

// listTarChildren reads only the headers of the direct children of the archive's root
// directory (bodies are discarded by the tar reader). It stops after limit entries
// (truncated=true) and fails with errDirTooLarge when more than scanLimit bytes were streamed.
func listTarChildren(r io.Reader, dirPath string, limit int, scanLimit int64) ([]model.FileEntry, bool, error) {
	tr := tar.NewReader(&scanReader{r: r, limit: scanLimit})
	first, err := tr.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, false, errors.New("empty archive")
		}
		return nil, false, err
	}
	if first.Typeflag != tar.TypeDir {
		return nil, false, errors.New("not a directory")
	}
	root, ok := tarRel(first.Name)
	if !ok {
		return nil, false, errors.New("unsafe archive entry name")
	}
	prefix := ""
	if root != "" {
		prefix = root + "/"
	}
	out := []model.FileEntry{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		name, ok := tarRel(h.Name)
		if !ok || name == "" || !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		if rest == "" || strings.Contains(rest, "/") {
			continue
		}
		if len(out) >= limit {
			return out, true, nil
		}
		out = append(out, fileEntryFromHeader(dirPath, rest, h))
	}
}

func sortEntries(es []model.FileEntry) {
	sort.SliceStable(es, func(i, j int) bool {
		if es[i].IsDir != es[j].IsDir {
			return es[i].IsDir
		}
		return strings.ToLower(es[i].Name) < strings.ToLower(es[j].Name)
	})
}

// isBinaryContent reports whether the sample looks like binary data (NUL byte or invalid UTF-8,
// ignoring a rune cut at the end of the sample).
func isBinaryContent(b []byte) bool {
	if bytes.IndexByte(b, 0) >= 0 {
		return true
	}
	if !utf8.Valid(b) {
		for i := 1; i < utf8.UTFMax && i <= len(b); i++ {
			if utf8.Valid(b[:len(b)-i]) {
				return false
			}
		}
		return true
	}
	return false
}

// ---- building upload tars ----

// mkdirTar returns a tar holding one empty directory entry.
func mkdirTar(name string) io.Reader {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: name + "/", Mode: 0o755})
	_ = tw.Close()
	return &buf
}

// validNewName checks a single path component for Mkdir.
func validNewName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
		return fmt.Errorf("invalid folder name %q", name)
	}
	return nil
}
