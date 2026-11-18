package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"docklet/internal/engine"
	"docklet/internal/hostopen"
	"docklet/internal/model"
)

// HostService connects what an Engine reports (bind mount sources, Compose directories) to this
// computer: it says whether a path exists here and opens it in the file manager or an editor.
// Paths are always engine paths of a named connection; they are mapped and checked in Go.
type HostService struct {
	Profiles engine.ProfileSource

	// Replaced by tests.
	goos  string
	stat  hostopen.Stat
	look  func(string) (string, error)
	start func(argv []string) error
}

func (s *HostService) deps() (goos string, stat hostopen.Stat, env hostopen.Env, start func([]string) error) {
	goos, stat, look, start := s.goos, s.stat, s.look, s.start
	if goos == "" {
		goos = runtime.GOOS
	}
	if stat == nil {
		stat = statWithTimeout
	}
	if look == nil {
		look = exec.LookPath
	}
	if start == nil {
		start = startDetached
	}
	exists := func(p string) bool { _, ok := stat(p); return ok }
	return goos, stat, hostopen.Env{GOOS: goos, Look: look, Exists: exists, Getenv: os.Getenv}, start
}

// statWithTimeout keeps a dead network share from hanging the call.
func statWithTimeout(p string) (isDir, exists bool) {
	type res struct{ dir, ok bool }
	ch := make(chan res, 1)
	go func() {
		fi, err := os.Stat(p)
		ch <- res{err == nil && fi.IsDir(), err == nil}
	}()
	select {
	case r := <-ch:
		return r.dir, r.ok
	case <-time.After(4 * time.Second):
		return false, false
	}
}

func (s *HostService) resolve(connectionID, enginePath string) (hostopen.Resolved, string, hostopen.Env, func([]string) error, error) {
	if s.Profiles == nil {
		return hostopen.Resolved{}, "", hostopen.Env{}, nil, errors.New("connection profiles are not available")
	}
	p, err := s.Profiles.Profile(connectionID)
	if err != nil {
		return hostopen.Resolved{}, "", hostopen.Env{}, nil, err
	}
	goos, stat, env, start := s.deps()
	r := hostopen.Resolve(goos, hostopen.Conn{Type: p.Type, Distro: p.WSLDistro}, enginePath, stat)
	return r, goos, env, start, nil
}

// ResolveLocalPath says where an engine path is on this computer and whether it exists.
func (s *HostService) ResolveLocalPath(_ context.Context, connectionID, enginePath string) (model.LocalPath, error) {
	r, _, _, _, err := s.resolve(connectionID, enginePath)
	if err != nil {
		return model.LocalPath{}, err
	}
	return model.LocalPath{LocalPath: r.Path, Exists: r.Exists, IsDir: r.IsDir, Reason: r.Reason}, nil
}

// ShowInFileManager opens the folder (or selects the file) in the OS file manager.
func (s *HostService) ShowInFileManager(_ context.Context, connectionID, enginePath string) error {
	r, goos, _, start, err := s.resolve(connectionID, enginePath)
	if err != nil {
		return err
	}
	if !r.Exists {
		return errors.New(r.Reason)
	}
	if err := hostopen.ValidateLocal(goos, r.Path); err != nil {
		return err
	}
	argv := hostopen.FileManagerArgv(goos, r.Path, r.IsDir)
	if err := start(argv); err != nil {
		return fmt.Errorf("could not open the file manager: %w", err)
	}
	return nil
}

// OpenInEditor opens the path in the configured or detected editor. When no editor is found it
// starts nothing and returns NeedsSetting.
func (s *HostService) OpenInEditor(_ context.Context, connectionID, enginePath string, editor model.EditorSetting) (model.EditorLaunch, error) {
	r, _, env, start, err := s.resolve(connectionID, enginePath)
	if err != nil {
		return model.EditorLaunch{}, err
	}
	if !r.Exists {
		return model.EditorLaunch{}, errors.New(r.Reason)
	}
	argv, name, ok, err := hostopen.EditorArgv(env, editor, r.Candidate)
	if err != nil {
		return model.EditorLaunch{}, err
	}
	if !ok {
		return model.EditorLaunch{NeedsSetting: true, Message: hostopen.NoEditorMessage}, nil
	}
	if err := start(argv); err != nil {
		return model.EditorLaunch{}, fmt.Errorf("could not start %s: %w", name, err)
	}
	return model.EditorLaunch{Started: true, Editor: name}, nil
}
