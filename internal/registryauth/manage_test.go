package registryauth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"docklet/internal/secrets"
)

// scriptRunner is a fake credential helper that remembers entries like a real one.
type scriptRunner struct {
	entries map[string][2]string // serverURL -> user, secret
	failOn  string               // verb that fails
	failOut string
}

func newScript() *scriptRunner { return &scriptRunner{entries: map[string][2]string{}} }

func (f *scriptRunner) Run(_ context.Context, _, stdin string, args ...string) ([]byte, []byte, error) {
	verb := args[0]
	if verb == f.failOn {
		return []byte(f.failOut), nil, errors.New("exit status 1")
	}
	switch verb {
	case "store":
		var in struct{ ServerURL, Username, Secret string }
		if err := json.Unmarshal([]byte(stdin), &in); err != nil {
			return nil, nil, err
		}
		f.entries[in.ServerURL] = [2]string{in.Username, in.Secret}
	case "erase":
		if _, ok := f.entries[stdin]; !ok {
			return []byte("credentials not found in native keychain"), nil, errors.New("exit status 1")
		}
		delete(f.entries, stdin)
	case "get":
		e, ok := f.entries[stdin]
		if !ok {
			return []byte("credentials not found in native keychain"), nil, errors.New("exit status 1")
		}
		b, _ := json.Marshal(map[string]string{"ServerURL": stdin, "Username": e[0], "Secret": e[1]})
		return b, nil, nil
	case "list":
		m := map[string]string{}
		for k, v := range f.entries {
			m[k] = v[0]
		}
		b, _ := json.Marshal(m)
		return b, nil, nil
	}
	return nil, nil, nil
}

func found(string) (string, error)    { return "x", nil }
func notFound(string) (string, error) { return "", errors.New("not found") }

func TestHelper_StoreGetListEraseProtocol(t *testing.T) {
	f := newScript()
	h := Helper{Name: "wincred", Runner: f}
	ctx := context.Background()
	if err := h.Store(ctx, "127.0.0.1:5055", "alice", "pw1"); err != nil {
		t.Fatal(err)
	}
	u, s, err := h.Get(ctx, "127.0.0.1:5055")
	if err != nil || u != "alice" || s != "pw1" {
		t.Fatalf("get: %q %q %v", u, s, err)
	}
	m, err := h.List(ctx)
	if err != nil || m["127.0.0.1:5055"] != "alice" {
		t.Fatalf("list: %v %v", m, err)
	}
	if err := h.Erase(ctx, "127.0.0.1:5055"); err != nil {
		t.Fatal(err)
	}
	if err := h.Erase(ctx, "127.0.0.1:5055"); err != nil {
		t.Errorf("erasing a missing entry must succeed: %v", err)
	}
	if u, s, err := h.Get(ctx, "127.0.0.1:5055"); err != nil || u != "" || s != "" {
		t.Errorf("a missing entry is not an error: %q %q %v", u, s, err)
	}
}

func TestHelper_ErrorsNeverContainTheSecret(t *testing.T) {
	f := newScript()
	f.failOn, f.failOut = "store", `error storing "Secret":"topsecret99" password=topsecret99`
	err := Helper{Name: "x", Runner: f}.Store(context.Background(), "r.io", "u", "topsecret99")
	if err == nil || strings.Contains(err.Error(), "topsecret99") {
		t.Fatalf("want an error without the secret: %v", err)
	}
	if (Helper{Name: "../bad", Runner: f}).Store(context.Background(), "r.io", "u", "p") == nil {
		t.Error("invalid helper names are rejected")
	}
}

