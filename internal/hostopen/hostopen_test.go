package hostopen

import (
	"errors"
	"reflect"
	"testing"

	"docklet/internal/model"
)

func existing(paths ...string) Stat {
	return func(p string) (bool, bool) {
		for _, x := range paths {
			if x == p {
				return true, true
			}
		}
		return false, false
	}
}

func TestResolveMapsEnginePathsToThisComputer(t *testing.T) {
	wsl := Conn{Type: model.ConnWSL, Distro: "Ubuntu-24.04"}
	pipe := Conn{Type: model.ConnNamedPipe}
	tests := []struct {
		name   string
		goos   string
		conn   Conn
		path   string
		exists []string
		want   string // local path, or "" when not local
	}{
		{"WSL linux path becomes the wsl.localhost share", "windows", wsl, "/home/ubuntu/projects/catalo",
			[]string{`\\wsl.localhost\Ubuntu-24.04\home\ubuntu\projects\catalo`}, `\\wsl.localhost\Ubuntu-24.04\home\ubuntu\projects\catalo`},
		{"older Windows without wsl.localhost falls back to wsl$", "windows", wsl, "/home/ubuntu/x",
			[]string{`\\wsl$\Ubuntu-24.04\home\ubuntu\x`}, `\\wsl$\Ubuntu-24.04\home\ubuntu\x`},
		{"WSL /mnt/c goes back to the Windows drive", "windows", wsl, "/mnt/c/Users/me/app",
			[]string{`C:\Users\me\app`}, `C:\Users\me\app`},
		{"dot segments cannot climb out of the distro", "windows", wsl, "/home/ubuntu/../../etc",
			[]string{`\\wsl.localhost\Ubuntu-24.04\etc`}, `\\wsl.localhost\Ubuntu-24.04\etc`},
		{"Docker Desktop /run/desktop/mnt/host", "windows", pipe, "/run/desktop/mnt/host/c/Users/me/app",
			[]string{`C:\Users\me\app`}, `C:\Users\me\app`},
		{"Docker Desktop /host_mnt", "windows", pipe, "/host_mnt/d/data", []string{`D:\data`}, `D:\data`},
		{"Docker Desktop reports a Windows path", "windows", pipe, `c:/Users/me`, []string{`C:\Users\me`}, `C:\Users\me`},
		{"a path inside the Docker Desktop VM is not local", "windows", pipe, "/var/lib/docker/volumes/x/_data", nil, ""},
		{"device paths are not local", "windows", pipe, `\\.\pipe\docker_engine`, nil, ""},
		{"unix socket on Linux uses the path as is", "linux", Conn{Type: model.ConnUnix}, "/srv/app/../app", []string{"/srv/app"}, "/srv/app"},
		{"macOS Docker Desktop /host_mnt prefix is dropped", "darwin", Conn{Type: model.ConnUnix}, "/host_mnt/Users/me/app",
			[]string{"/Users/me/app"}, "/Users/me/app"},
		{"tcp engines are remote", "windows", Conn{Type: model.ConnTCP}, "/srv/app", nil, ""},
		{"ssh engines are remote", "linux", Conn{Type: model.ConnSSH}, "/srv/app", nil, ""},
		{"a WSL connection on Linux is not local", "linux", wsl, "/home/x", nil, ""},
		{"relative paths are refused", "linux", Conn{Type: model.ConnUnix}, "app/data", nil, ""},
		{"NUL is refused", "linux", Conn{Type: model.ConnUnix}, "/srv/a\x00b", nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Resolve(tc.goos, tc.conn, tc.path, existing(tc.exists...))
			if tc.want == "" {
				if r.Exists || r.Reason == "" {
					t.Fatalf("want not local with a reason, got %+v", r)
				}
				return
			}
			if !r.Exists || r.Path != tc.want {
				t.Fatalf("want %q, got %+v", tc.want, r)
			}
		})
	}
}

func TestResolveReportsAMissingPath(t *testing.T) {
	r := Resolve("linux", Conn{Type: model.ConnUnix}, "/srv/gone", existing())
	if r.Exists || r.Path != "/srv/gone" || r.Reason == "" {
		t.Fatalf("got %+v", r)
	}
}

func TestResolveKeepsTheWSLDistroForRemoteEditing(t *testing.T) {
	r := Resolve("windows", Conn{Type: model.ConnWSL, Distro: "Debian"}, "/srv/app", existing(`\\wsl.localhost\Debian\srv\app`))
	if r.WSLDistro != "Debian" || r.LinuxPath != "/srv/app" {
		t.Fatalf("got %+v", r)
	}
}

