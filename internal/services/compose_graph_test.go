package services

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"docklet/internal/compose"
)

const graphConfigJSON = `{"services":{"web":{"image":"nginx","depends_on":{"db":{"condition":"service_started"}}},"db":{"image":"postgres"}}}`

func TestGraph_ReadsConfigWithAllProfiles(t *testing.T) {
	s, r, _ := newWatchSvc(t, "5.7.0")
	r.config = graphConfigJSON
	g, err := s.Graph(bg, "c1", "shop")
	if err != nil || g.Source != "config" || len(g.Services) != 2 {
		t.Fatalf("%+v %v", g, err)
	}
	args := composeArgs(t, &r.fakeRunner, "config")
	i := slices.Index(args, "--profile")
	if i < 0 || args[i+1] != "*" || !slices.Contains(args, "--format") || !slices.Contains(args, "json") {
		t.Errorf("argv: %v", args)
	}
}

func TestGraph_FallsBackToContainersWhenConfigFails(t *testing.T) {
	s, r, _ := newWatchSvc(t, "5.7.0")
	r.config = "not json at all"
	g, err := s.Graph(bg, "c1", "shop")
	if err != nil || g.Source != "containers" || !strings.HasPrefix(g.Note, "docker compose config failed:") {
		t.Fatalf("not JSON: %+v %v", g, err)
	}
}

func TestGraphOrFallback(t *testing.T) {
	cs := []compose.Container{{Name: "shop-web-1", Labels: map[string]string{compose.LabelService: "web"}}}

	g := graphOrFallback(graphConfigJSON, nil, true, "", cs)
	if g.Source != "config" || g.Note != "" {
		t.Errorf("valid config: %+v", g)
	}

	g = graphOrFallback("", errors.New("yaml: line 3: bad\nsecond line\nthird"), true, "", cs)
	if g.Source != "containers" || g.Note != "docker compose config failed: yaml: line 3: bad" {
		t.Errorf("runner error: %+v", g)
	}

	g = graphOrFallback("<html>", nil, true, "", cs)
	if g.Source != "containers" || !strings.HasPrefix(g.Note, "docker compose config failed:") || strings.Contains(g.Note, "\n") {
		t.Errorf("not JSON: %+v", g)
	}

	g = graphOrFallback(graphConfigJSON, nil, false, "no CLI here", cs)
	if g.Source != "containers" || g.Note != "no CLI here" {
		t.Errorf("cannot run: %+v", g)
	}
}
