package services

import (
	"regexp"
	"strings"

	"docklet/internal/model"
)

var (
	aptInstall   = regexp.MustCompile(`\bapt(?:-get)?\s+(?:-\S+\s+)*install\b`)
	depInstall   = regexp.MustCompile(`\b(?:npm\s+(?:install|ci|i)|yarn(?:\s+install)?|pnpm\s+install|pip3?\s+install|poetry\s+install|bundle\s+install|composer\s+install|go\s+mod\s+download|cargo\s+(?:build|fetch)|mvn\s+(?:dependency|package|install))\b`)
	copyAllLocal = map[string]bool{".": true, "./": true}
)

// layerHints returns heuristic hints derived only from the instruction text of
// the steps (build order). They are pointers to review, never measurements.
func layerHints(steps []model.ImageLayerStep) []model.ImageLayerHint {
	hints := []model.ImageLayerHint{}
	for i, st := range steps {
		cmd := st.Command
		switch st.Kind {
		case "RUN":
			if aptInstall.MatchString(cmd) && !strings.Contains(cmd, "/var/lib/apt/lists") {
				hints = append(hints, model.ImageLayerHint{
					Step:   st.Index,
					Title:  "apt-get install without cleaning the package lists",
					Detail: "This step does not remove /var/lib/apt/lists in the same instruction, so the downloaded package index stays in this layer. Consider adding `rm -rf /var/lib/apt/lists/*` at the end of the same RUN.",
				})
			}
		case "COPY":
			if copiesWholeContext(cmd) && laterDependencyInstall(steps[i+1:]) {
				hints = append(hints, model.ImageLayerHint{
					Step:   st.Index,
					Title:  "Whole build context copied before a dependency install",
					Detail: "Copying everything first means any source change invalidates the cache of the later install step. Consider copying only the dependency manifest files first, installing, then copying the rest.",
				})
			}
		case "ADD":
			if f := strings.Fields(cmd); len(f) > 0 && (strings.HasPrefix(f[0], "http://") || strings.HasPrefix(f[0], "https://")) {
				hints = append(hints, model.ImageLayerHint{
					Step:   st.Index,
					Title:  "ADD of a remote URL",
					Detail: "Remote downloads with ADD are re-fetched when the cache is invalidated and are not verified by default. Consider a RUN with curl and a checksum check, or ADD --checksum.",
				})
			}
		}
	}
	return hints
}

// copiesWholeContext reports whether a COPY command copies "." (the entire context).
func copiesWholeContext(cmd string) bool {
	var args []string
	for _, f := range strings.Fields(cmd) {
		if !strings.HasPrefix(f, "--") {
			args = append(args, f)
		}
	}
	return len(args) >= 2 && copyAllLocal[args[0]]
}

func laterDependencyInstall(rest []model.ImageLayerStep) bool {
	for _, st := range rest {
		if st.Kind == "RUN" && depInstall.MatchString(st.Command) {
			return true
		}
	}
	return false
}