func TestFileManagerArgv(t *testing.T) {
	tests := []struct {
		goos, path string
		dir        bool
		want       []string
	}{
		{"windows", `C:\Users\me`, true, []string{"explorer.exe", `C:\Users\me`}},
		{"windows", `C:\Users\me\a b\compose.yml`, false, []string{"explorer.exe", `/select,C:\Users\me\a b\compose.yml`}},
		{"darwin", "/Users/me", true, []string{"open", "/Users/me"}},
		{"darwin", "/Users/me/compose.yml", false, []string{"open", "-R", "/Users/me/compose.yml"}},
		{"linux", "/srv/app", true, []string{"xdg-open", "/srv/app"}},
		{"linux", "/srv/app/compose.yml", false, []string{"xdg-open", "/srv/app"}},
	}
	for _, tc := range tests {
		if got := FileManagerArgv(tc.goos, tc.path, tc.dir); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s %s: got %q want %q", tc.goos, tc.path, got, tc.want)
		}
	}
}

func winEnv(onPath map[string]string, files ...string) Env {
	return Env{
		GOOS: "windows",
		Look: func(n string) (string, error) {
			if p, ok := onPath[n]; ok {
				return p, nil
			}
			return "", errors.New("not found")
		},
		Exists: func(p string) bool {
			for _, f := range files {
				if f == p {
					return true
				}
			}
			return false
		},
		Getenv: func(k string) string {
			return map[string]string{"LOCALAPPDATA": `C:\Users\me\AppData\Local`, "ProgramFiles": `C:\Program Files`}[k]
		},
	}
}

func TestEditorArgv(t *testing.T) {
	codeCmd := `C:\Program Files\Microsoft VS Code\bin\code.cmd`
	codeExe := `C:\Program Files\Microsoft VS Code\Code.exe`
	cursorCmd := `C:\Program Files\cursor\resources\app\bin\cursor.cmd`
	cursorExe := `C:\Program Files\cursor\Cursor.exe`
	env := winEnv(map[string]string{"code": codeCmd, "cursor": cursorCmd}, codeExe, cursorExe)
	local := Candidate{Path: `C:\Users\me\app`}
	wsl := Candidate{Path: `\\wsl.localhost\Ubuntu\home\u\app`, WSLDistro: "Ubuntu", LinuxPath: "/home/u/app"}

	tests := []struct {
		name string
		set  model.EditorSetting
		c    Candidate
		want []string
	}{
		{"auto picks VS Code and starts the real exe, not the .cmd shim", model.EditorSetting{Mode: "auto"}, local, []string{codeExe, `C:\Users\me\app`}},
		{"a WSL path opens through the VS Code WSL remote", model.EditorSetting{Mode: "auto"}, wsl, []string{codeExe, "--remote", "wsl+Ubuntu", "/home/u/app"}},
		{"Cursor can be chosen and also uses the WSL remote", model.EditorSetting{Mode: "cursor"}, wsl, []string{cursorExe, "--remote", "wsl+Ubuntu", "/home/u/app"}},
		{"custom template gets the path as one argument", model.EditorSetting{Mode: "custom", Template: `zed --new "{path}"`}, local, []string{"zed", "--new", `C:\Users\me\app`}},
		{"custom template on a WSL path uses the share path", model.EditorSetting{Mode: "custom", Template: "zed {path}"}, wsl, []string{"zed", `\\wsl.localhost\Ubuntu\home\u\app`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			argv, _, ok, err := EditorArgv(env, tc.set, tc.c)
			if err != nil || !ok || !reflect.DeepEqual(argv, tc.want) {
				t.Fatalf("got %q ok=%v err=%v, want %q", argv, ok, err, tc.want)
			}
		})
	}
}

func TestEditorArgvWithoutAnEditorStartsNothing(t *testing.T) {
	argv, _, ok, err := EditorArgv(winEnv(nil), model.EditorSetting{Mode: "auto"}, Candidate{Path: `C:\x`})
	if ok || err != nil || argv != nil {
		t.Fatalf("got %q ok=%v err=%v", argv, ok, err)
	}
}

func TestEditorFoundInTheUsualWindowsFolderWhenNotOnPath(t *testing.T) {
	exe := `C:\Users\me\AppData\Local\Programs\Microsoft VS Code\Code.exe`
	argv, name, ok, _ := EditorArgv(winEnv(nil, exe), model.EditorSetting{Mode: "vscode"}, Candidate{Path: `C:\x`})
	if !ok || name != "VS Code" || argv[0] != exe {
		t.Fatalf("got %q %q %v", argv, name, ok)
	}
}

func TestCustomEditorTemplateIsValidated(t *testing.T) {
	env := winEnv(nil)
	for _, tpl := range []string{"", "zed", "{path}", "{path} --new", "   "} {
		if _, _, _, err := EditorArgv(env, model.EditorSetting{Mode: "custom", Template: tpl}, Candidate{Path: `C:\x`}); err == nil {
			t.Errorf("template %q should be rejected", tpl)
		}
	}
}

func TestEditorRefusesPathsThatAreNotAbsolute(t *testing.T) {
	for _, p := range []string{"-n", `..\x`, "", "C:\\a\x00b"} {
		if _, _, _, err := EditorArgv(winEnv(nil), model.EditorSetting{Mode: "auto"}, Candidate{Path: p}); err == nil {
			t.Errorf("path %q should be rejected", p)
		}
	}
}