func TestServerKeyAndCleanHost(t *testing.T) {
	cases := map[string]string{
		"docker.io": "https://index.docker.io/v1/", "index.docker.io": "https://index.docker.io/v1/",
		"registry-1.docker.io": "https://index.docker.io/v1/", "https://index.docker.io/v1/": "https://index.docker.io/v1/",
		"ghcr.io": "ghcr.io", "https://GHCR.io/": "ghcr.io", "127.0.0.1:5055": "127.0.0.1:5055", "http://reg.local:5000/v2/": "reg.local:5000",
	}
	for in, want := range cases {
		if got := ServerKey(in); got != want {
			t.Errorf("ServerKey(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "  ", "a b", "ghcr.io:abc", "-x.io", "a..b"} {
		if _, err := CleanHost(bad); err == nil {
			t.Errorf("CleanHost(%q) must fail", bad)
		}
	}
}

func TestConfigEdit_PreservesUnknownFieldsOrderAndFormat(t *testing.T) {
	orig := "{\n\t\"auths\": {},\n\t\"credsStore\": \"desktop\",\n\t\"currentContext\": \"x\",\n\t\"plugins\": {\n\t\t\"-x-cli-hints\": {\n\t\t\t\"enabled\": \"true\"\n\t\t}\n\t},\n\t\"<weird>&\": [\n\t\t1,\n\t\t2\n\t]\n}\n"
	d := cfgDir(t, orig)
	p := filepath.Join(d, "config.json")
	if err := markHelperEntry(p, "127.0.0.1:5055"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	got := string(b)
	if !strings.Contains(got, `"127.0.0.1:5055": {}`) || !strings.Contains(got, `"<weird>&": [`) || !strings.Contains(got, `"-x-cli-hints"`) {
		t.Fatalf("unknown fields lost:\n%s", got)
	}
	if strings.Index(got, "auths") > strings.Index(got, "credsStore") || strings.Index(got, "credsStore") > strings.Index(got, "currentContext") {
		t.Errorf("key order changed:\n%s", got)
	}
	if err := removeAuthsEntry(p, "127.0.0.1:5055", true); err != nil {
		t.Fatal(err)
	}
	if b, _ = os.ReadFile(p); string(b) != orig {
		t.Errorf("round trip is not byte-identical:\n%q\nwant\n%q", b, orig)
	}
}

func TestConfigEdit_NeverWritesWhenParsingFailsAndKeepsInlineEntries(t *testing.T) {
	bad := `{"auths": {`
	d := cfgDir(t, bad)
	p := filepath.Join(d, "config.json")
	if err := markHelperEntry(p, "r.io"); err == nil {
		t.Error("invalid JSON must be an error")
	}
	if b, _ := os.ReadFile(p); string(b) != bad {
		t.Errorf("file was modified: %s", b)
	}
	d = cfgDir(t, `{"auths":{"r.io":{"auth":"dTpw"}}}`)
	p = filepath.Join(d, "config.json")
	if err := removeAuthsEntry(p, "r.io", true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); !strings.Contains(string(b), "dTpw") {
		t.Errorf("an entry with inline data must survive removal of helper markers: %s", b)
	}
	if err := removeAuthsEntry(p, "r.io", false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "dTpw") {
		t.Errorf("inline entry should be removed: %s", b)
	}
}

func TestSave_UsesHelperWhenInstalledElseDockletStoreWithoutTouchingConfig(t *testing.T) {
	ctx := context.Background()
	orig := `{"credsStore":"desktop","auths":{}}`
	d := cfgDir(t, orig)
	f, st := newScript(), secrets.NewMemoryStore()
	r := &Resolver{ConfigDir: d, Runner: f, Store: st, LookPath: found}
	e, err := r.Save(ctx, "127.0.0.1:5055", "alice", "pw1")
	if err != nil || e.Kind != KindHelper || e.Helper != "desktop" || f.entries["127.0.0.1:5055"][1] != "pw1" {
		t.Fatalf("helper save: %+v %v", e, err)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "config.json")); !strings.Contains(string(b), `"127.0.0.1:5055": {}`) {
		t.Errorf("docker CLI marker missing: %s", b)
	}
	if c, _ := r.ResolveHost(ctx, "127.0.0.1:5055"); c.Source != "helper:desktop" || c.Password != "pw1" {
		t.Errorf("resolve: %v", c)
	}
	if err := r.Remove(ctx, "127.0.0.1:5055", KindHelper); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "config.json")); strings.Contains(string(b), "5055") || len(f.entries) != 0 {
		t.Errorf("entry or marker not removed: %s", b)
	}

	// Helper named in config but not installed: Docklet's own store, config untouched.
	d = cfgDir(t, orig)
	r = &Resolver{ConfigDir: d, Runner: newScript(), Store: st, LookPath: notFound}
	e, err = r.Save(ctx, "ghcr.io", "bob", "tok")
	if err != nil || e.Kind != KindDocklet {
		t.Fatalf("docklet save: %+v %v", e, err)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "config.json")); string(b) != orig {
		t.Errorf("config.json must not change: %s", b)
	}
	list, _ := r.List(ctx)
	if len(list) != 1 || list[0].Host != "ghcr.io" || list[0].Username != "bob" || list[0].Kind != KindDocklet {
		t.Errorf("list: %+v", list)
	}
	if err := r.Remove(ctx, "ghcr.io", KindDocklet); err != nil {
		t.Fatal(err)
	}
	if list, _ = r.List(ctx); len(list) != 0 {
		t.Errorf("after sign out: %+v", list)
	}
}

