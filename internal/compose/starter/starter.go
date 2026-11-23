// Package starter holds the text a new Compose file begins with.
package starter

import (
	"embed"
	"fmt"
	"strings"

	"docklet/internal/model"
)

//go:embed minimal.yaml
var minimal string

// withName puts the project name first, so the file gives the same project outside Docklet.
func withName(project, body string) string {
	return "name: " + project + "\n" + strings.ReplaceAll(body, "\r\n", "\n")
}

// Minimal is one example service with comments pointing at the usual next keys.
func Minimal(project string) string { return withName(project, minimal) }

//go:embed web-postgres.yaml web-postgres-redis.yaml static-site.yaml
var templateFiles embed.FS

var templates = []model.ComposeTemplate{
	{ID: "web-postgres", Title: "Web app + PostgreSQL"},
	{ID: "web-postgres-redis", Title: "Web app + PostgreSQL + Redis"},
	{ID: "static-site", Title: "Static site"},
}

// Templates lists the ready-made starting files, in the order they are offered.
func Templates() []model.ComposeTemplate { return append([]model.ComposeTemplate{}, templates...) }

// Template returns the text of one template for a project named project.
func Template(id, project string) (string, error) {
	for _, t := range templates {
		if t.ID == id {
			b, err := templateFiles.ReadFile(id + ".yaml")
			if err != nil {
				return "", err
			}
			return withName(project, string(b)), nil
		}
	}
	return "", fmt.Errorf("unknown template %q", id)
}
