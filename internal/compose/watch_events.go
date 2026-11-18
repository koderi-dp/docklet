package compose

import (
	"regexp"
	"strconv"
	"strings"
)

// WatchKind classifies one line of `docker compose watch` output.
type WatchKind string

const (
	WatchRaw        WatchKind = "raw" // not understood (build output, container status lines, ...)
	WatchEnabled    WatchKind = "enabled"
	WatchSync       WatchKind = "sync"       // "Syncing service ... after N changes were detected"
	WatchRebuilding WatchKind = "rebuilding" // "Rebuilding service(s) [...] after changes were detected..."
	WatchRebuilt    WatchKind = "rebuilt"    // "service(s) [...] successfully built"
	WatchRestarted  WatchKind = "restarted"  // "service(s) [...] restarted"
	WatchError      WatchKind = "error"
)

// WatchLine is the meaning of one CLI line. Services is empty for project-wide lines; Count is the
// number of changes of a sync line.
type WatchLine struct {
	Kind     WatchKind
	Services []string
	Count    int
}

var (
	watchEnabledRe    = regexp.MustCompile(`(?i)^watch enabled\b`)
	watchSyncRe       = regexp.MustCompile(`^Syncing service(?:\(s\))?\s+(.+?)\s+after\s+(\d+)\s+changes?\s+(?:were|was)\s+detected`)
	watchRebuildingRe = regexp.MustCompile(`^Rebuilding service(?:\(s\))?\s+(.+?)\s+after changes were detected`)
	watchBuiltRe      = regexp.MustCompile(`^service(?:\(s\))?\s+(.+?)\s+successfully built\b`)
	watchRestartedRe  = regexp.MustCompile(`^service(?:\(s\))?\s+(.+?)\s+restarted\b`)
	watchQuotedRe     = regexp.MustCompile(`"([^"]+)"`)
	// " Container web-1 Restarting": Compose status lines; only an Error status is an error.
	watchStatusRe = regexp.MustCompile(`^\s*(?:Container|Image|Network|Volume)\s+\S+\s+(\S.*?)\s*$`)
	// BuildKit plain progress ("#8 exporting layers"): only an ERROR step is an error.
	watchBuildRe = regexp.MustCompile(`^#\d+\s`)
	watchErrorRe = regexp.MustCompile(`(?i)\b(error|failed|cannot|unable to|denied|no such service|none of the selected services|load error)\b|^validating\s`)
)

// ParseWatchLine classifies a CLI line without ever failing: anything unknown is WatchRaw.
func ParseWatchLine(line string) WatchLine {
	l := strings.TrimSpace(line)
	if m := watchStatusRe.FindStringSubmatch(l); m != nil {
		if s := strings.ToLower(m[1]); strings.HasPrefix(s, "error") {
			return WatchLine{Kind: WatchError}
		}
		return WatchLine{Kind: WatchRaw}
	}
	if watchBuildRe.MatchString(l) {
		if strings.Contains(l, "ERROR") {
			return WatchLine{Kind: WatchError}
		}
		return WatchLine{Kind: WatchRaw}
	}
	switch {
	case watchEnabledRe.MatchString(l):
		return WatchLine{Kind: WatchEnabled}
	}
	if m := watchSyncRe.FindStringSubmatch(l); m != nil {
		n, _ := strconv.Atoi(m[2])
		return WatchLine{Kind: WatchSync, Services: quotedNames(m[1]), Count: n}
	}
	if m := watchRebuildingRe.FindStringSubmatch(l); m != nil {
		return WatchLine{Kind: WatchRebuilding, Services: quotedNames(m[1])}
	}
	if m := watchBuiltRe.FindStringSubmatch(l); m != nil {
		return WatchLine{Kind: WatchRebuilt, Services: quotedNames(m[1])}
	}
	if m := watchRestartedRe.FindStringSubmatch(l); m != nil {
		return WatchLine{Kind: WatchRestarted, Services: quotedNames(m[1])}
	}
	if watchErrorRe.MatchString(l) {
		return WatchLine{Kind: WatchError}
	}
	return WatchLine{Kind: WatchRaw}
}

func quotedNames(s string) []string {
	var out []string
	for _, m := range watchQuotedRe.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}
