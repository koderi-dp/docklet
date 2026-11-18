package services

import (
	"strings"
	"testing"
)

func TestImageReferenceValidation_RejectsFlagsAndPathTricks(t *testing.T) {
	good := []string{"nginx", "nginx:1.27", "ghcr.io/org/app:v1", "localhost:5000/team/app:latest", "busybox@sha256:" + strings.Repeat("a", 64)}
	for _, g := range good {
		if _, err := validateRepoTag(g); err != nil {
			t.Errorf("%q should be valid: %v", g, err)
		}
	}
	bad := []string{"", "  ", "Nginx", "nginx:", "nginx:bad tag", "a b", "nginx;rm -rf /", "-v", "nginx?force=1", "sha256:" + strings.Repeat("a", 64)}
	for _, b := range bad {
		if _, err := validateRepoTag(b); err == nil {
			t.Errorf("%q should be invalid", b)
		}
	}
	if _, err := validateTarget("busybox@sha256:" + strings.Repeat("a", 64)); err == nil {
		t.Error("digest target must be rejected")
	}
	if _, err := validateTarget("docklet-test:latest"); err != nil {
		t.Error(err)
	}
	for _, id := range []string{"sha256:" + strings.Repeat("a", 64), "abcdef123456"} {
		if _, err := validateImageRef(id); err != nil {
			t.Errorf("image id %q: %v", id, err)
		}
	}
	if _, err := validateImageRef("x/../../etc?a=b"); err == nil {
		t.Error("path tricks must be rejected")
	}
}
