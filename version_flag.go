package main

import (
	"fmt"

	"docklet/internal/version"
)

// handleVersionFlag prints the version for `docklet --version` and reports whether it did,
// so main can exit before anything else (including Wails) starts.
func handleVersionFlag(args []string) bool {
	if len(args) != 1 || (args[0] != "--version" && args[0] != "-version") {
		return false
	}
	fmt.Fprintln(versionOutput(), "Docklet "+version.Version)
	return true
}