func TestResolvePrecedence_HelperThenDockletThenConfig(t *testing.T) {
	ctx := context.Background()
	d := cfgDir(t, `{"credsStore":"desktop","auths":{"r.io":{"username":"cfg","password":"cfgpw"}}}`)
	f, st := newScript(), secrets.NewMemoryStore()
	r := &Resolver{ConfigDir: d, Runner: f, Store: st}
	if c, _ := r.ResolveHost(ctx, "r.io"); c.Source != "config" {
		t.Fatalf("config only: %v", c)
	}
	if err := SaveDocklet(st, "r.io", "dk", "dkpw"); err != nil {
		t.Fatal(err)
	}
	if c, _ := r.ResolveHost(ctx, "r.io"); c.Source != "docklet" || c.Username != "dk" || c.Password != "dkpw" {
		t.Fatalf("docklet beats config: %v", c)
	}
	f.entries["r.io"] = [2]string{"hp", "hppw"}
	if c, _ := r.ResolveHost(ctx, "r.io"); c.Source != "helper:desktop" || c.Password != "hppw" {
		t.Fatalf("helper beats docklet: %v", c)
	}
	// An unparsable config does not hide Docklet's own sign-ins.
	r = &Resolver{ConfigDir: cfgDir(t, `{broken`), Store: st}
	if c, err := r.ResolveHost(ctx, "r.io"); err != nil || c.Source != "docklet" {
		t.Errorf("broken config: %v %v", c, err)
	}
}

func TestList_MergesSourcesOncePerRegistry(t *testing.T) {
	d := cfgDir(t, `{"credsStore":"desktop","auths":{"old.io":{"username":"o","password":"p"},"both.io":{"username":"c","password":"p"}}}`)
	f, st := newScript(), secrets.NewMemoryStore()
	f.entries["https://index.docker.io/v1/"] = [2]string{"hubuser", "x"}
	f.entries["https://index.docker.io/v1/access-token"] = [2]string{"desktop-internal", "x"}
	f.entries["both.io"] = [2]string{"helperuser", "x"}
	_ = SaveDocklet(st, "both.io", "dockletuser", "x")
	list, warns := (&Resolver{ConfigDir: d, Runner: f, Store: st, LookPath: found}).List(context.Background())
	if len(warns) != 0 {
		t.Errorf("warnings: %v", warns)
	}
	var got []string
	for _, e := range list {
		got = append(got, e.Host+"/"+e.Username+"/"+e.Kind)
	}
	sort.Strings(got)
	want := "both.io/helperuser/helper,docker.io/hubuser/helper,old.io/o/config"
	if strings.Join(got, ",") != want {
		t.Errorf("got %v", got)
	}
}

func TestDockletStore_TokenAndListNeverExposeSecret(t *testing.T) {
	st := secrets.NewMemoryStore()
	if err := SaveDocklet(st, "https://Quay.io/", "<token>", "idtok"); err != nil {
		t.Fatal(err)
	}
	list, err := ListDocklet(st)
	if err != nil || len(list) != 1 || list[0].Host != "quay.io" {
		t.Fatalf("%+v %v", list, err)
	}
	if strings.Contains(list[0].Username+list[0].Host, "idtok") {
		t.Error("secret leaked into the list")
	}
	c, ok := (&Resolver{Store: st}).fromDockletStore("quay.io")
	if !ok || c.IdentityToken != "idtok" || c.Password != "" {
		t.Errorf("token entry: %v", c)
	}
}
