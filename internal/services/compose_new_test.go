package services

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"docklet/internal/compose/starter"
	"docklet/internal/model"
)

const inspectWeb = `{"Id":"aaa111","Name":"/web","Image":"sha256:1","Config":{"Image":"nginx:1.27","Env":["MODE=prod"]},"HostConfig":{"NetworkMode":"bridge"},"NetworkSettings":{"Networks":{}}}`
const inspectDB = `{"Id":"bbb222","Name":"/db","Image":"sha256:2","Config":{"Image":"postgres:16","Env":["POSTGRES_PASSWORD=s3cret"]},"HostConfig":{"NetworkMode":"container:aaa111"},"NetworkSettings":{"Networks":{}}}`

// newFileSvc serves the two containers above; every other request is a 404. calls counts requests.
func newFileSvc(t *testing.T) (*ComposeService, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/aaa111/json"):
			_, _ = w.Write([]byte(inspectWeb))
		case strings.HasSuffix(r.URL.Path, "/containers/bbb222/json"):
			_, _ = w.Write([]byte(inspectDB))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"no such thing"}`))
		}
	}), "1.45")
	return &ComposeService{Engines: fakeProvider{cli}, runner: &fakeRunner{}, Profiles: fakeProfiles{"c1": localConn}}, &calls
}

func TestNewFilePreview_Minimal(t *testing.T) {
	s, calls := newFileSvc(t)
	p, err := s.NewFilePreview(context.Background(), "c1", "shop", model.ComposeNewSource{Kind: "minimal"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Text != starter.Minimal("shop") {
		t.Fatalf("got:\n%s", p.Text)
	}
	if p.SecretKeys == nil || p.NotIncluded == nil {
		t.Fatal("slices must not be nil")
	}
	if calls.Load() != 0 {
		t.Fatalf("the engine was called %d times", calls.Load())
	}
}

func TestNewFilePreview_Containers(t *testing.T) {
	s, _ := newFileSvc(t)
	src := model.ComposeNewSource{Kind: "containers", ContainerIDs: []string{"aaa111", "bbb222"}}
	p, err := s.NewFilePreview(context.Background(), "c1", "shop", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"name: shop\n", "  web:\n", "  db:\n", "image: nginx:1.27", "image: postgres:16", "POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}"} {
		if !strings.Contains(p.Text, want) {
			t.Errorf("missing %q in:\n%s", want, p.Text)
		}
	}
	if strings.Contains(p.Text, "s3cret") {
		t.Errorf("the secret value leaked:\n%s", p.Text)
	}
	if len(p.SecretKeys) != 1 || p.SecretKeys[0] != "db.POSTGRES_PASSWORD" {
		t.Errorf("secret keys: %v", p.SecretKeys)
	}
	// db shares web's network namespace by container id; in the file that is the service.
	if !strings.Contains(p.Text, "network_mode: service:web") {
		t.Errorf("container:<id> of a ticked container must become its service:\n%s", p.Text)
	}

	src.IncludeSecrets = true
	p, err = s.NewFilePreview(context.Background(), "c1", "shop", src)
	if err != nil || !strings.Contains(p.Text, "POSTGRES_PASSWORD: s3cret") {
		t.Errorf("included secrets must be literal: %v\n%s", err, p.Text)
	}
}

func TestNewFilePreview_InspectFailureNamesTheContainer(t *testing.T) {
	s, _ := newFileSvc(t)
	src := model.ComposeNewSource{Kind: "containers", ContainerIDs: []string{"aaa111", "gone999"}}
	p, err := s.NewFilePreview(context.Background(), "c1", "shop", src)
	if err == nil || !strings.Contains(err.Error(), "gone999") {
		t.Fatalf("want an error naming the container, got %v", err)
	}
	if p.Text != "" {
		t.Fatalf("no partial text may be returned:\n%s", p.Text)
	}
}

func TestNewFilePreview_RejectsBadInput(t *testing.T) {
	s, _ := newFileSvc(t)
	for _, tc := range []struct {
		name string
		src  model.ComposeNewSource
	}{
		{"Shop!", model.ComposeNewSource{Kind: "minimal"}},
		{"", model.ComposeNewSource{Kind: "minimal"}},
		{"shop", model.ComposeNewSource{}},
		{"shop", model.ComposeNewSource{Kind: "containers"}},
		{"shop", model.ComposeNewSource{Kind: "template", Template: "nope"}},
		{"shop", model.ComposeNewSource{Kind: "other"}},
	} {
		if _, err := s.NewFilePreview(context.Background(), "c1", tc.name, tc.src); err == nil {
			t.Errorf("name %q source %+v: want an error", tc.name, tc.src)
		}
	}
}

func TestNewFilePreview_Template(t *testing.T) {
	s, calls := newFileSvc(t)
	p, err := s.NewFilePreview(context.Background(), "c1", "shop", model.ComposeNewSource{Kind: "template", Template: "web-postgres"})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := starter.Template("web-postgres", "shop")
	if p.Text != want || want == "" {
		t.Fatalf("got: %s", p.Text)
	}
	if calls.Load() != 0 {
		t.Fatal("a template needs nothing from the engine")
	}
	if got := s.NewFileTemplates(); len(got) != 3 || got[0].ID != "web-postgres" || got[0].Title != "Web app + PostgreSQL" {
		t.Fatalf("templates: %+v", got)
	}
}
