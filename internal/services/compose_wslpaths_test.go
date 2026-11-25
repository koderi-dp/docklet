package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklet/internal/compose"
	"docklet/internal/model"
)

// wslPathProject is a project folder on this machine standing in for one inside a distribution:
// the file access is real, the Windows-to-Linux translation is a table.
func wslPathProject(t *testing.T, composeText string) (model.ComposeProject, wslPathDeps, string) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(file, []byte(composeText), 0o644); err != nil {
		t.Fatal(err)
	}
	p := model.ComposeProject{Name: "dv", WorkingDir: dir, ConfigFiles: []string{file}, EnvFiles: []string{}}
	fs := compose.LocalFS{}
	d := wslPathDeps{
		io: fs, fs: fs, creator: fs, kind: compose.KindLocal,
		translate: func(_ context.Context, p string) (string, error) {
			if strings.HasPrefix(strings.ToLower(p), "c:") {
				return "/mnt/c" + strings.ReplaceAll(p[2:], `\`, "/"), nil
			}
			return "", errors.New("the drive is not mounted in the distribution")
		},
	}
	return p, d, file
}

const dvCompose = "services:\r\n  devrun:\r\n    image: x\r\n    volumes:\r\n      - data:/app/data\r\n      - C:/Source/amagi/studio-e:/studio:ro   # your checkouts\r\n      - Z:/far:/far\r\nvolumes:\r\n  data:\r\n"

const dvConfig = `{"services":{"devrun":{"volumes":[
  {"type":"volume","source":"data","target":"/app/data"},
  {"type":"bind","source":"C:/Source/amagi/studio-e","target":"/studio","read_only":true},
  {"type":"bind","source":"Z:/far","target":"/far"}]}}}`

func TestPlanWSLPaths_OffersAVariableForAReachableDriveAndExplainsAnUnreachableOne(t *testing.T) {
	p, d, file := wslPathProject(t, dvCompose)
	rep, err := planWSLPaths(bg, p, dvConfig, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Issues) != 2 || rep.EnvFile != filepath.Join(p.WorkingDir, ".env.wsl") || !rep.Fixable {
		t.Fatalf("report: %+v", rep)
	}
	a := rep.Issues[0]
	if a.Service != "devrun" || a.LinuxPath != "/mnt/c/Source/amagi/studio-e" || a.Variable != "STUDIO_DIR" || a.File != file || a.Line != 6 || a.Problem != "" {
		t.Errorf("fixable issue: %+v", a)
	}
	if a.NewLine != "      - ${STUDIO_DIR:-C:/Source/amagi/studio-e}:/studio:ro   # your checkouts" {
		t.Errorf("new line: %q", a.NewLine)
	}
	if b := rep.Issues[1]; b.LinuxPath != "" || !strings.Contains(b.Problem, "not mounted") || b.NewLine != "" {
		t.Errorf("unreachable issue: %+v", b)
	}
}

func TestPlanWSLPaths_DoesNotEditAPathThatOnlyComesFromAVariable(t *testing.T) {
	p, d, _ := wslPathProject(t, "services:\n  devrun:\n    volumes:\n      - ${STUDIO}:/studio\n")
	rep, err := planWSLPaths(bg, p, `{"services":{"devrun":{"volumes":[{"type":"bind","source":"C:/Source/x","target":"/studio"}]}}}`, d)
	if err != nil || len(rep.Issues) != 1 {
		t.Fatalf("%+v, %v", rep, err)
	}
	if i := rep.Issues[0]; i.NewLine != "" || i.LinuxPath != "/mnt/c/Source/x" || !strings.Contains(i.Problem, "variable") || rep.Fixable {
		t.Errorf("issue: %+v (fixable %v)", i, rep.Fixable)
	}
}

func TestPlanWSLPaths_NothingToReportWithoutWindowsMounts(t *testing.T) {
	p, d, _ := wslPathProject(t, "services: {}\n")
	rep, err := planWSLPaths(bg, p, `{"services":{"a":{"volumes":[{"type":"bind","source":"/srv/x","target":"/x"}]}}}`, d)
	if err != nil || len(rep.Issues) != 0 || rep.Fixable {
		t.Errorf("%+v, %v", rep, err)
	}
}

func TestApplyWSLPaths_EditsOnlyThatLineAndWritesTheLinuxPathToTheEnvFile(t *testing.T) {
	p, d, file := wslPathProject(t, dvCompose)
	if err := os.WriteFile(filepath.Join(p.WorkingDir, ".env"), []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, _ := planWSLPaths(bg, p, dvConfig, d)
	envFiles, undo, err := applyWSLPaths(bg, p, rep, d)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(file)
	want := strings.Replace(dvCompose, "- C:/Source/amagi/studio-e:/studio:ro", "- ${STUDIO_DIR:-C:/Source/amagi/studio-e}:/studio:ro", 1)
	if string(got) != want {
		t.Errorf("compose file (line endings and every other line must be kept):\n%q", got)
	}
	env, _ := os.ReadFile(rep.EnvFile)
	if string(env) != "STUDIO_DIR=/mnt/c/Source/amagi/studio-e\n" {
		t.Errorf("env file: %q", env)
	}
	// Compose stops reading .env on its own once --env-file is given, so .env goes first.
	if len(envFiles) != 2 || envFiles[0] != filepath.Join(p.WorkingDir, ".env") || envFiles[1] != rep.EnvFile {
		t.Errorf("env files: %v", envFiles)
	}

	undo()
	if back, _ := os.ReadFile(file); string(back) != dvCompose {
		t.Errorf("undo did not restore the compose file: %q", back)
	}
	if _, err := os.Stat(rep.EnvFile); !os.IsNotExist(err) {
		t.Error("undo left the env file it created")
	}
}

func TestApplyWSLPaths_KeepsWhatAnExistingEnvFileHoldsAndTheProjectsOwnEnvFiles(t *testing.T) {
	p, d, _ := wslPathProject(t, dvCompose)
	envWSL := filepath.Join(p.WorkingDir, ".env.wsl")
	other := filepath.Join(p.WorkingDir, "prod.env")
	for f, text := range map[string]string{envWSL: "KEEP=1\nSTUDIO_DIR=old\n", other: "B=2\n"} {
		if err := os.WriteFile(f, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p.EnvFiles = []string{other}
	rep, _ := planWSLPaths(bg, p, dvConfig, d)
	envFiles, undo, err := applyWSLPaths(bg, p, rep, d)
	if err != nil {
		t.Fatal(err)
	}
	if env, _ := os.ReadFile(envWSL); string(env) != "KEEP=1\nSTUDIO_DIR=/mnt/c/Source/amagi/studio-e\n" {
		t.Errorf("env file: %q", env)
	}
	if len(envFiles) != 2 || envFiles[0] != other || envFiles[1] != envWSL {
		t.Errorf("env files: %v", envFiles)
	}
	undo()
	if env, _ := os.ReadFile(envWSL); string(env) != "KEEP=1\nSTUDIO_DIR=old\n" {
		t.Errorf("undo did not restore the env file: %q", env)
	}
}

func TestApplyWSLPaths_OnlySetsTheVariableWhenTheLineAlreadyUsesOne(t *testing.T) {
	text := "services:\n  devrun:\n    volumes:\n      - ${STUDIO_DIR:-C:/Source/amagi/studio-e}:/studio:ro\n"
	p, d, file := wslPathProject(t, text)
	rep, _ := planWSLPaths(bg, p, `{"services":{"devrun":{"volumes":[{"type":"bind","source":"C:/Source/amagi/studio-e","target":"/studio"}]}}}`, d)
	if len(rep.Issues) != 1 || !rep.Fixable || rep.Issues[0].Variable != "STUDIO_DIR" {
		t.Fatalf("report: %+v", rep)
	}
	if _, _, err := applyWSLPaths(bg, p, rep, d); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); string(got) != text {
		t.Errorf("the compose file must not change: %q", got)
	}
	if env, _ := os.ReadFile(rep.EnvFile); string(env) != "STUDIO_DIR=/mnt/c/Source/amagi/studio-e\n" {
		t.Errorf("env file: %q", env)
	}
}
