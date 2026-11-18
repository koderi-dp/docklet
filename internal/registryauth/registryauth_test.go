package registryauth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	stdout, stderr string
	err            error
	gotName, gotIn string
	gotArgs        []string
}

func (f *fakeRunner) Run(_ context.Context, name, stdin string, args ...string) ([]byte, []byte, error) {
	f.gotName, f.gotIn, f.gotArgs = name, stdin, args
	return []byte(f.stdout), []byte(f.stderr), f.err
}

func cfgDir(t *testing.T, content string) string {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "config.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestRegistryOf_MapsDockerHubAliases(t *testing.T) {
	cases := map[string]string{
		"nginx":                       "docker.io",
		"library/nginx:1":             "docker.io",
		"docker.io/foo/bar":           "docker.io",
		"index.docker.io/foo/bar:1":   "docker.io",
		"ghcr.io/org/app:v1":          "ghcr.io",
		"localhost:5000/team/app:1":   "localhost:5000",
		"registry.example.com:8443/a": "registry.example.com:8443",
	}
	for in, want := range cases {
		got, err := RegistryOf(in)
		if err != nil || got != want {
			t.Errorf("RegistryOf(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, k := range []string{"https://index.docker.io/v1/", "docker.io", "index.docker.io"} {
		if NormalizeHost(k) != "docker.io" {
			t.Errorf("NormalizeHost(%q)", k)
		}
	}
}

func TestResolve_ParsesInlineAuths(t *testing.T) {
	enc := base64.StdEncoding.EncodeToString([]byte("alice:s3cr:et"))
	d := cfgDir(t, fmt.Sprintf(`{"auths":{"https://index.docker.io/v1/":{"auth":%q},"ghcr.io":{"username":"bob","password":"pw"},"empty.io":{}}}`, enc))
	r := &Resolver{ConfigDir: d}
	c, err := r.Resolve(context.Background(), "nginx")
	if err != nil || !c.Has() || c.Source != "config" || c.Username != "alice" || c.Password != "s3cr:et" {
		t.Fatalf("hub: %v %v", c, err)
	}
	if c.ServerAddress() != "https://index.docker.io/v1/" {
		t.Errorf("server address %q", c.ServerAddress())
	}
	c, _ = r.Resolve(context.Background(), "ghcr.io/x/y")
	if c.Username != "bob" || c.Password != "pw" {
		t.Errorf("ghcr: %v", c)
	}
	c, err = r.Resolve(context.Background(), "empty.io/x")
	if err != nil || c.Has() || c.Source != "none" {
		t.Errorf("empty: %v %v", c, err)
	}
	c, _ = r.Resolve(context.Background(), "other.io/x")
	if c.Has() {
		t.Error("unexpected credentials")
	}
}

func TestResolve_MissingConfigMeansNoCredentials(t *testing.T) {
	c, err := (&Resolver{ConfigDir: t.TempDir()}).Resolve(context.Background(), "nginx")
	if err != nil || c.Has() {
		t.Fatalf("%v %v", c, err)
	}
}

func TestResolve_PrefersCredHelperAndInvokesItSafely(t *testing.T) {
	d := cfgDir(t, `{"credsStore":"desktop","credHelpers":{"ghcr.io":"ghhelper"},"auths":{"ghcr.io":{}}}`)
	f := &fakeRunner{stdout: `{"ServerURL":"ghcr.io","Username":"u","Secret":"topsecret"}`}
	r := &Resolver{ConfigDir: d, Runner: f}
	c, err := r.Resolve(context.Background(), "ghcr.io/o/a")
	if err != nil || c.Source != "helper:ghhelper" || c.Password != "topsecret" {
		t.Fatalf("%v %v", c, err)
	}
	if f.gotName != "docker-credential-ghhelper" || f.gotIn != "ghcr.io" || len(f.gotArgs) != 1 || f.gotArgs[0] != "get" {
		t.Errorf("invocation: %q %q %v", f.gotName, f.gotIn, f.gotArgs)
	}
	// Docker Hub: credsStore with the legacy server URL on stdin.
	f.stdout = `{"Username":"<token>","Secret":"tok"}`
	c, err = r.Resolve(context.Background(), "nginx")
	if err != nil || c.Source != "helper:desktop" || c.IdentityToken != "tok" || c.Password != "" {
		t.Fatalf("%v %v", c, err)
	}
	if f.gotName != "docker-credential-desktop" || f.gotIn != "https://index.docker.io/v1/" {
		t.Errorf("hub invocation: %q %q", f.gotName, f.gotIn)
	}
}

func TestResolve_FallsBackToInlineAuthsWhenHelperHasNone(t *testing.T) {
	enc := base64.StdEncoding.EncodeToString([]byte("u:p"))
	d := cfgDir(t, fmt.Sprintf(`{"credsStore":"x","auths":{"reg.io":{"auth":%q}}}`, enc))
	f := &fakeRunner{stdout: "credentials not found in native keychain", err: errors.New("exit status 1")}
	c, err := (&Resolver{ConfigDir: d, Runner: f}).Resolve(context.Background(), "reg.io/a")
	if err != nil || c.Source != "config" || c.Username != "u" {
		t.Fatalf("%v %v", c, err)
	}
	c, err = (&Resolver{ConfigDir: d, Runner: f}).Resolve(context.Background(), "none.io/a")
	if err != nil || c.Has() {
		t.Fatalf("%v %v", c, err)
	}
}

func TestResolve_RedactsHelperFailuresAndRejectsBadHelperNames(t *testing.T) {
	d := cfgDir(t, `{"credsStore":"x"}`)
	f := &fakeRunner{stdout: `{"Secret":"hunter2"}`, stderr: `boom password=hunter2 "Secret":"hunter2"`, err: errors.New("exit status 2")}
	_, err := (&Resolver{ConfigDir: d, Runner: f}).Resolve(context.Background(), "reg.io/a")
	if err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("error must exist and not leak: %v", err)
	}
	d = cfgDir(t, `{"credsStore":"../evil & calc"}`)
	f = &fakeRunner{}
	if _, err := (&Resolver{ConfigDir: d, Runner: f}).Resolve(context.Background(), "reg.io/a"); err == nil || f.gotName != "" {
		t.Errorf("bad helper name must be rejected without running: %v %q", err, f.gotName)
	}
}

func TestCredentials_NeverPrintSecretsButEncodeAuthHeader(t *testing.T) {
	c := Credentials{Registry: "r.io", Source: "config", Username: "u", Password: "hunter2"}
	for _, s := range []string{fmt.Sprint(c), fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c)} {
		if strings.Contains(s, "hunter2") {
			t.Errorf("leaked: %s", s)
		}
	}
	raw, _ := base64.URLEncoding.DecodeString(c.EncodedAuth())
	if !strings.Contains(string(raw), `"password":"hunter2"`) || !strings.Contains(string(raw), `"serveraddress":"r.io"`) {
		t.Errorf("auth header: %s", raw)
	}
	if (Credentials{}).EncodedAuth() != "" {
		t.Error("empty creds must give empty header")
	}
}
