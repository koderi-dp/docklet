package services

import (
	"testing"

	"github.com/moby/moby/api/types/container"
)

func TestMapSummary_CarriesImageID(t *testing.T) {
	got := mapSummary("c1", container.Summary{ID: "x", Names: []string{"/web-1"}, Image: "nginx:1.27", ImageID: "sha256:abc123"})
	if got.ImageID != "sha256:abc123" {
		t.Fatalf("ImageID = %q, want sha256:abc123", got.ImageID)
	}
}
