package compose

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A Compose file written for Docker Desktop on Windows mounts folders by their Windows path
// (C:/Source/app:/app). A Linux engine, such as the one inside a WSL distribution, rejects that.
// The helpers here find such mounts and turn the path into a variable that keeps the Windows
// path as its default, so the same file still works where it worked before and an env file can
// give the Linux path where it is needed.

// WindowsBind is a bind mount whose source is a path on a Windows drive.
type WindowsBind struct {
	Service string
	Source  string
	Target  string
}

// FindWindowsBinds reads `docker compose config --format json` and returns, sorted by service,
// the bind mounts whose source is a Windows drive path.
func FindWindowsBinds(jsonText string) ([]WindowsBind, error) {
	var raw struct {
		Services map[string]struct {
			Volumes []json.RawMessage `json:"volumes"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return nil, fmt.Errorf("parse compose config: %w", err)
	}
	names := make([]string, 0, len(raw.Services))
	for n := range raw.Services {
		names = append(names, n)
	}
	sort.Strings(names)
	out := []WindowsBind{}
	for _, n := range names {
		for _, v := range raw.Services[n].Volumes {
			var m struct {
				Type, Source, Target string
			}
			if json.Unmarshal(v, &m) == nil && m.Type == "bind" && winDriveRe.MatchString(m.Source) {
				out = append(out, WindowsBind{Service: n, Source: m.Source, Target: m.Target})
			}
		}
	}
	return out, nil
}

// LineEdit is one line of a file and what it becomes.
type LineEdit struct {
	Line     int // 1-based
	Old, New string
	Variable string
	// AlreadyVariable is true when the line already reads ${Variable:-<the path>}: nothing to
	// edit, the variable only needs a value.
	AlreadyVariable bool
}

var varDefaultPrefixRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*):-$`)

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// pathKey compares paths as written in YAML with paths as Compose reports them.
func winPathKey(p string) string {
	p = strings.ReplaceAll(p, `\\`, `\`)
	return strings.ToLower(strings.TrimRight(strings.ReplaceAll(p, `\`, "/"), "/"))
}

// winTokens returns the [start, end) ranges of the Windows drive paths written in a line. A path
// ends at the colon of the short volume syntax, a quote, the brace of a variable or a comment.
func winTokens(line string) [][2]int {
	var out [][2]int
	for i := 0; i+2 < len(line); i++ {
		c := line[i]
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !letter || line[i+1] != ':' || (line[i+2] != '\\' && line[i+2] != '/') || (i > 0 && isAlnum(line[i-1])) {
			continue
		}
		j := i + 2
		for j < len(line) && !strings.ContainsRune(`:"'}`, rune(line[j])) && !(line[j] == '#' && line[j-1] == ' ') {
			j++
		}
		for j > i && (line[j-1] == ' ' || line[j-1] == '\t' || line[j-1] == '\r') {
			j--
		}
		out = append(out, [2]int{i, j})
		i = j
	}
	return out
}

// PlanBindVariable finds, in the text of a Compose file, the line that mounts the Windows path
// source and returns it rewritten as ${NAME:-source}. A line that mounts it onto target is
// preferred when the path is written more than once. taken holds variable names in use.
func PlanBindVariable(text, source, target string, taken map[string]bool) (LineEdit, bool) {
	want := winPathKey(source)
	var found *LineEdit
	for n, raw := range strings.Split(text, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		for _, t := range winTokens(line) {
			token := line[t[0]:t[1]]
			if winPathKey(token) != want {
				continue
			}
			e := LineEdit{Line: n + 1, Old: line, New: line}
			if m := varDefaultPrefixRe.FindStringSubmatch(line[:t[0]]); m != nil && strings.HasPrefix(line[t[1]:], "}") {
				e.Variable, e.AlreadyVariable = m[1], true
			} else {
				e.Variable = BindVariableName(target, taken)
				e.New = line[:t[0]] + "${" + e.Variable + ":-" + token + "}" + line[t[1]:]
			}
			rest := strings.TrimPrefix(line[t[1]:], "}")
			if strings.HasPrefix(rest, ":"+target) {
				return e, true // exactly this mount
			}
			if found == nil {
				found = &e
			}
		}
	}
	if found == nil {
		return LineEdit{}, false
	}
	return *found, true
}

// ApplyLineEdit returns text with the edit's line replaced. The text is returned unchanged when
// that line is no longer what the edit was planned on.
func ApplyLineEdit(text string, e LineEdit) string {
	lines := strings.Split(text, "\n")
	if e.Line < 1 || e.Line > len(lines) {
		return text
	}
	cur := lines[e.Line-1]
	if strings.TrimSuffix(cur, "\r") != e.Old {
		return text
	}
	lines[e.Line-1] = e.New + cur[len(strings.TrimSuffix(cur, "\r")):]
	return strings.Join(lines, "\n")
}

var notVarCharRe = regexp.MustCompile(`[^A-Z0-9]+`)

// BindVariableName derives a variable name from the mount target: /studio becomes STUDIO_DIR.
func BindVariableName(target string, taken map[string]bool) string {
	parts := strings.Split(strings.Trim(strings.ReplaceAll(target, "\\", "/"), "/"), "/")
	base := strings.Trim(notVarCharRe.ReplaceAllString(strings.ToUpper(parts[len(parts)-1]), "_"), "_")
	switch {
	case base == "":
		base = "MOUNT"
	case base[0] >= '0' && base[0] <= '9':
		base = "MOUNT_" + base
	}
	name := base + "_DIR"
	for i := 2; taken[name]; i++ {
		name = fmt.Sprintf("%s_DIR_%d", base, i)
	}
	return name
}

// SetEnvVar returns the text of an env file with name set to value: its line is replaced when
// there is one, otherwise added at the end. Every other line is kept.
func SetEnvVar(text, name, value string) string {
	if strings.ContainsAny(value, " #\"'") {
		value = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
	}
	entry := name + "=" + value
	assign := regexp.MustCompile(`^\s*(?:export\s+)?` + regexp.QuoteMeta(name) + `\s*=`)
	var lines []string
	if strings.TrimSpace(text) != "" {
		lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	}
	done := false
	for i, l := range lines {
		if assign.MatchString(l) {
			lines[i], done = entry, true
		}
	}
	if !done {
		lines = append(lines, entry)
	}
	return strings.Join(lines, "\n") + "\n"
}
