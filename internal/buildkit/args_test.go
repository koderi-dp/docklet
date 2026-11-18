package buildkit

import (
	"reflect"
	"strings"
	"testing"

	"docklet/internal/compose"
)

func fullOptions() Options {
	return Options{
		Method: MethodBuildx, RawJSON: true, ContextDir: "/work/app", Dockerfile: "/work/app/docker/Dockerfile.dev",
		Tags: []string{"app:1", "app:latest"}, Target: "final", Platform: "linux/arm64", NoCache: true, Pull: true,
		Args: []Arg{{"GREETING", "hello world"}, {"API_TOKEN", "s3cr3t"}},
	}
}

func TestArgv_EveryOptionAndSecretsStayOutOfArgv(t *testing.T) {
	args, env, err := Argv(fullOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"buildx", "build", "--builder", "default", "--load", "--progress=rawjson",
		"-f", "/work/app/docker/Dockerfile.dev", "-t", "app:1", "-t", "app:latest",
		"--build-arg", "GREETING=hello world", "--build-arg", "API_TOKEN",
		"--target", "final", "--platform", "linux/arm64", "--no-cache", "--pull", "/work/app"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("args:\n got %q\nwant %q", args, want)
	}
	if !reflect.DeepEqual(env, []string{"API_TOKEN=s3cr3t"}) {
		t.Errorf("env = %q", env)
	}
	if strings.Contains(strings.Join(args, " "), "s3cr3t") {
		t.Error("a secret value reached argv")
	}
}

func TestArgv_Variants(t *testing.T) {
	o := Options{Method: MethodBuildx, ContextDir: "/c"}
	if a, _, _ := Argv(o); !reflect.DeepEqual(a, []string{"buildx", "build", "--builder", "default", "--load", "--progress=plain", "/c"}) {
		t.Errorf("plain progress: %q", a)
	}
	a, env, _ := Argv(Options{Method: MethodDockerBuild, ContextDir: "/c", Tags: []string{"x:1"}})
	if !reflect.DeepEqual(a, []string{"build", "--progress=plain", "-t", "x:1", "/c"}) || !reflect.DeepEqual(env, []string{"DOCKER_BUILDKIT=1"}) {
		t.Errorf("docker build: %q %q", a, env)
	}
	if _, _, err := Argv(Options{Method: "x", ContextDir: "/c"}); err == nil {
		t.Error("unknown method must fail")
	}
	if _, _, err := Argv(Options{Method: MethodBuildx}); err == nil {
		t.Error("missing context must fail")
	}
	// Names that cannot be environment variables, and reserved ones, fall back to NAME=value.
	a, env, _ = Argv(Options{Method: MethodBuildx, ContextDir: "/c", Args: []Arg{{"my.secret", "v1"}, {"DOCKER_AUTH_TOKEN", "v2"}}})
	if len(env) != 0 || !strings.Contains(strings.Join(a, " "), "my.secret=v1") {
		t.Errorf("fallback: %q %q", a, env)
	}
}

func TestIsSecretName(t *testing.T) {
	for _, n := range []string{"API_TOKEN", "npm_token", "DB_PASSWORD", "SECRET", "GITHUB_PAT_SECRET", "ssh_private_key", "API_KEY", "key", "AWS_ACCESS_KEY_ID", "AUTH"} {
		if !IsSecretName(n) {
			t.Errorf("%s should be secret-looking", n)
		}
	}
	for _, n := range []string{"GREETING", "NODE_VERSION", "BASE_IMAGE", "PORT", "KEYBOARD_LAYOUT"} {
		if IsSecretName(n) {
			t.Errorf("%s should not be secret-looking", n)
		}
	}
}

func TestCopyCommand_ReplacesValuesWithVariables(t *testing.T) {
	got := CopyCommand(fullOptions())
	want := "docker buildx build --load -f /work/app/docker/Dockerfile.dev -t app:1 -t app:latest " +
		"--build-arg GREETING=$GREETING --build-arg API_TOKEN=$API_TOKEN --target final --platform linux/arm64 --no-cache --pull /work/app"
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
	if strings.Contains(got, "s3cr3t") || strings.Contains(got, "hello world") {
		t.Error("values leaked into the copied command")
	}
	q := CopyCommand(Options{Method: MethodDockerBuild, ContextDir: `C:\My Projects\app`})
	if q != `DOCKER_BUILDKIT=1 docker build 'C:\My Projects\app'` {
		t.Errorf("quoting: %s", q)
	}
}

func TestCommand_PerExecutionEnvironment(t *testing.T) {
	o := fullOptions()
	local := compose.SelectEnv(modelProfile("npipe", "npipe:////./pipe/docker_engine", ""))
	c, err := Command(local, o, "")
	if err != nil || c.Name != "docker" || c.Args[0] != "buildx" || !contains(c.EnvSet, "DOCKER_HOST=npipe:////./pipe/docker_engine") || !contains(c.EnvSet, "API_TOKEN=s3cr3t") {
		t.Fatalf("local: %+v %v", c, err)
	}
	wsl := compose.SelectEnv(modelProfile("wsl", "", "Ubuntu-24.04"))
	c, err = Command(wsl, o, "FOO/p")
	if err != nil || c.Name != "wsl.exe" {
		t.Fatalf("wsl: %+v %v", c, err)
	}
	if !reflect.DeepEqual(c.Args[:6], []string{"-d", "Ubuntu-24.04", "--cd", "/work/app", "--exec", "docker"}) {
		t.Errorf("wsl args: %q", c.Args)
	}
	if !contains(c.EnvSet, "WSLENV=FOO/p:API_TOKEN") || !contains(c.EnvSet, "API_TOKEN=s3cr3t") {
		t.Errorf("wsl env: %q", c.EnvSet)
	}
	if strings.Contains(strings.Join(c.Args, " "), "s3cr3t") {
		t.Error("secret in wsl argv")
	}
	o.ContextDir = `C:\app`
	if _, err = Command(wsl, o, ""); err == nil {
		t.Error("a Windows path must not be handed to the CLI inside WSL")
	}
	if _, err = Command(compose.ExecEnv{Kind: compose.KindNone, Reason: "nope"}, fullOptions(), ""); err == nil {
		t.Error("unsupported connection types must fail")
	}
}

func TestScrubber(t *testing.T) {
	s := NewScrubber([]string{"abc", "abcdef", "xy", "abc"}, 3)
	if got := s.Apply("a abcdef b abc xy"); got != "a *** b *** xy" {
		t.Errorf("got %q", got)
	}
	if (*Scrubber)(nil).Apply("same") != "same" {
		t.Error("nil scrubber must be a no-op")
	}
	args := []Arg{{"API_TOKEN", "t0k"}, {"GREETING", "hi there"}}
	if v := SecretValues(args); !reflect.DeepEqual(v, []string{"t0k"}) {
		t.Errorf("secret values %q", v)
	}
	if v := AllValues(args); len(v) != 2 {
		t.Errorf("all values %q", v)
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
