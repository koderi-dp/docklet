package services

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"docklet/internal/model"
)

// List returns the direct children of a directory. Running containers are listed through a
// fast exec; stopped (or shell-less / paused) containers through a cached full-tree index built
// from one archive scan.
func (s *FileService) List(ctx context.Context, connectionID, containerID, p string) (model.FileListing, error) {
	dir, err := cleanContainerPath(p, true)
	if err != nil {
		return model.FileListing{}, err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.FileListing{}, err
	}
	l, err := s.list(ctx, api, connectionID, containerID, dir)
	if err != nil {
		return model.FileListing{}, err
	}
	if l.Entries == nil {
		l.Entries = []model.FileEntry{}
	}
	return l, nil
}

func (s *FileService) list(ctx context.Context, api filesAPI, connectionID, containerID, dir string) (model.FileListing, error) {
	info, err := api.Inspect(ctx, containerID)
	if err != nil {
		return model.FileListing{}, friendlyError(err)
	}
	if info.canExec() {
		lctx, cancel := context.WithTimeout(ctx, listTimeout)
		defer cancel()
		l, ok, err := listViaExec(lctx, api, containerID, dir)
		if err != nil {
			return model.FileListing{}, err
		}
		if ok {
			return l, nil
		}
		l, err = listViaArchive(lctx, api, containerID, dir)
		if err == nil || !errors.Is(err, errDirTooLarge) {
			return l, err
		}
		// a big tree near the root: the index is the only practical way
	}
	x, err := s.getIndex(ctx, api, connectionID, containerID, info, false)
	if err != nil {
		var unavailable errIndexUnavailable
		if errors.As(err, &unavailable) {
			// too big to index: a small directory may still be listable on its own
			actx, cancel := context.WithTimeout(ctx, listTimeout)
			defer cancel()
			if l, aerr := listViaArchive(actx, api, containerID, dir); aerr == nil {
				return l, nil
			}
		}
		return model.FileListing{}, err
	}
	return x.list(dir)
}

// listDir lists one directory (volume helper containers use tryExec=false).
func listDir(ctx context.Context, api filesAPI, id, dir string, tryExec bool) (model.FileListing, error) {
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	if tryExec {
		if info, err := api.Inspect(ctx, id); err == nil && info.canExec() {
			l, ok, err := listViaExec(ctx, api, id, dir)
			if err != nil {
				return model.FileListing{}, err
			}
			if ok {
				return l, nil
			}
		}
	}
	return listViaArchive(ctx, api, id, dir)
}

func listViaArchive(ctx context.Context, api filesAPI, id, dir string) (model.FileListing, error) {
	src := dir
	st, err := api.StatPath(ctx, id, dir)
	if err != nil {
		return model.FileListing{}, fileError(err)
	}
	switch {
	case st.Mode&fs.ModeSymlink != 0:
		src = strings.TrimSuffix(dir, "/") + "/." // resolve the link
	case !st.Mode.IsDir():
		return model.FileListing{}, fmt.Errorf("%s is not a directory", dir)
	}
	body, _, err := api.CopyFrom(ctx, id, src)
	if err != nil {
		return model.FileListing{}, fileError(err)
	}
	defer body.Close()
	entries, truncated, err := listTarChildren(body, dir, maxListEntries, maxListScanBytes)
	if err != nil {
		if errors.Is(err, errDirTooLarge) {
			return model.FileListing{}, err
		}
		if ctx.Err() != nil {
			return model.FileListing{}, friendlyError(ctx.Err())
		}
		return model.FileListing{}, fileError(err)
	}
	sortEntries(entries)
	return model.FileListing{Path: dir, Entries: entries, Truncated: truncated, Limit: maxListEntries, Method: "archive"}, nil
}

// Record layout of the exec protocol: fields are separated by RS (0x1e), records end with
// US+newline (0x1f 0x0a), so names with newlines, spaces, "|" or unicode survive.
const (
	fieldSep  = "\x1e"
	recordEnd = "\x1f\n"
)

// listScript lists the directory given as $1 with one stat call (no ls parsing). Names are
// passed as arguments, never interpolated into the script. Exit codes: 4 = missing,
// 5 = not a directory, 7 = broken or looping link, 3 = cannot enter, 6 = entries exist but could not be stat'ed,
// 127 = no stat. Works with GNU coreutils and busybox.
const listScript = `[ -e "$1" ] || { [ -L "$1" ] && exit 7; exit 4; }
[ -d "$1" ] || exit 5
cd -- "$1" 2>/dev/null || exit 3
F="$(printf '%%f\036%%s\036%%Y\036%%n\037')"
out=$(stat -c "$F" -- .[!.]* ..?* * 2>/dev/null)
[ "$?" -eq 127 ] && exit 127
if [ -z "$out" ]; then
  [ -n "$(ls -A 2>/dev/null)" ] && exit 6
  exit 0
fi
printf '%s\n' "$out"
exit 0`

