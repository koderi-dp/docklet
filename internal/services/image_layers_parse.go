package services

import (
	"regexp"
	"strings"
)

var (
	nopPrefix      = regexp.MustCompile(`^/bin/sh -c #\(nop\)\s*`)
	shPrefix       = regexp.MustCompile(`^/bin/sh -c\s+`)
	buildArgPrefix = regexp.MustCompile(`^\|\d+\s+(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+)*`)
	buildkitSuffix = regexp.MustCompile(`\s*#\s*buildkit\s*$`)
	kinds          = map[string]bool{"RUN": true, "COPY": true, "ADD": true, "ENV": true, "CMD": true}
	otherKeywords  = map[string]bool{
		"FROM": true, "ENTRYPOINT": true, "LABEL": true, "EXPOSE": true, "WORKDIR": true, "USER": true, "ARG": true,
		"VOLUME": true, "SHELL": true, "STOPSIGNAL": true, "HEALTHCHECK": true, "ONBUILD": true, "MAINTAINER": true,
	}
)

// parseCreatedBy turns a history CreatedBy string into a classified, cleaned
// instruction. Handles the classic builder ("/bin/sh -c #(nop)  CMD ..." and
// "/bin/sh -c <cmd>") and BuildKit ("RUN /bin/sh -c <cmd> # buildkit") forms.
// The command is the instruction without its keyword for RUN/COPY/ADD/ENV/CMD.
func parseCreatedBy(createdBy string) (kind, instruction, command string) {
	s := strings.TrimSpace(createdBy)
	if s == "" {
		return "OTHER", "", ""
	}
	s = strings.TrimSpace(buildkitSuffix.ReplaceAllString(s, ""))
	switch {
	case nopPrefix.MatchString(s):
		s = strings.TrimSpace(nopPrefix.ReplaceAllString(s, ""))
	case shPrefix.MatchString(s):
		s = "RUN " + buildArgStrip(strings.TrimSpace(shPrefix.ReplaceAllString(s, "")))
	case buildArgPrefix.MatchString(s) && shPrefix.MatchString(buildArgStrip(s)):
		// Classic builder RUN with build args: "|2 A=b C=d /bin/sh -c cmd".
		s = "RUN " + strings.TrimSpace(shPrefix.ReplaceAllString(buildArgStrip(s), ""))
	default:
		// BuildKit form: keyword first.
		kw, rest := splitKeyword(s)
		if kw == "RUN" {
			rest = buildArgStrip(rest)
			rest = strings.TrimSpace(shPrefix.ReplaceAllString(rest, ""))
			s = "RUN " + rest
		}
	}
	kw, rest := splitKeyword(s)
	switch {
	case kinds[kw]:
		return kw, kw + " " + rest, rest
	case otherKeywords[kw]:
		return "OTHER", s, s
	default:
		// Unknown producer text (e.g. "BusyBox 1.36 ..."): show as is.
		return "OTHER", s, s
	}
}

func buildArgStrip(s string) string {
	return strings.TrimSpace(buildArgPrefix.ReplaceAllString(s, ""))
}

// splitKeyword returns the upper-case first word and the rest, or ("", s) when
// the first word is not a Dockerfile-like keyword.
func splitKeyword(s string) (string, string) {
	word, rest, _ := strings.Cut(s, " ")
	up := strings.ToUpper(word)
	if word == up && (kinds[up] || otherKeywords[up]) {
		return up, strings.TrimSpace(rest)
	}
	return "", s
}
