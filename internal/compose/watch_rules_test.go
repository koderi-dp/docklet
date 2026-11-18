package compose

import (
	"reflect"
	"testing"
)

func TestParseWatchRules_ReadsRulesPerServiceSortedAndRelativizesPaths(t *testing.T) {
	cfg := `{"services":{
	  "web":{"develop":{"watch":[
	    {"path":"C:\\proj\\site","action":"sync","target":"/usr/share/nginx/html","ignore":["*.tmp","node_modules/"],"exec":{"command":null}},
	    {"path":"C:\\proj\\conf","action":"sync+exec","target":"/etc","exec":{"command":["sh","-c","nginx -s reload"]}}]}},
	  "api":{"develop":{"watch":[{"path":"C:\\proj\\api","action":"rebuild"}]}},
	  "cache":{"image":"busybox"},
	  "db":{"develop":{"watch":[]}}}}`
	got, err := ParseWatchRules([]byte(cfg), `c:\proj`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "api" || got[1].Name != "web" {
		t.Fatalf("services (rule-less ones must be left out, sorted): %+v", got)
	}
	web := got[1].Rules
	if len(web) != 2 || web[0].Action != "sync" || web[0].DisplayPath != "./site" || web[0].Target != "/usr/share/nginx/html" {
		t.Errorf("web rule 0: %+v", web)
	}
	if !reflect.DeepEqual(web[0].Ignore, []string{"*.tmp", "node_modules/"}) || web[1].Ignore == nil {
		t.Errorf("ignore: %+v %+v", web[0].Ignore, web[1].Ignore)
	}
	if web[1].Exec != "sh -c nginx -s reload" {
		t.Errorf("exec: %q", web[1].Exec)
	}
	if got[0].Rules[0].DisplayPath != "./api" || got[0].Rules[0].Action != "rebuild" {
		t.Errorf("api: %+v", got[0].Rules)
	}
}

func TestParseWatchRules_NoRulesAndBadJSON(t *testing.T) {
	got, err := ParseWatchRules([]byte(`{"services":{"a":{"image":"x"}}}`), "/p")
	if err != nil || len(got) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := ParseWatchRules([]byte(`not json`), "/p"); err == nil {
		t.Fatal("bad JSON accepted")
	}
}

func TestDisplayPath(t *testing.T) {
	for _, c := range []struct{ dir, p, want string }{
		{"/home/u/app", "/home/u/app/src", "./src"},
		{"/home/u/app/", "/home/u/app", "."},
		{"/home/u/app", "/home/u/application/src", "/home/u/application/src"},
		{`C:\Proj`, `c:\proj\a\B`, "./a/B"},
		{"", "/x", "/x"},
	} {
		if got := DisplayPath(c.dir, c.p); got != c.want {
			t.Errorf("DisplayPath(%q,%q) = %q, want %q", c.dir, c.p, got, c.want)
		}
	}
}

func TestWatchSupportedAndVersionParsing(t *testing.T) {
	for _, c := range []struct {
		out  string
		ok   bool
		text string
		want bool
	}{
		{"5.7.0\n", true, "5.7.0", true},
		{"Docker Compose version v2.22.0", true, "2.22.0", true},
		{"v2.21.9-desktop.1", true, "2.21.9", false},
		{"1.29.2", true, "1.29.2", false},
		{"nonsense", false, "", false},
	} {
		v, text, ok := ParseComposeVersion(c.out)
		if ok != c.ok || text != c.text || (ok && WatchSupported(v) != c.want) {
			t.Errorf("%q: v=%v text=%q ok=%v supported=%v", c.out, v, text, ok, WatchSupported(v))
		}
	}
}
