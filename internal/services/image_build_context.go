package services

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
)

// buildContextEntries lists the files and directories (slash-separated paths
// relative to dir) that belong in the build context, honouring .dockerignore.
// The Dockerfile and the ignore file are always kept, as the Docker CLI does,
// so the daemon can read them (the daemon still keeps them out of COPY . ).
func buildContextEntries(dir, dockerfile string) ([]string, error) {
	pats, ignoreName, err := readIgnorePatterns(dir, dockerfile)
	if err != nil {
		return nil, err
	}
	var pm *patternmatcher.PatternMatcher
	if len(pats) > 0 {
		if pm, err = patternmatcher.New(pats); err != nil {
			return nil, fmt.Errorf("invalid .dockerignore pattern: %w", err)
		}
	}
	keep := map[string]bool{dockerfile: true}
	if ignoreName != "" {
		keep[ignoreName] = true
	}
	var out []string
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return fmt.Errorf("cannot read %s: %w", p, werr)
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if pm != nil && !keep[rel] {
			skip, err := pm.MatchesOrParentMatches(rel)
			if err != nil {
				return err
			}
			if skip {
				if d.IsDir() && !pm.Exclusions() && !holdsKept(keep, rel) {
					return filepath.SkipDir
				}
				return nil
			}
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func holdsKept(keep map[string]bool, dir string) bool {
	for k := range keep {
		if strings.HasPrefix(k, dir+"/") {
			return true
		}
	}
	return false
}

// writeBuildContext writes a tar of the context to w.
func writeBuildContext(ctx context.Context, w io.Writer, dir string, entries []string) error {
	tw := tar.NewWriter(w)
	for _, rel := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		full := filepath.Join(dir, filepath.FromSlash(rel))
		fi, err := os.Lstat(full)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", rel, err)
		}
		link := ""
		if fi.Mode()&os.ModeSymlink != 0 {
			if link, err = os.Readlink(full); err != nil {
				return fmt.Errorf("cannot read link %s: %w", rel, err)
			}
			link = filepath.ToSlash(link)
		} else if !fi.Mode().IsRegular() && !fi.IsDir() {
			continue // sockets, devices, pipes
		}
		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return fmt.Errorf("cannot archive %s: %w", rel, err)
		}
		hdr.Name = rel
		if fi.IsDir() {
			hdr.Name += "/"
		}
		hdr.Uid, hdr.Gid, hdr.Uname, hdr.Gname = 0, 0, "", ""
		if runtime.GOOS == "windows" {
			// Windows has no Unix modes: like the Docker CLI, mark everything 0755.
			hdr.Mode = 0o755
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if fi.Mode().IsRegular() {
			f, err := os.Open(full)
			if err != nil {
				return fmt.Errorf("cannot read %s: %w", rel, err)
			}
			_, err = io.Copy(tw, &ctxReader{ctx: ctx, r: f})
			f.Close()
			if err != nil {
				return err
			}
		}
	}
	return tw.Close()
}

// ctxReader aborts reads once ctx is done.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// openBuildContext streams the context as a tar. Errors while archiving
// surface on the reader.
func openBuildContext(ctx context.Context, dir string, entries []string) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(writeBuildContext(ctx, pw, dir, entries))
	}()
	return pr
}

// readIgnorePatterns reads "<dockerfile>.dockerignore" if present (it takes
// precedence, like the Docker CLI), else ".dockerignore".
func readIgnorePatterns(dir, dockerfile string) ([]string, string, error) {
	for _, name := range []string{dockerfile + ".dockerignore", ".dockerignore"} {
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", fmt.Errorf("cannot read %s: %w", name, err)
		}
		pats, err := ignorefile.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, "", fmt.Errorf("invalid %s: %w", name, err)
		}
		return pats, name, nil
	}
	return nil, "", nil
}
