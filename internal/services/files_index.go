package services

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"

	"docklet/internal/model"
)

// A full-tree index of a (stopped) container built from ONE archive scan of "/": only tar
// headers are kept, bodies are skipped. Listings and stats are then answered from memory.

const (
	indexBuildBudget = 60 * time.Second
	indexTTL         = 10 * time.Minute
	indexCacheSize   = 4
	indexMaxHops     = 40 // symlink hops before a loop is assumed
)

var indexMaxEntries = 500_000 // var so tests can shrink it

// errIndexUnavailable means the container is too large (or too slow) to index; the message
// is shown to the user as is.
type errIndexUnavailable struct{ reason string }

func (e errIndexUnavailable) Error() string {
	return "This container's filesystem is too large to browse while it is stopped (" + e.reason + "). Start the container for fast browsing."
}

var (
	errIdxNotExist = errors.New("no such file or directory")
	errIdxNotDir   = errors.New("not a directory")
	errIdxLoop     = errors.New("too many levels of symbolic links")
)

type idxNode struct {
	name                    string
	link                    string
	size, mtime             int64 // mtime in unix seconds, 0 = unknown
	mode                    fs.FileMode
	parent, firstKid, nextS int32 // -1 = none
}

func (n *idxNode) isDir() bool  { return n.mode.IsDir() }
func (n *idxNode) isLink() bool { return n.mode&fs.ModeSymlink != 0 }

type fileIndex struct {
	nodes      []idxNode // nodes[0] is the root directory
	builtAt    time.Time
	buildTook  time.Duration
	stamp      string
	entryCount int
}

func newFileIndex() *fileIndex {
	return &fileIndex{nodes: []idxNode{{name: "", mode: fs.ModeDir | 0o755, parent: -1, firstKid: -1, nextS: -1}}}
}

func (x *fileIndex) add(parent int32, n idxNode) (int32, error) {
	if len(x.nodes) >= indexMaxEntries {
		return 0, errIndexUnavailable{fmt.Sprintf("it has more than %d files", indexMaxEntries)}
	}
	n.parent, n.firstKid = parent, -1
	n.nextS = x.nodes[parent].firstKid
	idx := int32(len(x.nodes))
	x.nodes = append(x.nodes, n)
	x.nodes[parent].firstKid = idx
	return idx, nil
}

// buildFileIndex reads only the headers of a tar archive of the container root. Directories
// missing from the archive are created implicitly. maxEntries caps the node count.
func buildFileIndex(ctx context.Context, r io.Reader) (*fileIndex, error) {
	tr := tar.NewReader(r)
	x := newFileIndex()
	dirs := map[string]int32{"": 0}
	var ensure func(rel string) (int32, error)
	ensure = func(rel string) (int32, error) {
		if i, ok := dirs[rel]; ok {
			return i, nil
		}
		parent, err := ensure(dirName(rel))
		if err != nil {
			return 0, err
		}
		i, err := x.add(parent, idxNode{name: path.Base(rel), mode: fs.ModeDir | 0o755})
		if err != nil {
			return 0, err
		}
		dirs[rel] = i
		return i, nil
	}
	prefix, first, n := "", true, 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		if n++; n%512 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		rel, ok := tarRel(h.Name)
		if !ok {
			continue
		}
		if first {
			first = false
			if h.Typeflag == tar.TypeDir {
				prefix = rel // the archive's root directory name ("" for "/")
			}
		}
		if prefix != "" {
			if rel == prefix {
				rel = ""
			} else if strings.HasPrefix(rel, prefix+"/") {
				rel = rel[len(prefix)+1:]
			} else {
				continue
			}
		}
		mode := h.FileInfo().Mode()
		mt := int64(0)
		if !h.ModTime.IsZero() {
			mt = h.ModTime.Unix()
		}
		if rel == "" {
			x.nodes[0].mode, x.nodes[0].mtime = mode|fs.ModeDir, mt
			continue
		}
		if h.Typeflag == tar.TypeDir {
			i, err := ensure(rel)
			if err != nil {
				return nil, err
			}
			x.nodes[i].mode, x.nodes[i].mtime = mode|fs.ModeDir, mt
			continue
		}
		parent, err := ensure(dirName(rel))
		if err != nil {
			return nil, err
		}
		nd := idxNode{name: path.Base(rel), mode: mode, size: h.Size, mtime: mt}
		if h.Typeflag == tar.TypeSymlink {
			nd.link, nd.size = h.Linkname, 0
		}
		if _, err := x.add(parent, nd); err != nil {
			return nil, err
		}
	}
	if len(x.nodes) == 1 && first {
		return nil, errors.New("the container archive is empty")
	}
	x.entryCount = len(x.nodes) - 1
	return x, nil
}

