package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"docklet/internal/compose"
)

type fakePicker struct {
	kind   string // "file" or "folder", from the last call
	answer string
}

func (f *fakePicker) OpenFile(_, _, _ string) (string, error) {
	f.kind = "file"
	return f.answer, nil
}

func (f *fakePicker) OpenDirectory(string) (string, error) {
	f.kind = "folder"
	return f.answer, nil
}

func (f *fakePicker) SaveFile(_, _ string) (string, error) { return "", nil }

// wslpathRunner answers `wslpath -u <windows path>` from `mounted` and fails for every other path,
// as the distribution does for a drive it has not mounted.
func wslpathRunner(mounted map[string]string) *fakeRunner {
	return &fakeRunner{out: func(c compose.Command) (string, error) {
		if len(c.Args) < 2 || c.Args[len(c.Args)-2] != "-u" || !strings.Contains(strings.Join(c.Args, " "), "wslpath") {
			return "", nil
		}
		if linux, ok := mounted[c.Args[len(c.Args)-1]]; ok {
			return linux + "\n", nil
		}
		return "", errors.New("wslpath: Invalid argument")
	}}
}

func pickSvc(p *fakePicker, mounted map[string]string) (*ComposeService, *fakeRunner) {
	r := wslpathRunner(mounted)
	s := newSvc(r, nil)
	s.Picker = p
	return s, r
}

func TestPickPath_WSLReturnsTheLinuxPathOfAFolderInsideTheDistribution(t *testing.T) {
	p := &fakePicker{answer: `\\wsl.localhost\Ubuntu\home\u\app`}
	s, _ := pickSvc(p, nil)
	got, err := s.PickPath(context.Background(), "wsl", "folder", "Choose", "", "")
	if err != nil || got != "/home/u/app" || p.kind != "folder" {
		t.Errorf("got %q, %v (dialog: %s)", got, err, p.kind)
	}
}

func TestPickPath_WSLAsksTheDistributionForTheLinuxPathOfAWindowsFile(t *testing.T) {
	p := &fakePicker{answer: `C:\Source\dv\compose.yaml`}
	s, r := pickSvc(p, map[string]string{`C:\Source\dv\compose.yaml`: "/mnt/c/Source/dv/compose.yaml"})
	got, err := s.PickPath(context.Background(), "wsl", "file", "Choose", "Compose", "*.yaml")
	if err != nil || got != "/mnt/c/Source/dv/compose.yaml" || p.kind != "file" {
		t.Fatalf("got %q, %v (dialog: %s)", got, err, p.kind)
	}
	c := r.cmds[len(r.cmds)-1]
	if c.Name != "wsl.exe" || strings.Join(c.Args, " ") != `-d Ubuntu --exec wslpath -u C:\Source\dv\compose.yaml` {
		t.Errorf("asked with %s %v", c.Name, c.Args)
	}
}

func TestPickPath_WSLRefusesAWindowsDriveTheDistributionCannotReach(t *testing.T) {
	s, _ := pickSvc(&fakePicker{answer: `Z:\apps\shop`}, nil)
	got, err := s.PickPath(context.Background(), "wsl", "folder", "Choose", "", "")
	if err == nil || got != "" || !strings.Contains(err.Error(), "Windows path") || !strings.Contains(err.Error(), "Ubuntu") {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestPickPath_CancelReturnsNothing(t *testing.T) {
	s, r := pickSvc(&fakePicker{}, nil)
	if got, err := s.PickPath(context.Background(), "wsl", "folder", "Choose", "", ""); err != nil || got != "" || r.calls != 0 {
		t.Errorf("got %q, %v, %d commands", got, err, r.calls)
	}
}

func TestPickPath_LocalConnectionUsesThePickedPathAsItIs(t *testing.T) {
	s, r := pickSvc(&fakePicker{answer: `C:\Source\dv`}, nil)
	got, err := s.PickPath(context.Background(), "ssh", "folder", "Choose", "", "")
	if err != nil || got != `C:\Source\dv` || r.calls != 0 {
		t.Errorf("got %q, %v, %d commands", got, err, r.calls)
	}
}

func TestPickPath_RejectsAnUnknownKind(t *testing.T) {
	s, _ := pickSvc(&fakePicker{}, nil)
	if _, err := s.PickPath(context.Background(), "ssh", "volume", "Choose", "", ""); err == nil {
		t.Error("an unknown kind must fail")
	}
}

func TestNormalizeFiles_WSLAcceptsWindowsAndLinuxPathsAndDropsDuplicates(t *testing.T) {
	s, _ := pickSvc(&fakePicker{}, map[string]string{`C:\Source\dv\compose.yaml`: "/mnt/c/Source/dv/compose.yaml"})
	env := compose.ExecEnv{Kind: compose.KindWSL, Distro: "Ubuntu", Location: "WSL: Ubuntu"}
	got, err := s.normalizeFiles(context.Background(), env, []string{`C:\Source\dv\compose.yaml`, "/mnt/c/Source/dv/compose.yaml", " ", "/srv/a.yml"}, "Compose file")
	if err != nil || strings.Join(got, " ") != "/mnt/c/Source/dv/compose.yaml /srv/a.yml" {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestNormalizeFiles_WSLRejectsAnAnswerThatIsNotALinuxPath(t *testing.T) {
	s, _ := pickSvc(&fakePicker{}, map[string]string{`C:\x`: `C:\x`})
	env := compose.ExecEnv{Kind: compose.KindWSL, Distro: "Ubuntu"}
	if _, err := s.normalizeFiles(context.Background(), env, []string{`C:\x`}, "folder"); err == nil || !strings.Contains(err.Error(), "folder") {
		t.Errorf("got %v", err)
	}
}
