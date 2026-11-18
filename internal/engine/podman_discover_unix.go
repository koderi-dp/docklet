//go:build !windows

package engine

import (
	"os"
	"path/filepath"
	"runtime"

	"docklet/internal/model"
)

func machinePipes() []string { return nil }

// podmanProfiles are the Podman API sockets that exist: rootless and rootful on Linux, the
// machine sockets on macOS.
func podmanProfiles() []model.ConnectionProfile {
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		return podmanMacCandidates(home, os.TempDir(), func(p string) []string { m, _ := filepath.Glob(p); return m })
	}
	return podmanLinuxCandidates(os.Getenv("XDG_RUNTIME_DIR"), os.Getuid(), isSocket)
}