func dirName(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}

func (x *fileIndex) child(dir int32, name string) int32 {
	for c := x.nodes[dir].firstKid; c >= 0; c = x.nodes[c].nextS {
		if x.nodes[c].name == name {
			return c
		}
	}
	return -1
}

// resolve walks an absolute path, following symlinks in every non-final segment and in the
// final one when follow is set. Loops end with errIdxLoop.
func (x *fileIndex) resolve(p string, follow bool) (int32, error) {
	queue := splitSegs(p)
	stack := []int32{0}
	hops := 0
	for len(queue) > 0 {
		seg := queue[0]
		queue = queue[1:]
		switch seg {
		case "", ".":
			continue
		case "..":
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		cur := stack[len(stack)-1]
		if !x.nodes[cur].isDir() {
			return 0, errIdxNotDir
		}
		c := x.child(cur, seg)
		if c < 0 {
			return 0, errIdxNotExist
		}
		if n := &x.nodes[c]; n.isLink() && (follow || len(queue) > 0) {
			if hops++; hops > indexMaxHops {
				return 0, errIdxLoop
			}
			rest := splitSegs(n.link)
			if strings.HasPrefix(n.link, "/") {
				stack = stack[:1]
			}
			queue = append(rest, queue...)
			continue
		}
		stack = append(stack, c)
	}
	return stack[len(stack)-1], nil
}

func splitSegs(p string) []string { return strings.Split(p, "/") }

func (x *fileIndex) entry(n *idxNode, p string) model.FileEntry {
	e := model.FileEntry{
		Name: n.name, Path: p, IsDir: n.isDir(), IsSymlink: n.isLink(),
		Size: n.size, Mode: n.mode.String(),
	}
	if e.IsSymlink {
		e.LinkTarget, e.Size = n.link, 0
		if t, err := x.resolve(p, true); err == nil && x.nodes[t].isDir() {
			e.IsDir = true
		}
	}
	if n.mtime != 0 {
		e.ModTime = n.mtime * 1000
	}
	return e
}

func (x *fileIndex) pathErr(p string, err error) error {
	switch {
	case errors.Is(err, errIdxNotDir):
		return fmt.Errorf("%s is not a directory", p)
	case errors.Is(err, errIdxLoop):
		return fmt.Errorf("%s is a symbolic link loop", p)
	}
	return fmt.Errorf("That path does not exist in the container: %s", p)
}

func (x *fileIndex) list(dir string) (model.FileListing, error) {
	i, err := x.resolve(dir, true)
	if err != nil {
		if j, e2 := x.resolve(dir, false); e2 == nil && x.nodes[j].isLink() {
			return model.FileListing{}, fmt.Errorf("%s is a broken or looping symbolic link", dir)
		}
		return model.FileListing{}, x.pathErr(dir, err)
	}
	if !x.nodes[i].isDir() {
		return model.FileListing{}, fmt.Errorf("%s is not a directory", dir)
	}
	var kids []int32
	for c := x.nodes[i].firstKid; c >= 0; c = x.nodes[c].nextS {
		kids = append(kids, c)
	}
	out := make([]model.FileEntry, 0, len(kids))
	for _, c := range kids {
		out = append(out, x.entry(&x.nodes[c], path.Join(dir, x.nodes[c].name)))
	}
	sortEntries(out)
	truncated := false
	if len(out) > maxListEntries {
		out, truncated = out[:maxListEntries], true
	}
	return model.FileListing{
		Path: dir, Entries: out, Truncated: truncated, Limit: maxListEntries, Method: "index",
		IndexedAt: x.builtAt.UnixMilli(), IndexedEntries: x.entryCount,
	}, nil
}

// stat describes p without following a final symlink.
func (x *fileIndex) stat(p string) (model.FileEntry, error) {
	i, err := x.resolve(p, false)
	if err != nil {
		return model.FileEntry{}, x.pathErr(p, err)
	}
	e := x.entry(&x.nodes[i], p)
	if p == "/" {
		e.Name = "/"
	}
	return e, nil
}
