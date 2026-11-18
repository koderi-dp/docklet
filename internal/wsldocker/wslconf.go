package wsldocker

import "strings"

// EnableSystemd returns wsl.conf text with systemd=true in the [boot] section.
// Other sections, keys and comments are kept; applying it twice changes nothing more.
func EnableSystemd(conf string) string {
	conf = strings.ReplaceAll(conf, "\r\n", "\n")
	if strings.TrimSpace(conf) == "" {
		return "[boot]\nsystemd=true\n"
	}
	lines := strings.Split(strings.TrimRight(conf, "\n"), "\n")
	boot, next := -1, len(lines) // index of the [boot] header and of the section after it
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "[") {
			continue
		}
		if boot < 0 && strings.EqualFold(t, "[boot]") {
			boot = i
		} else if boot >= 0 && next == len(lines) {
			next = i
		}
	}
	if boot < 0 {
		return strings.Join(lines, "\n") + "\n\n[boot]\nsystemd=true\n"
	}
	at := boot + 1 // insert after the last non-blank line of [boot]
	for i := boot + 1; i < next; i++ {
		if k, _, ok := strings.Cut(lines[i], "="); ok && strings.EqualFold(strings.TrimSpace(k), "systemd") {
			lines[i] = "systemd=true"
			return strings.Join(lines, "\n") + "\n"
		}
		if strings.TrimSpace(lines[i]) != "" {
			at = i + 1
		}
	}
	out := append(append(append([]string{}, lines[:at]...), "systemd=true"), lines[at:]...)
	return strings.Join(out, "\n") + "\n"
}
