// Package hostopen decides which paths reported by an Engine exist on this computer and builds
// the argv that shows them in the file manager or opens them in an editor. It is pure: the
// caller supplies file-system checks and launches the processes.
package hostopen

import (
	"errors"
	"path"
	"strings"

	"docklet/internal/compose"
	"docklet/internal/model"
)

// Conn is the part of a connection profile that decides how engine paths map to this machine.
type Conn struct {
	Type   model.ConnectionType
	Distro string
}

// Candidate is one possible local form of an engine path. WSLDistro and LinuxPath are set when
// the path lives inside a WSL distribution (editors can then open it through the WSL remote).
type Candidate struct {
	Path      string
	WSLDistro string
	LinuxPath string
}

// Resolved is the chosen Candidate plus what the file system said about it.
type Resolved struct {
	Candidate
	Exists bool
	IsDir  bool
	Reason string
}

// Stat reports whether a local path exists and whether it is a directory.
type Stat func(p string) (isDir, exists bool)

const (
	reasonRemote = "This path is on the remote engine's host."
	reasonVM     = "This path is inside the engine's virtual machine, not on this computer."
	reasonBad    = "This is not an absolute path."
)

// Candidates maps an engine path to the local paths it may correspond to, best first.
// The reason is set when it can never be local.
func Candidates(goos string, c Conn, enginePath string) ([]Candidate, string) {
	p := strings.TrimSpace(enginePath)
	if p == "" || strings.ContainsRune(p, 0) {
		return nil, reasonBad
	}
	switch c.Type {
	case model.ConnWSL:
		if goos != "windows" {
			return nil, reasonRemote
		}
		return wslCandidates(c.Distro, p)
	case model.ConnNamedPipe, model.ConnUnix:
		if goos == "windows" {
			if c.Type == model.ConnUnix {
				return nil, reasonRemote
			}
			return desktopWindowsCandidates(p)
		}
		return posixCandidates(goos, p)
	}
	return nil, reasonRemote
}

// Resolve picks the first candidate that exists. When none does, Reason says so.
func Resolve(goos string, c Conn, enginePath string, stat Stat) Resolved {
	cands, reason := Candidates(goos, c, enginePath)
	if len(cands) == 0 {
		return Resolved{Reason: reason}
	}
	for _, cand := range cands {
		if isDir, ok := stat(cand.Path); ok {
			return Resolved{Candidate: cand, Exists: true, IsDir: isDir}
		}
	}
	return Resolved{Candidate: cands[0], Reason: "This path does not exist on this computer."}
}

func posixCandidates(goos, p string) ([]Candidate, string) {
	if !strings.HasPrefix(p, "/") {
		return nil, reasonBad
	}
	p = path.Clean(p)
	if goos == "darwin" && strings.HasPrefix(p, "/host_mnt/") {
		return []Candidate{{Path: path.Clean(strings.TrimPrefix(p, "/host_mnt"))}, {Path: p}}, ""
	}
	return []Candidate{{Path: p}}, ""
}

// Docker Desktop on Windows reports bind sources as C:\..., /run/desktop/mnt/host/c/..., /host_mnt/c/...
var desktopMountPrefixes = []string{"/run/desktop/mnt/host/", "/host_mnt/", "/mnt/host/"}

func desktopWindowsCandidates(p string) ([]Candidate, string) {
	if w, ok := windowsPath(p); ok {
		return []Candidate{{Path: w}}, ""
	}
	if strings.HasPrefix(p, "/") {
		c := path.Clean(p)
		for _, pre := range desktopMountPrefixes {
			if rest, ok := strings.CutPrefix(c, pre); ok {
				if w, ok := driveFromPosix(rest); ok {
					return []Candidate{{Path: w}}, ""
				}
			}
		}
		return nil, reasonVM
	}
	return nil, reasonBad
}

const (
	uncWSLLocalhost = "\\\\wsl.localhost\\"
	uncWSLDollar    = "\\\\wsl$\\"
)

func wslCandidates(distro, p string) ([]Candidate, string) {
	if err := compose.ValidateDistro(distro); err != nil {
		return nil, err.Error()
	}
	if !strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return nil, reasonBad
	}
	c := path.Clean(p)
	if rest, ok := strings.CutPrefix(c, "/mnt/"); ok {
		if w, ok := driveFromPosix(rest); ok {
			return []Candidate{{Path: w}}, ""
		}
	}
	tail := ""
	if c != "/" {
		tail = strings.ReplaceAll(c, "/", "\\")
	}
	return []Candidate{
		{Path: uncWSLLocalhost + distro + tail, WSLDistro: distro, LinuxPath: c},
		{Path: uncWSLDollar + distro + tail, WSLDistro: distro, LinuxPath: c},
	}, ""
}

// driveFromPosix turns "c/Users/me" into C:\Users\me; the first element must be one letter.
func driveFromPosix(rest string) (string, bool) {
	drive, tail, _ := strings.Cut(rest, "/")
	if len(drive) != 1 || !isLetter(drive[0]) {
		return "", false
	}
	return strings.ToUpper(drive) + ":\\" + strings.ReplaceAll(tail, "/", "\\"), true
}

// windowsPath accepts C:\x, C:/x and \\server\share\x (not device paths such as \\.\ or \\?\).
func windowsPath(p string) (string, bool) {
	if len(p) >= 2 && isLetter(p[0]) && p[1] == ':' && (len(p) == 2 || p[2] == '\\' || p[2] == '/') {
		rest := strings.ReplaceAll(p[2:], "/", "\\")
		return strings.ToUpper(p[:1]) + ":\\" + strings.TrimLeft(rest, "\\"), true
	}
	if strings.HasPrefix(p, "\\\\") && len(p) > 2 && p[2] != '.' && p[2] != '?' {
		return p, true
	}
	return "", false
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

// ValidateLocal is the last check before a path reaches a process: absolute, no NUL.
func ValidateLocal(goos, p string) error {
	if p == "" || strings.ContainsRune(p, 0) {
		return errors.New("invalid path")
	}
	if goos == "windows" {
		if _, ok := windowsPath(p); !ok {
			return errors.New("the path is not an absolute Windows path")
		}
		return nil
	}
	if !strings.HasPrefix(p, "/") {
		return errors.New("the path is not absolute")
	}
	return nil
}
