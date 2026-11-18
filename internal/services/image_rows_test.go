package services

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"

	"docklet/internal/model"
)

func TestBuildImageRows_CountsContainersPerTagAndSortsDanglingLast(t *testing.T) {
	imgs := []image.Summary{
		{ID: "sha256:aaa", Created: 10, Size: 100, RepoTags: []string{"nginx:1.27", "nginx:latest"}},
		{ID: "sha256:bbb", Created: 20, Size: 50, RepoTags: []string{"redis:7"}},
		{ID: "sha256:ccc", Created: 30, Size: 70, RepoTags: []string{"<none>:<none>"}},
		{ID: "sha256:ddd", Created: 40, Size: 10, RepoDigests: []string{"x@sha256:1"}},
		{ID: "sha256:eee", Created: 5, Size: 10, RepoTags: []string{"mcr.io/pw@sha256:abc"}},
	}
	cs := []container.Summary{
		{ID: "c1", Names: []string{"/web"}, ImageID: "sha256:aaa", State: "running"},
		{ID: "c2", Names: []string{"/old"}, ImageID: "sha256:aaa", State: "exited"},
		{ID: "c3", Names: []string{"/other"}, ImageID: "sha256:zzz", State: "running"},
	}
	rows := buildImageRows("c1", imgs, containersByImage(cs))
	if len(rows) != 6 { // 2 nginx tags + redis + 3 dangling (untagged, none, digest-only)
		t.Fatalf("want 5 rows, got %d: %+v", len(rows), rows)
	}
	by := map[string]model.ImageRow{}
	for _, r := range rows {
		by[r.Key] = r
	}
	n := by["sha256:aaa|nginx:latest"]
	if !n.InUse || n.ContainerCount != 2 || n.Repository != "nginx" || n.Tag != "latest" {
		t.Errorf("nginx row: %+v", n)
	}
	if by["sha256:aaa|nginx:1.27"].ContainerCount != 2 {
		t.Error("tag rows of one image share the count")
	}
	if r := by["sha256:bbb|redis:7"]; r.InUse || r.ContainerCount != 0 {
		t.Errorf("redis must be unused: %+v", r)
	}
	for _, id := range []string{"sha256:ccc|", "sha256:ddd|", "sha256:eee|mcr.io/pw@sha256:abc"} {
		if r := by[id]; !r.Dangling || r.Ref == "" || r.InUse {
			t.Errorf("dangling row %s: %+v", id, r)
		}
	}
	if !rows[len(rows)-1].Dangling {
		t.Error("dangling rows sort last")
	}
	if r, tg := splitRepoTag("localhost:5000/app"); r != "localhost:5000/app" || tg != "" {
		t.Errorf("registry port is not a tag: %q %q", r, tg)
	}
}
