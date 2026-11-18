//go:build windows

package engine

import (
	"os"
	"strings"

	"docklet/internal/model"
)

// machinePipes lists the names under \\.\pipe\ (read-only; nothing is opened).
func machinePipes() []string {
	entries, err := os.ReadDir(`\\.\pipe\`)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, strings.TrimPrefix(e.Name(), `\\.\pipe\`))
	}
	return out
}

// podmanProfiles are the running Podman machines' named pipes.
func podmanProfiles() []model.ConnectionProfile { return podmanPipeProfiles(machinePipes()) }
