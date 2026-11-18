package hostopen

import (
	"fmt"
	"strings"

	"docklet/internal/cmdtemplate"
	"docklet/internal/model"
)

// Env is what editor detection needs from the machine; tests replace it.
type Env struct {
	GOOS   string
	Look   func(name string) (string, error) // PATH lookup
	Exists func(p string) bool
	Getenv func(key string) string
}

// Editor is a detected editor program.
type Editor struct {
	Name string // shown to the user, e.g. "VS Code"
	Exe  string
	// VSCodeLike editors understand --remote wsl+<distro>.
	VSCodeLike bool
}

type editorSpec struct {
	mode       string // value of EditorSetting.Mode that selects only this editor
	name       string
	bin        string // name on PATH
	winExe     string // real executable on Windows (the PATH entry is a .cmd shim)
	winDirs    []string
	macPaths   []string
	vscodeLike bool
}

// Detection order for "auto".
var editorSpecs = []editorSpec{
	{mode: "vscode", name: "VS Code", bin: "code", winExe: "Code.exe", vscodeLike: true,
		winDirs:  []string{"%LOCALAPPDATA%\\Programs\\Microsoft VS Code", "%ProgramFiles%\\Microsoft VS Code"},
		macPaths: []string{"/usr/local/bin/code", "/opt/homebrew/bin/code", "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code"}},
	{mode: "cursor", name: "Cursor", bin: "cursor", winExe: "Cursor.exe", vscodeLike: true,
		winDirs:  []string{"%LOCALAPPDATA%\\Programs\\cursor", "%ProgramFiles%\\cursor"},
		macPaths: []string{"/usr/local/bin/cursor", "/opt/homebrew/bin/cursor", "/Applications/Cursor.app/Contents/Resources/app/bin/cursor"}},
	{name: "VS Code Insiders", bin: "code-insiders", winExe: "Code - Insiders.exe", vscodeLike: true,
		winDirs:  []string{"%LOCALAPPDATA%\\Programs\\Microsoft VS Code Insiders", "%ProgramFiles%\\Microsoft VS Code Insiders"},
		macPaths: []string{"/usr/local/bin/code-insiders", "/opt/homebrew/bin/code-insiders"}},
	{name: "Zed", bin: "zed", winExe: "zed.exe", winDirs: []string{"%LOCALAPPDATA%\\Programs\\Zed"},
		macPaths: []string{"/usr/local/bin/zed", "/opt/homebrew/bin/zed"}},
	{name: "Sublime Text", bin: "subl", winExe: "subl.exe", winDirs: []string{"%ProgramFiles%\\Sublime Text"},
		macPaths: []string{"/usr/local/bin/subl", "/opt/homebrew/bin/subl"}},
}

// NoEditorMessage is shown when nothing could be found.
const NoEditorMessage = "No code editor was found. Install VS Code or Cursor, or set a custom editor command in Settings › Editor."

// Detect finds the editor for mode "auto", "vscode" or "cursor".
func Detect(env Env, mode string) (Editor, bool) {
	for _, s := range editorSpecs {
		if mode != "auto" && mode != "" && s.mode != mode {
			continue
		}
		if exe, ok := findExe(env, s); ok {
			return Editor{Name: s.name, Exe: exe, VSCodeLike: s.vscodeLike}, true
		}
	}
	return Editor{}, false
}

func findExe(env Env, s editorSpec) (string, bool) {
	if p, err := env.Look(s.bin); err == nil && p != "" {
		if env.GOOS == "windows" {
			// code.cmd / cursor.cmd are batch shims: start the real program, which has no console window.
			if low := strings.ToLower(p); strings.HasSuffix(low, ".cmd") || strings.HasSuffix(low, ".bat") {
				if exe, ok := exeAbove(env, p, s.winExe); ok {
					return exe, true
				}
			}
		}
		return p, true
	}
	switch env.GOOS {
	case "windows":
		for _, d := range s.winDirs {
			if exe := expandEnv(env, d) + "\\" + s.winExe; !strings.HasPrefix(exe, "\\") && env.Exists(exe) {
				return exe, true
			}
		}
	case "darwin":
		for _, p := range s.macPaths {
			if env.Exists(p) {
				return p, true
			}
		}
	}
	return "", false
}

// exeAbove looks for exe in the folder of file and up to three folders above it.
func exeAbove(env Env, file, exe string) (string, bool) {
	dir := file
	for i := 0; i < 4; i++ {
		k := strings.LastIndex(dir, "\\")
		if k <= 2 {
			return "", false
		}
		dir = dir[:k]
		if p := dir + "\\" + exe; env.Exists(p) {
			return p, true
		}
	}
	return "", false
}

// expandEnv replaces %NAME% at the start of d; an unset variable leaves "\..." which callers skip.
func expandEnv(env Env, d string) string {
	if !strings.HasPrefix(d, "%") {
		return d
	}
	end := strings.Index(d[1:], "%")
	if end < 0 {
		return d
	}
	return env.Getenv(d[1:end+1]) + d[end+2:]
}

// EditorArgv builds the command that opens target in the editor chosen by s. ok is false when no
// editor was found (nothing to start); err is a problem with the setting itself.
func EditorArgv(env Env, s model.EditorSetting, target Candidate) (argv []string, editor string, ok bool, err error) {
	if err := ValidateLocal(env.GOOS, target.Path); err != nil {
		return nil, "", false, err
	}
	if s.Mode == "custom" {
		argv, err := cmdtemplate.Expand(s.Template, "{path}", []string{target.Path})
		if err != nil {
			return nil, "", false, fmt.Errorf("editor setting: %w", err)
		}
		return argv, argv[0], true, nil
	}
	ed, found := Detect(env, s.Mode)
	if !found {
		return nil, "", false, nil
	}
	if ed.VSCodeLike && target.WSLDistro != "" && target.LinuxPath != "" {
		return []string{ed.Exe, "--remote", "wsl+" + target.WSLDistro, target.LinuxPath}, ed.Name, true, nil
	}
	return []string{ed.Exe, target.Path}, ed.Name, true, nil
}
