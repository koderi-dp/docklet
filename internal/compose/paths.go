package compose

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// ConvertWSLPath turns a path typed or picked by the user into an absolute Linux path
// inside the WSL distribution `distro`. Linux paths pass through (cleaned); the UNC
// forms \\wsl$\<distro>\... and \\wsl.localhost\<distro>\... are converted; every other
// Windows path is rejected, never silently mapped (no /mnt/c guessing).
func ConvertWSLPath(distro, p string) (string, error) {
	p = strings.TrimSpace(p)
	if err := validatePath("path", p); err != nil {
		return "", err
	}
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//") {
		return path.Clean(p), nil
	}
	norm := strings.ReplaceAll(p, "\\", "/")
	if strings.HasPrefix(norm, "//") {
		parts := strings.Split(strings.TrimLeft(norm, "/"), "/")
		host := strings.ToLower(parts[0])
		if host == "wsl$" || host == "wsl.localhost" {
			if len(parts) < 2 || parts[1] == "" {
				return "", fmt.Errorf("%q does not name a WSL distribution", p)
			}
			if !strings.EqualFold(parts[1], distro) {
				return "", fmt.Errorf("%q belongs to the WSL distribution %q, but this connection uses %q", p, parts[1], distro)
			}
			return path.Clean("/" + strings.Join(parts[2:], "/")), nil
		}
		return "", fmt.Errorf("%q is a network share, not a file inside the WSL distribution %q. Use a path under \\\\wsl$\\%s\\... or a Linux path such as /home/user/app/compose.yaml", p, distro, distro)
	}
	if winDriveRe.MatchString(p) {
		return "", fmt.Errorf("%q is a Windows path. The Compose CLI runs inside WSL (%s) and cannot read files on Windows drives. Pick the file through \\\\wsl$\\%s\\... or type its Linux path such as /home/user/app/compose.yaml. Docklet does not translate Windows paths automatically", p, distro, distro)
	}
	return "", fmt.Errorf("%q is not an absolute Linux path inside %s", p, distro)
}

// AbsLocalPath validates an absolute path on this machine (the CLI's host).
func AbsLocalPath(hostOS, p string) (string, error) {
	p = strings.TrimSpace(p)
	if err := validatePath("path", p); err != nil {
		return "", err
	}
	if hostOS == "windows" {
		if !winDriveRe.MatchString(p) && !strings.HasPrefix(p, `\\`) {
			return "", fmt.Errorf("%q is not an absolute Windows path (for example C:\\projects\\app\\compose.yaml)", p)
		}
		return p, nil
	}
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%q is not an absolute path", p)
	}
	return p, nil
}

// DirOf returns the directory of a file path for the given execution kind.
func DirOf(kind Kind, p string) string {
	if kind == KindWSL {
		return path.Dir(p)
	}
	return filepath.Dir(p)
}
