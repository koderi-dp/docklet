package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"docklet/internal/buildkit"
	"docklet/internal/compose"
	"docklet/internal/model"
)

type cliRunner struct {
	mu    sync.Mutex
	calls []compose.Command
	// outputs maps a joined-argv suffix to stdout; a command matching no entry fails like a missing CLI.
	outputs map[string]string
}

func (r *cliRunner) Run(context.Context, compose.Command, func(string)) error { return nil }

func (r *cliRunner) Output(_ context.Context, c compose.Command) (string, error) {
	r.mu.Lock()
	r.calls = append(r.calls, c)
	r.mu.Unlock()
	joined := strings.Join(c.Args, " ")
	for suffix, out := range r.outputs {
		if strings.HasSuffix(joined, suffix) {
			return out, nil
		}
	}
	return "", errors.New("executable file not found")
}

type profiles map[string]model.ConnectionProfile

func (p profiles) Profile(id string) (model.ConnectionProfile, error) {
	if pr, ok := p[id]; ok {
		return pr, nil
	}
	return model.ConnectionProfile{}, errors.New("unknown connection")
}

func newBuildSvc(r *cliRunner) *ImageService {
	return &ImageService{
		runner: r,
		Profiles: profiles{
			"local": {ID: "local", Type: model.ConnNamedPipe, Endpoint: "npipe:////./pipe/docker_engine"},
			"wsl":   {ID: "wsl", Type: model.ConnWSL, WSLDistro: "Ubuntu-24.04"},
		},
	}
}

func TestBuildCapability_DetectedPerConnectionAndCached(t *testing.T) {
	r := &cliRunner{outputs: map[string]string{"buildx version": "github.com/docker/buildx v0.19.2 abc\n"}}
	s := newBuildSvc(r)
	ctx := context.Background()
	c, err := s.BuildCapability(ctx, "wsl")
	if err != nil || !c.Available || c.Summary != "BuildKit via docker buildx 0.19" || c.Location != "WSL: Ubuntu-24.04" || c.ContextHint == "" {
		t.Fatalf("%+v %v", c, err)
	}
	if first := r.calls[0]; first.Name != "wsl.exe" || strings.Join(first.Args, " ") != "-d Ubuntu-24.04 --exec docker buildx version" {
		t.Errorf("the CLI must run inside WSL: %+v", first)
	}
	if _, err := s.BuildCapability(ctx, "wsl"); err != nil || len(r.calls) != 1 {
		t.Errorf("second call should be cached, calls = %d", len(r.calls))
	}
	if c, _ = s.BuildCapability(ctx, "local"); !c.Available || c.Location != "This machine" || r.calls[len(r.calls)-1].Name != "docker" {
		t.Errorf("local: %+v", c)
	}
}

func TestBuildCapability_FallsBackToDockerBuildThenClassic(t *testing.T) {
	r := &cliRunner{outputs: map[string]string{"version --format {{.Client.Version}}": "20.10.24\n"}}
	c, _ := newBuildSvc(r).BuildCapability(context.Background(), "local")
	if !c.Available || c.Method != buildkit.MethodDockerBuild || c.RawJSON {
		t.Errorf("legacy docker build: %+v", c)
	}
	c, _ = newBuildSvc(&cliRunner{}).BuildCapability(context.Background(), "local")
	if c.Available || c.Summary != "Classic builder: the docker CLI was not found for this connection" {
		t.Errorf("no CLI: %+v", c)
	}
	s := newBuildSvc(&cliRunner{})
	s.Profiles = profiles{"ctx": {ID: "ctx", Type: model.ConnContext, Endpoint: "tcp://x"}}
	if c, _ = s.BuildCapability(context.Background(), "ctx"); c.Available || !strings.HasPrefix(c.Summary, "Classic builder:") {
		t.Errorf("unsupported transport: %+v", c)
	}
}