// linkScript prints "kind RS target US NL" (kind d = resolves to a directory) for every
// link name passed after the directory.
const linkScript = `cd -- "$1" 2>/dev/null || exit 3
shift
for f; do
  t=$(readlink -- "$f")
  if [ -d "$f" ]; then k=d; else k=f; fi
  printf '%s\036%s\037\n' "$k" "$t"
done`

func modeFromRaw(raw uint32) fs.FileMode {
	m := fs.FileMode(raw & 0o777)
	switch raw & 0o170000 {
	case 0o040000:
		m |= fs.ModeDir
	case 0o120000:
		m |= fs.ModeSymlink
	case 0o010000:
		m |= fs.ModeNamedPipe
	case 0o140000:
		m |= fs.ModeSocket
	case 0o020000:
		m |= fs.ModeDevice | fs.ModeCharDevice
	case 0o060000:
		m |= fs.ModeDevice
	}
	if raw&0o4000 != 0 {
		m |= fs.ModeSetuid
	}
	if raw&0o2000 != 0 {
		m |= fs.ModeSetgid
	}
	if raw&0o1000 != 0 {
		m |= fs.ModeSticky
	}
	return m
}

// parseStatLines parses listScript records "%f RS %s RS %Y RS %n US NL"; unparsable records are skipped.
func parseStatLines(out []byte, dir string) []model.FileEntry {
	res := []model.FileEntry{}
	for _, rec := range strings.Split(string(out), recordEnd) {
		f := strings.SplitN(rec, fieldSep, 4)
		if len(f) != 4 || f[3] == "" || strings.Contains(f[3], "/") {
			continue
		}
		raw, err := strconv.ParseUint(f[0], 16, 32)
		if err != nil {
			continue
		}
		size, _ := strconv.ParseInt(f[1], 10, 64)
		mt, _ := strconv.ParseInt(f[2], 10, 64)
		m := modeFromRaw(uint32(raw))
		e := model.FileEntry{
			Name: f[3], Path: path.Join(dir, f[3]), IsDir: m.IsDir(), IsSymlink: m&fs.ModeSymlink != 0,
			Size: size, Mode: m.String(), ModTime: mt * 1000,
		}
		if e.IsSymlink || e.IsDir {
			e.Size = 0
		}
		res = append(res, e)
	}
	return res
}

// parseLinkRecords parses linkScript output.
func parseLinkRecords(out []byte) (kinds, targets []string) {
	for _, rec := range strings.Split(string(out), recordEnd) {
		f := strings.SplitN(rec, fieldSep, 2)
		if len(f) != 2 {
			continue
		}
		kinds, targets = append(kinds, f[0]), append(targets, f[1])
	}
	return kinds, targets
}

// listViaExec lists dir inside a running container. ok=false means "use another method";
// a non-nil error is a definitive answer (missing path, not a directory).
func listViaExec(ctx context.Context, api filesAPI, id, dir string) (model.FileListing, bool, error) {
	out, _, code, err := api.Exec(ctx, id, []string{"sh", "-c", listScript, "sh", dir}, execListMax)
	if err != nil {
		if ctx.Err() != nil {
			return model.FileListing{}, false, friendlyError(ctx.Err())
		}
		return model.FileListing{}, false, nil
	}
	switch code {
	case 0:
	case 4:
		return model.FileListing{}, false, fmt.Errorf("That path does not exist in the container: %s", dir)
	case 5:
		return model.FileListing{}, false, fmt.Errorf("%s is not a directory", dir)
	case 7:
		return model.FileListing{}, false, fmt.Errorf("%s is a broken or looping symbolic link", dir)
	default:
		return model.FileListing{}, false, nil
	}
	entries := parseStatLines(out, dir)
	truncated := false
	if len(entries) > maxListEntries {
		entries, truncated = entries[:maxListEntries], true
	}
	resolveLinks(ctx, api, id, dir, entries)
	sortEntries(entries)
	return model.FileListing{Path: dir, Entries: entries, Truncated: truncated, Limit: maxListEntries, Method: "exec"}, true, nil
}

// resolveLinks fills LinkTarget and IsDir (link resolves to a directory) of symlink entries.
func resolveLinks(ctx context.Context, api filesAPI, id, dir string, entries []model.FileEntry) {
	const chunk = 300
	var idx []int
	for i, e := range entries {
		if e.IsSymlink {
			idx = append(idx, i)
		}
	}
	for len(idx) > 0 {
		n := min(chunk, len(idx))
		part := idx[:n]
		idx = idx[n:]
		argv := []string{"sh", "-c", linkScript, "sh", dir}
		for _, i := range part {
			argv = append(argv, entries[i].Name)
		}
		out, _, code, err := api.Exec(ctx, id, argv, execListMax)
		if err != nil || code != 0 {
			continue
		}
		kinds, targets := parseLinkRecords(out)
		if len(kinds) != len(part) {
			continue
		}
		for k, i := range part {
			entries[i].LinkTarget = targets[k]
			entries[i].IsDir = kinds[k] == "d"
		}
	}
}
