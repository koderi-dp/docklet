//go:build linux

package ports

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

type linuxHost struct{ run runFunc }

// NewLocalHost reads this machine's listeners with `ss`, or /proc when ss is missing.
func NewLocalHost() HostSource { return linuxHost{run: execRun} }

func (linuxHost) Where() string { return "This machine" }

func (h linuxHost) Listeners(ctx context.Context) ([]Listener, error) {
	if b, err := h.run(ctx, "ss", "-H", "-ltnup"); err == nil {
		return ParseSS(string(b), "This machine"), nil
	}
	return readProc()
}

// readProc parses /proc/net and names the owners of sockets whose process this user may inspect.
func readProc() ([]Listener, error) {
	owners := socketOwners()
	var out []Listener
	var firstErr error
	for _, t := range []struct{ file, proto string }{{"tcp", "tcp"}, {"tcp6", "tcp"}, {"udp", "udp"}, {"udp6", "udp"}} {
		b, err := os.ReadFile("/proc/net/" + t.file)
		if err != nil {
			if firstErr == nil && !errors.Is(err, os.ErrNotExist) {
				firstErr = err
			}
			continue
		}
		for _, s := range ParseProcNet(string(b), t.proto, "This machine") {
			l := s.Listener
			if o, ok := owners[s.Inode]; ok {
				l.PID, l.Name, l.Path, l.Hidden = o.pid, o.name, o.path, false
			}
			l.User = userName(s.UID)
			out = append(out, l)
		}
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

type sockOwner struct {
	pid        int
	name, path string
}

// socketOwners maps socket inodes to processes through /proc/<pid>/fd.
func socketOwners() map[uint64]sockOwner {
	out := map[uint64]sockOwner{}
	dirs, _ := filepath.Glob("/proc/[0-9]*/fd/*")
	for _, fd := range dirs {
		target, err := os.Readlink(fd)
		if err != nil || !strings.HasPrefix(target, "socket:[") {
			continue
		}
		inode, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]"), 10, 64)
		if err != nil {
			continue
		}
		proc := filepath.Dir(filepath.Dir(fd))
		pid, _ := strconv.Atoi(filepath.Base(proc))
		comm, _ := os.ReadFile(proc + "/comm")
		exe, _ := os.Readlink(proc + "/exe")
		out[inode] = sockOwner{pid: pid, name: strings.TrimSpace(string(comm)), path: exe}
	}
	return out
}

func userName(uid int) string {
	if u, err := user.LookupId(strconv.Itoa(uid)); err == nil {
		return u.Username
	}
	return strconv.Itoa(uid)
}
