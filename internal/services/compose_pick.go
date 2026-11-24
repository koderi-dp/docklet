package services

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"docklet/internal/compose"
)

// PickPath opens the native dialog for a Compose file ("file") or folder ("folder") of a
// connection and returns the choice as the path the Compose CLI will use: for a WSL connection
// that is the Linux path inside the distribution, also for a file on a Windows drive.
// An empty result means the user cancelled.
func (s *ComposeService) PickPath(ctx context.Context, connectionID, kind, title, filterName, pattern string) (string, error) {
	if kind != "file" && kind != "folder" {
		return "", fmt.Errorf("unknown kind %q", kind)
	}
	if s.Picker == nil {
		return "", errors.New("file dialogs are not available")
	}
	env, err := s.env(connectionID)
	if err != nil {
		return "", err
	}
	var p string
	if kind == "folder" {
		p, err = s.Picker.OpenDirectory(title)
	} else {
		p, err = s.Picker.OpenFile(title, filterName, pattern)
	}
	if err != nil || p == "" || env.Kind != compose.KindWSL {
		return p, err
	}
	return s.wslPath(ctx, env.Distro, p)
}

// wslPath turns a typed or picked path into the Linux path inside the distribution. A Windows
// drive path is translated by the distribution itself (wslpath), so a drive it has not mounted
// is an error instead of a guess.
func (s *ComposeService) wslPath(ctx context.Context, distro, p string) (string, error) {
	cmd, isDrive, err := compose.WSLPathCommand(distro, p)
	if !isDrive {
		return compose.ConvertWSLPath(distro, p)
	}
	if err != nil {
		return "", err
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := s.run().Output(cctx, cmd)
	linux := strings.TrimSpace(out)
	if err != nil || !strings.HasPrefix(linux, "/") {
		return "", fmt.Errorf("%q is a Windows path that the WSL distribution %q cannot reach. The Compose CLI runs inside that distribution, so the drive must be mounted there (normally under /mnt)", strings.TrimSpace(p), distro)
	}
	return compose.ConvertWSLPath(distro, linux)
}

// normalizeFiles turns the paths a user gave into the form the Compose CLI reads, without
// empty entries and duplicates.
func (s *ComposeService) normalizeFiles(ctx context.Context, env compose.ExecEnv, in []string, kind string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, f := range in {
		if strings.TrimSpace(f) == "" {
			continue
		}
		var p string
		var err error
		if env.Kind == compose.KindWSL {
			p, err = s.wslPath(ctx, env.Distro, f)
		} else {
			p, err = compose.AbsLocalPath(runtime.GOOS, f)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", kind, err)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out, nil
}
