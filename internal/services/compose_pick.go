package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"docklet/internal/compose"
	"docklet/internal/hostopen"
	"docklet/internal/model"
)

// PickPath opens the native dialog for a Compose file ("file") or folder ("folder") of a
// connection and returns the choice as the path the Compose CLI will use. For a WSL connection
// the dialog opens inside the distribution, in the user's home folder, and the picked share path
// comes back as its Linux path; a folder outside the distribution is refused right here.
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
	dir := ""
	if env.Kind == compose.KindWSL {
		dir = s.wslStartDir(ctx, env.Distro)
	}
	var p string
	if kind == "folder" {
		p, err = s.Picker.OpenDirectory(title, dir)
	} else {
		p, err = s.Picker.OpenFile(title, dir, filterName, pattern)
	}
	if err != nil || p == "" || env.Kind != compose.KindWSL {
		return p, err
	}
	return compose.ConvertWSLPath(env.Distro, p)
}

// wslStartDir is the distribution's share folder to open the dialog in: the default user's home
// folder, the distribution's root when the home folder is not known, "" when neither is reachable.
func (s *ComposeService) wslStartDir(ctx context.Context, distro string) string {
	stat := s.pickStat
	if stat == nil {
		stat = func(p string) (bool, bool) {
			fi, err := os.Stat(p)
			return err == nil && fi.IsDir(), err == nil
		}
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	home, err := s.run().Output(cctx, compose.Command{Name: "wsl.exe", Args: []string{"-d", distro, "--exec", "sh", "-c", `printf %s "$HOME"`}})
	conn := hostopen.Conn{Type: model.ConnWSL, Distro: distro}
	for _, linux := range []string{strings.TrimSpace(home), "/"} {
		if err != nil && linux != "/" {
			continue
		}
		if r := hostopen.Resolve("windows", conn, linux, stat); r.Exists && r.IsDir {
			return r.Path
		}
	}
	return ""
}