func TestBuildPlan_ContextMustBeReachableByTheCLI(t *testing.T) {
	r := &cliRunner{outputs: map[string]string{"buildx version": "github.com/docker/buildx v0.19.2 abc", "test -d /tmp/app": "", "test -f /tmp/app/Dockerfile": ""}}
	s := newBuildSvc(r)
	ctx := context.Background()

	p, err := s.BuildPlan(ctx, "wsl", model.BuildSpec{ContextDir: `C:\work\my app`})
	if err != nil || p.Builder != model.BuilderBuildKit || !strings.Contains(p.ContextProblem, "/mnt/c/work/my app") || !strings.Contains(p.ContextProblem, "Classic") {
		t.Fatalf("windows folder for a WSL CLI: %+v %v", p, err)
	}
	if p, _ = s.BuildPlan(ctx, "wsl", model.BuildSpec{ContextDir: "/tmp/app"}); p.ContextProblem != "" || p.Builder != model.BuilderBuildKit {
		t.Errorf("linux folder inside WSL: %+v", p)
	}
	if p, _ = s.BuildPlan(ctx, "wsl", model.BuildSpec{ContextDir: "/tmp/missing"}); !strings.Contains(p.ContextProblem, "does not exist inside WSL: Ubuntu-24.04") {
		t.Errorf("missing folder: %+v", p)
	}
	if p, _ = s.BuildPlan(ctx, "wsl", model.BuildSpec{ContextDir: "/tmp/app", Dockerfile: "../x"}); p.ContextProblem == "" {
		t.Error("a Dockerfile outside the context must be refused")
	}
	// Chosen Classic: BuildKit stays available but is not used.
	if p, _ = s.BuildPlan(ctx, "wsl", model.BuildSpec{Builder: model.BuilderClassic}); p.Builder != model.BuilderClassic || !strings.Contains(p.Summary, "Classic builder (chosen)") {
		t.Errorf("classic chosen: %+v", p)
	}
	// BuildKit demanded but missing: say why and offer Classic.
	none := newBuildSvc(&cliRunner{})
	if p, _ = none.BuildPlan(ctx, "local", model.BuildSpec{Builder: model.BuilderBuildKit}); p.Builder != model.BuilderClassic || !p.OfferClassic || !strings.Contains(p.ContextProblem, "docker CLI was not found") {
		t.Errorf("buildkit forced without CLI: %+v", p)
	}
	if p, _ = none.BuildPlan(ctx, "local", model.BuildSpec{}); p.Builder != model.BuilderClassic || p.ContextProblem != "" {
		t.Errorf("auto without CLI: %+v", p)
	}
}

func TestBuildRecord_NeverHoldsArgumentValues(t *testing.T) {
	args := []buildkit.Arg{{Name: "API_TOKEN", Value: "tok-12345"}, {Name: "GREETING", Value: "hello-world"}}
	meta := buildMeta{
		connectionID: "c", builder: model.BuilderBuildKit, method: "buildx 0.19.2", contextDir: "/work", dockerfile: "Dockerfile",
		tags: []string{"app:1"}, argNames: []string{"API_TOKEN", "GREETING"}, started: time.Now().Add(-3 * time.Second),
		command: buildkit.CopyCommand(buildkit.Options{Method: buildkit.MethodBuildx, ContextDir: "/work", Args: args}),
		hide:    buildkit.NewScrubber(buildkit.AllValues(args), 4),
	}
	rec := meta.makeRecord(buildOutcome{
		err:  errors.New("Build failed: curl with tok-12345 returned 403"),
		tail: []string{"+ echo hello-world", "token=tok-12345"},
		steps: []model.BuildStep{
			{Name: "[2/2] RUN echo tok-12345", State: model.StepError, Error: "exit 1: hello-world", Log: []string{"tok-12345"}},
			{Name: "[1/2] RUN true", State: model.StepDone, Log: []string{"noisy"}},
		},
	})
	b, _ := json.Marshal(rec)
	for _, leak := range []string{"tok-12345", "hello-world"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("record leaks %q: %s", leak, b)
		}
	}
	if rec.Result != model.BuildFailed || rec.DurationMs < 3000 || len(rec.LogTail) != 2 || rec.Steps[0].Log[0] != "***" || rec.Steps[1].Log != nil {
		t.Errorf("record = %+v", rec)
	}
	if !strings.Contains(rec.Command, "--build-arg API_TOKEN=$API_TOKEN") || len(rec.ArgNames) != 2 {
		t.Errorf("command/arg names: %q %v", rec.Command, rec.ArgNames)
	}
	if c := meta.makeRecord(buildOutcome{cancelled: true, err: context.Canceled}); c.Result != model.BuildCancelled || c.Error != "" || c.LogTail != nil {
		t.Errorf("cancelled: %+v", c)
	}
	if ok := meta.makeRecord(buildOutcome{imageID: "sha256:abc"}); ok.Result != model.BuildSuccess || ok.ImageID != "sha256:abc" {
		t.Errorf("success: %+v", ok)
	}
}
