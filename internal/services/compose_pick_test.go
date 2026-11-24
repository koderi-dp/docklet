package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"docklet/internal/compose"
)

type fakePicker struct {
	dirs   []string // start folders the dialog was opened in
	kind   string   // "file" or "folder", from the last call
	answer string
}

func (f *fakePicker) OpenFile(_, dir, _, _ string) (string, error) {
	f.dirs, f.kind = append(f.dirs, dir), "file"
	return f.answer, nil
}

func (f *fakePicker) OpenDirectory(_, dir string) (string, error) {
	f.dirs, f.kind = append(f.dirs, dir), "folder"
	return f.answer, nil
}

func (f *fakePicker) SaveFile(_, _ string) (string, error) { return "", nil }

// pickSvc is a service whose WSL distribution "Ubuntu" has the home folder `home` ("" = lookup fails)
// and whose machine has the share folders listed in `shares`.
func pickSvc(p *fakePicker, home string, shares ...string) *ComposeService {
	r := &fakeRunner{out: func(compose.Command) (string, error) {
		if home == "" {
			return "", errors.New("wsl is not running")
		}
		return home + "\n", nil
	}}
	s := newSvc(r, nil)
	s.Picker = p
	s.pickStat = func(path string) (bool, bool) {
		for _, sh := range shares {
			if sh == path {
				return true, true
			}
		}
		return false, false
	}
	return s
}

func TestPickPath_WSLOpensInTheDistroHomeAndReturnsTheLinuxPath(t *testing.T) {
	p := &fakePicker{answer: `\\wsl.localhost\Ubuntu\home\u\app`}
	s := pickSvc(p, "/home/u", `\\wsl.localhost\Ubuntu\home\u`)
	got, err := s.PickPath(context.Background(), "wsl", "folder", "Choose", "", "")
	if err != nil || got != "/home/u/app" {
		t.Fatalf("got %q, %v", got, err)
	}
	if len(p.dirs) != 1 || p.dirs[0] != `\\wsl.localhost\Ubuntu\home\u` || p.kind != "folder" {
		t.Errorf("dialog opened in %v as %s", p.dirs, p.kind)
	}
}

func TestPickPath_WSLFallsBackToTheOlderShareName(t *testing.T) {
	p := &fakePicker{}
	s := pickSvc(p, "/home/u", `\\wsl$\Ubuntu\home\u`)
	if _, err := s.PickPath(context.Background(), "wsl", "file", "Choose", "Compose", "*.yaml"); err != nil {
		t.Fatal(err)
	}
	if p.dirs[0] != `\\wsl$\Ubuntu\home\u` || p.kind != "file" {
		t.Errorf("dialog opened in %v as %s", p.dirs, p.kind)
	}
}

func TestPickPath_WSLStartsAtTheDistroRootWhenTheHomeFolderIsUnknown(t *testing.T) {
	p := &fakePicker{}
	s := pickSvc(p, "", `\\wsl.localhost\Ubuntu`)
	if _, err := s.PickPath(context.Background(), "wsl", "folder", "Choose", "", ""); err != nil {
		t.Fatal(err)
	}
	if p.dirs[0] != `\\wsl.localhost\Ubuntu` {
		t.Errorf("dialog opened in %v", p.dirs)
	}
}

func TestPickPath_WSLRejectsAWindowsFolderWhenItIsPicked(t *testing.T) {
	s := pickSvc(&fakePicker{answer: `C:\Source\dv`}, "/home/u", `\\wsl.localhost\Ubuntu\home\u`)
	got, err := s.PickPath(context.Background(), "wsl", "folder", "Choose", "", "")
	if err == nil || got != "" || !strings.Contains(err.Error(), "Windows path") {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestPickPath_CancelReturnsNothing(t *testing.T) {
	s := pickSvc(&fakePicker{}, "/home/u", `\\wsl.localhost\Ubuntu\home\u`)
	if got, err := s.PickPath(context.Background(), "wsl", "folder", "Choose", "", ""); err != nil || got != "" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestPickPath_LocalConnectionUsesThePickedPathAsItIs(t *testing.T) {
	p := &fakePicker{answer: `C:\Source\dv`}
	s := pickSvc(p, "/home/u")
	got, err := s.PickPath(context.Background(), "ssh", "folder", "Choose", "", "")
	if err != nil || got != `C:\Source\dv` || p.dirs[0] != "" {
		t.Errorf("got %q, %v, opened in %v", got, err, p.dirs)
	}
}

func TestPickPath_RejectsAnUnknownKind(t *testing.T) {
	if _, err := pickSvc(&fakePicker{}, "").PickPath(context.Background(), "ssh", "volume", "Choose", "", ""); err == nil {
		t.Error("an unknown kind must fail")
	}
}
