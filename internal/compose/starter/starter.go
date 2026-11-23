// Package starter holds the text a new Compose file begins with.
package starter

import (
	_ "embed"
	"strings"
)

//go:embed minimal.yaml
var minimal string

// withName puts the project name first, so the file gives the same project outside Docklet.
func withName(project, body string) string {
	return "name: " + project + "\n" + strings.ReplaceAll(body, "\r\n", "\n")
}

// Minimal is one example service with comments pointing at the usual next keys.
func Minimal(project string) string { return withName(project, minimal) }
