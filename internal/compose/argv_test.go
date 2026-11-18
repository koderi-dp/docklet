package compose

import (
	"reflect"
	"testing"
)

func req(v Verb) Request {
	return Request{Project: "catalo", ConfigFiles: []string{"/srv/a.yml", "/srv/b.yml"}, EnvFiles: []string{"/srv/.env"}, WorkingDir: "/srv", Verb: v}
}

func TestArgv_BuildsExpectedCommandPerVerb(t *testing.T) {
	base := []string{"compose", "-p", "catalo", "--project-directory", "/srv", "-f", "/srv/a.yml", "-f", "/srv/b.yml", "--env-file", "/srv/.env"}
	cases := []struct {
		name string
		mod  func(*Request)
		verb Verb
		tail []string
	}{
		{"up", nil, VerbUp, []string{"up", "-d"}},
		{"up build orphans", func(r *Request) { r.Build, r.RemoveOrphans = true, true }, VerbUp, []string{"up", "-d", "--build", "--remove-orphans"}},
		{"down", nil, VerbDown, []string{"down"}},
		{"down orphans", func(r *Request) { r.RemoveOrphans = true }, VerbDown, []string{"down", "--remove-orphans"}},
		{"down volumes explicit", func(r *Request) { r.Volumes = true }, VerbDown, []string{"down", "--volumes"}},
		{"start", nil, VerbStart, []string{"start"}},
		{"stop", nil, VerbStop, []string{"stop"}},
		{"restart", nil, VerbRestart, []string{"restart"}},
		{"pull", nil, VerbPull, []string{"pull", "--ignore-buildable"}},
		{"build", nil, VerbBuild, []string{"build"}},
		{"config", nil, VerbConfig, []string{"config"}},
	}
	for _, c := range cases {
		r := req(c.verb)
		if c.mod != nil {
			c.mod(&r)
		}
		got, err := Argv(r)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if want := append(append([]string{}, base...), c.tail...); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v want %v", c.name, got, want)
		}
	}
}

func TestArgv_DownRemovesVolumesOnlyWhenExplicit(t *testing.T) {
	got, _ := Argv(req(VerbDown))
	for _, a := range got {
		if a == "--volumes" || a == "-v" {
			t.Fatalf("down must not add --volumes by default: %v", got)
		}
	}
	r := req(VerbStop)
	r.Volumes = true
	if _, err := Argv(r); err == nil {
		t.Fatal("--volumes must be rejected for non-down verbs")
	}
	r = req(VerbPull)
	r.RemoveOrphans = true
	if _, err := Argv(r); err == nil {
		t.Fatal("--remove-orphans must be rejected for pull")
	}
}

func TestValidateProjectName_RejectsShellMetacharactersAndFlags(t *testing.T) {
	for _, bad := range []string{"", "A", "my app", "x;rm -rf /", "-p", "a&b", "a\nb", "$(id)", "../x", "a b"} {
		if ValidateProjectName(bad) == nil {
			t.Errorf("project %q should be rejected", bad)
		}
		r := req(VerbUp)
		r.Project = bad
		if _, err := Argv(r); err == nil {
			t.Errorf("Argv accepted project %q", bad)
		}
	}
	for _, ok := range []string{"catalo", "my-app_2", "0abc"} {
		if err := ValidateProjectName(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
}

func TestValidateDistro_RejectsInjectionAndAcceptsRealNames(t *testing.T) {
	for _, bad := range []string{"", "-d", "Ubuntu; calc", "a|b", "u\"x", "../u", " x"} {
		if ValidateDistro(bad) == nil {
			t.Errorf("distro %q should be rejected", bad)
		}
	}
	for _, ok := range []string{"Ubuntu-22.04", "Debian GNU"} {
		if err := ValidateDistro(ok); err != nil {
			t.Errorf("valid distro %q rejected: %v", ok, err)
		}
	}
	if _, err := BuildCommand(ExecEnv{Kind: KindWSL, Distro: "x; y|z"}, req(VerbUp)); err == nil {
		t.Error("bad distro accepted in BuildCommand")
	}
}

func TestArgv_RejectsInvalidRequests(t *testing.T) {
	cases := map[string]func(*Request){
		"option-like config path": func(r *Request) { r.ConfigFiles = []string{"--env-file=/etc/passwd"} },
		"missing config files":    func(r *Request) { r.ConfigFiles = nil },
		"unknown verb":            func(r *Request) { r.Verb = "exec" },
	}
	for name, mod := range cases {
		r := req(VerbUp)
		mod(&r)
		if _, err := Argv(r); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestArgv_ProfilesServicesAndConfigModes(t *testing.T) {
	r := req(VerbUp)
	r.Profiles = []string{"debug", "web-2"}
	r.Services = []string{"web"}
	r.RemoveOrphans = true
	got, err := Argv(r)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"compose", "-p", "catalo", "--profile", "debug", "--profile", "web-2", "--project-directory", "/srv", "-f", "/srv/a.yml", "-f", "/srv/b.yml", "--env-file", "/srv/.env", "up", "-d", "--remove-orphans", "web"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	r = req(VerbRestart)
	r.Services = []string{"db"}
	if got, _ = Argv(r); got[len(got)-2] != "restart" || got[len(got)-1] != "db" {
		t.Errorf("restart service: %v", got)
	}
	for _, bad := range []string{"-x", "a b", "a;b", "", "--all"} {
		r = req(VerbUp)
		r.Services = []string{bad}
		if _, err := Argv(r); err == nil {
			t.Errorf("service %q accepted", bad)
		}
		r = req(VerbUp)
		r.Profiles = []string{bad}
		if _, err := Argv(r); err == nil {
			t.Errorf("profile %q accepted", bad)
		}
	}
	r = req(VerbDown)
	r.Services = []string{"web"}
	if _, err := Argv(r); err == nil {
		t.Error("down must not take services")
	}
	// Name detection run: no -p, json output, no project required.
	r = Request{ConfigFiles: []string{"/srv/a.yml"}, Verb: VerbConfig, ConfigMode: "json"}
	if got, err = Argv(r); err != nil || !reflect.DeepEqual(got, []string{"compose", "-f", "/srv/a.yml", "config", "--format", "json"}) {
		t.Errorf("json: %v %v", got, err)
	}
	r = Request{Project: "p", ConfigFiles: []string{"/srv/a.yml"}, Verb: VerbConfig, ConfigMode: "quiet"}
	if got, _ = Argv(r); got[len(got)-2] != "config" || got[len(got)-1] != "-q" {
		t.Errorf("quiet: %v", got)
	}
	if _, err := Argv(Request{ConfigFiles: []string{"/a"}, Verb: VerbUp}); err == nil {
		t.Error("project name must stay required for non-config verbs")
	}
}
