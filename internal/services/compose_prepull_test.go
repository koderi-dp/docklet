package services

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"docklet/internal/compose"
	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/registryauth"
	"docklet/internal/secrets"
)

// pullEngine is a fake Engine for the pre-pull: it knows which images are present, records every
// pull with the user it was authorised as, and can refuse a pull the way a registry does.
type pullEngine struct {
	mu      sync.Mutex
	present map[string]bool
	refuse  string    // error text of every pull; "" lets pulls succeed
	events  *[]string // shared with the CLI runner, to check the order
	pulls   []string  // "<image> as <user>"
}

func (e *pullEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path[strings.Index(r.URL.Path[1:], "/")+1:] // drop /v1.45
	switch {
	case path == "/images/create":
		img := r.URL.Query().Get("fromImage") + ":" + r.URL.Query().Get("tag")
		var auth struct {
			Username string `json:"username"`
		}
		raw, _ := base64.URLEncoding.DecodeString(r.Header.Get("X-Registry-Auth"))
		_ = json.Unmarshal(raw, &auth)
		e.mu.Lock()
		e.pulls = append(e.pulls, img+" as "+auth.Username)
		*e.events = append(*e.events, "pull "+img)
		e.mu.Unlock()
		if e.refuse != "" {
			_, _ = w.Write([]byte(`{"errorDetail":{"message":"` + e.refuse + `"},"error":"` + e.refuse + `"}` + "\n"))
			return
		}
		_, _ = w.Write([]byte(`{"status":"Status: Downloaded newer image for ` + img + `"}` + "\n"))
	case strings.HasPrefix(path, "/images/") && strings.HasSuffix(path, "/json"):
		ref := strings.TrimSuffix(strings.TrimPrefix(path, "/images/"), "/json")
		if !e.present[ref] {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"No such image: ` + ref + `"}`))
			return
		}
		_, _ = w.Write([]byte(`{"Id":"sha256:1"}`))
	default:
		_, _ = w.Write([]byte(`[]`))
	}
}

const prePullConfig = `{"name":"shop","services":{
  "web": {"image":"r.io/team/web:1"},
  "db":  {"image":"postgres:16"}
}}`

// newPrePullSvc is newVerbSvc with a fake Engine, a sign-in for r.io saved in Docklet and a CLI
// whose `config --format json` answers configJSON.
func newPrePullSvc(t *testing.T, eng *pullEngine, configJSON string) (*ComposeService, *fakeRunner, *ops.Manager, *[]string) {
	t.Helper()
	s, runner, m := newVerbSvc(t)
	var events []string
	eng.events = &events
	s.Engines = fakeProvider{dockerClient(t, eng, "1.45")}
	st := secrets.NewMemoryStore()
	if err := registryauth.SaveDocklet(st, "r.io", "amy", "s3cret"); err != nil {
		t.Fatal(err)
	}
	s.Registry = &registryauth.Resolver{ConfigDir: t.TempDir(), Store: st}
	runner.out = func(c compose.Command) (string, error) {
		if slices.Contains(c.Args, "json") {
			return configJSON, nil
		}
		return "", nil
	}
	return s, runner, m, &events
}

// ranVerb reports the CLI invocation of a verb (nil when it never ran).
func ranVerb(r *fakeRunner, verb string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.cmds {
		if slices.Contains(c.Args, verb) {
			return c.Args
		}
	}
	return nil
}

func TestComposeUp_PullsAMissingImageWithTheSavedSignInBeforeTheCLIRuns(t *testing.T) {
	eng := &pullEngine{present: map[string]bool{}}
	s, runner, m, _ := newPrePullSvc(t, eng, prePullConfig)
	op, err := s.Up(bg, "c1", "shop", model.ComposeUpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded {
		t.Fatalf("operation: %+v", done)
	}
	// Only the image with a sign-in is pulled here; postgres stays with Compose.
	if len(eng.pulls) != 1 || eng.pulls[0] != "r.io/team/web:1 as amy" {
		t.Errorf("pulls: %v", eng.pulls)
	}
	if ranVerb(runner, "up") == nil {
		t.Error("docker compose up did not run")
	}
}

func TestComposeUp_LeavesAnImageThatIsAlreadyPresent(t *testing.T) {
	eng := &pullEngine{present: map[string]bool{"r.io/team/web:1": true}}
	s, runner, m, _ := newPrePullSvc(t, eng, prePullConfig)
	op, _ := s.Up(bg, "c1", "shop", model.ComposeUpOptions{})
	waitOp(t, m, op.ID)
	if len(eng.pulls) != 0 || ranVerb(runner, "up") == nil {
		t.Errorf("pulls: %v, up: %v", eng.pulls, ranVerb(runner, "up"))
	}
}

func TestComposeUp_StopsWithTheRegistryReasonWhenThePullIsRefused(t *testing.T) {
	eng := &pullEngine{present: map[string]bool{}, refuse: "manifest for r.io/team/web:1 not found: manifest unknown"}
	s, runner, m, _ := newPrePullSvc(t, eng, prePullConfig)
	op, _ := s.Up(bg, "c1", "shop", model.ComposeUpOptions{})
	done := waitOp(t, m, op.ID)
	if done.Status != model.OpFailed || !strings.Contains(done.Error, "manifest unknown") || !strings.Contains(done.Error, "r.io/team/web:1") {
		t.Errorf("operation: %+v", done)
	}
	if ranVerb(runner, "up") != nil {
		t.Error("docker compose up ran although an image could not be pulled")
	}
}

func TestComposePull_PullsSignedInImagesHereAndLeavesTheRestToTheCLI(t *testing.T) {
	// Present images are pulled again: Pull is asked to refresh them.
	eng := &pullEngine{present: map[string]bool{"r.io/team/web:1": true}}
	s, runner, m, events := newPrePullSvc(t, eng, prePullConfig)
	op, _ := s.Pull(bg, "c1", "shop")
	if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded {
		t.Fatalf("operation: %+v", done)
	}
	if len(eng.pulls) != 1 || eng.pulls[0] != "r.io/team/web:1 as amy" {
		t.Errorf("pulls: %v", eng.pulls)
	}
	args := ranVerb(runner, "pull")
	if args == nil || args[len(args)-1] != "db" || slices.Contains(args, "web") {
		t.Errorf("docker compose pull must be limited to db: %v (events %v)", args, *events)
	}
}

func TestComposePull_SkipsTheCLIWhenEveryImageWasPulledHere(t *testing.T) {
	eng := &pullEngine{present: map[string]bool{}}
	s, runner, m, _ := newPrePullSvc(t, eng, `{"services":{"web":{"image":"r.io/team/web:1"},"api":{"image":"r.io/team/web:1"}}}`)
	op, _ := s.Pull(bg, "c1", "shop")
	if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded {
		t.Fatalf("operation: %+v", done)
	}
	if len(eng.pulls) != 1 {
		t.Errorf("one image shared by two services must be pulled once: %v", eng.pulls)
	}
	if args := ranVerb(runner, "pull"); args != nil {
		t.Errorf("docker compose pull ran with nothing left to pull: %v", args)
	}
}

func TestComposeUp_RunsTheCLIAsBeforeWhenTheConfigCannotBeRead(t *testing.T) {
	eng := &pullEngine{present: map[string]bool{}}
	s, runner, m, _ := newPrePullSvc(t, eng, "not json")
	op, _ := s.Up(bg, "c1", "shop", model.ComposeUpOptions{})
	if done := waitOp(t, m, op.ID); done.Status != model.OpSucceeded || len(eng.pulls) != 0 || ranVerb(runner, "up") == nil {
		t.Errorf("operation: %+v, pulls %v", done, eng.pulls)
	}
}
