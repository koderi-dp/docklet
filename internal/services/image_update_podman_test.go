package services

import (
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
)

func TestEndpointMissing_BareNotFoundOnly(t *testing.T) {
	bare := cerrdefs.ErrNotFound.WithMessage("Error response from daemon: Not Found")
	if !endpointMissing(bare) {
		t.Error("a bare 404 (route not implemented, as on Podman) must be recognised")
	}
	missing := cerrdefs.ErrNotFound.WithMessage("Error response from daemon: manifest unknown: manifest unknown")
	if endpointMissing(missing) {
		t.Error("a 404 that names the problem is a real answer, not a missing endpoint")
	}
	if endpointMissing(nil) {
		t.Error("nil is not an error")
	}
}

func TestImageError_ShortNameGetsTheQualifiedNameHint(t *testing.T) {
	err := imageError(errString("short-name resolution enforced but cannot prompt without a TTY"))
	if got := err.Error(); !strings.Contains(got, "docker.io/library/nginx") || !strings.Contains(got, "short-name resolution") {
		t.Fatalf("hint missing: %q", got)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
