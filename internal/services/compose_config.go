package services

import (
	"regexp"
	"strings"
)

var (
	// "  KEY: value" (mapping entry) and "  - KEY=value" (list entry).
	cfgMapRe  = regexp.MustCompile(`^(\s*)([^\s:=#"'\-][^\s:=#"']*)(:\s+)(\S.*)$`)
	cfgListRe = regexp.MustCompile(`^(\s*-\s+)([^\s:=#"']+)(=)(.*)$`)
	// scheme://user:password@host
	cfgURLRe = regexp.MustCompile(`(://[^/\s:@]+:)[^@\s/]+(@)`)
)

const maskedValue = "********"

// maskComposeConfig hides values of secret-like keys (same key patterns as container
// environment masking) in rendered `docker compose config` output.
func maskComposeConfig(text string) string {
	lines := strings.Split(text, "\n")
	for i, ln := range lines {
		if m := cfgListRe.FindStringSubmatch(ln); m != nil && isSecretKey(m[2]) && strings.TrimSpace(m[4]) != "" {
			lines[i] = m[1] + m[2] + m[3] + maskedValue
			continue
		}
		if m := cfgMapRe.FindStringSubmatch(ln); m != nil && isSecretKey(m[2]) {
			v := strings.TrimSpace(m[4])
			// Block scalars and nested structures carry no inline secret on this line.
			if v != "|" && v != ">" && v != "{}" && v != "[]" {
				lines[i] = m[1] + m[2] + m[3] + maskedValue
				continue
			}
		}
		lines[i] = cfgURLRe.ReplaceAllString(ln, "${1}"+maskedValue+"${2}")
	}
	return strings.Join(lines, "\n")
}
