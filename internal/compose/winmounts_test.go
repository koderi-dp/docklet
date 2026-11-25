package compose

import (
	"reflect"
	"strings"
	"testing"
)

func TestFindWindowsBinds_ReturnsOnlyBindMountsFromAWindowsDrive(t *testing.T) {
	got, err := FindWindowsBinds(`{"services":{
	  "web":{"volumes":[
	    {"type":"volume","source":"data","target":"/data"},
	    {"type":"bind","source":"/var/run/docker.sock","target":"/var/run/docker.sock"},
	    {"type":"bind","source":"C:/Source/amagi/studio-e","target":"/studio","read_only":true},
	    {"type":"bind","source":"d:\\conf","target":"/etc/conf"}]},
	  "api":{"volumes":[{"type":"bind","source":"/mnt/c/x","target":"/x"}]}}}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []WindowsBind{
		{Service: "web", Source: "C:/Source/amagi/studio-e", Target: "/studio"},
		{Service: "web", Source: `d:\conf`, Target: "/etc/conf"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
	if _, err := FindWindowsBinds("not json"); err == nil {
		t.Error("expected an error for output that is not the config")
	}
}

const dvFile = `services:
  devrun:
    image: amagiee/devrun:v3.0.2
    volumes:
      - sqlite-data:/app/Database             # the database
      - /var/run/docker.sock:/var/run/docker.sock
      - C:/Source/amagi/studio-e:/studio:ro          # 2. your checkouts (optional, see below)
`

func TestPlanBindVariable_TurnsTheSourceIntoAVariableThatKeepsThePathAsDefault(t *testing.T) {
	e, ok := PlanBindVariable(dvFile, "C:/Source/amagi/studio-e", "/studio", nil)
	if !ok {
		t.Fatal("line not found")
	}
	if e.Line != 7 || e.Variable != "STUDIO_DIR" || e.AlreadyVariable {
		t.Errorf("edit: %+v", e)
	}
	// Target, mode and the comment stay exactly as they were.
	if e.New != "      - ${STUDIO_DIR:-C:/Source/amagi/studio-e}:/studio:ro          # 2. your checkouts (optional, see below)" {
		t.Errorf("new line: %q", e.New)
	}
	out := ApplyLineEdit(dvFile, e)
	if !strings.Contains(out, e.New+"\n") || strings.Count(out, "\n") != strings.Count(dvFile, "\n") || !strings.Contains(out, "sqlite-data:/app/Database             # the database") {
		t.Errorf("file after the edit:\n%s", out)
	}
}

func TestPlanBindVariable_ReadsQuotedBackslashAndLongSyntax(t *testing.T) {
	for _, tc := range []struct{ name, text, source, want string }{
		{"double quoted backslashes", `      - "C:\\Source\\my app:/app"` + "\n", `C:\Source\my app`, `      - "${APP_DIR:-C:\\Source\\my app}:/app"`},
		{"single quoted", `      - 'D:\data:/app:ro'` + "\n", `D:\data`, `      - '${APP_DIR:-D:\data}:/app:ro'`},
		{"long syntax", "        source: C:/Source/x\n        target: /app\n", "C:/Source/x", "        source: ${APP_DIR:-C:/Source/x}"},
		{"other slashes than the config reports", "      - C:\\Source\\x:/app\n", "C:/Source/x", `      - ${APP_DIR:-C:\Source\x}:/app`},
	} {
		e, ok := PlanBindVariable(tc.text, tc.source, "/app", nil)
		if !ok || e.New != tc.want {
			t.Errorf("%s: %q (found %v)", tc.name, e.New, ok)
		}
	}
}

func TestPlanBindVariable_LeavesALineThatAlreadyUsesAVariable(t *testing.T) {
	text := "      - ${STUDIO_DIR:-C:/Source/amagi/studio-e}:/studio:ro\n"
	e, ok := PlanBindVariable(text, "C:/Source/amagi/studio-e", "/studio", nil)
	if !ok || !e.AlreadyVariable || e.Variable != "STUDIO_DIR" || e.New != e.Old {
		t.Errorf("edit: %+v (found %v)", e, ok)
	}
}

func TestPlanBindVariable_FindsNothingWhenThePathIsNotInTheFile(t *testing.T) {
	if _, ok := PlanBindVariable("      - ${STUDIO}:/studio\n", "C:/Source/x", "/studio", nil); ok {
		t.Error("a path that only comes from a variable cannot be edited")
	}
}

func TestBindVariableName(t *testing.T) {
	for _, tc := range []struct {
		target string
		taken  map[string]bool
		want   string
	}{
		{"/studio", nil, "STUDIO_DIR"},
		{"/var/lib/my-data/", nil, "MY_DATA_DIR"},
		{"/", nil, "MOUNT_DIR"},
		{"/9lives", nil, "MOUNT_9LIVES_DIR"},
		{"/studio", map[string]bool{"STUDIO_DIR": true}, "STUDIO_DIR_2"},
	} {
		if got := BindVariableName(tc.target, tc.taken); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.target, got, tc.want)
		}
	}
}

func TestSetEnvVar_AddsOrReplacesOneVariableAndKeepsTheRest(t *testing.T) {
	if got := SetEnvVar("", "STUDIO_DIR", "/mnt/c/Source/x"); got != "STUDIO_DIR=/mnt/c/Source/x\n" {
		t.Errorf("new file: %q", got)
	}
	if got := SetEnvVar("A=1\n# note\nSTUDIO_DIR=old\nB=2", "STUDIO_DIR", "/mnt/c/x"); got != "A=1\n# note\nSTUDIO_DIR=/mnt/c/x\nB=2\n" {
		t.Errorf("replace: %q", got)
	}
	if got := SetEnvVar("A=1", "APP_DIR", "/mnt/c/my app"); got != "A=1\nAPP_DIR=\"/mnt/c/my app\"\n" {
		t.Errorf("append with a space: %q", got)
	}
}
