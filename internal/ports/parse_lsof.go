package ports

import (
	"regexp"
	"strconv"
	"strings"
)

// lsofRe reads one `lsof -nP -iTCP -sTCP:LISTEN` / `-iUDP` line from the right, so a
// command name with spaces still parses:
//
//	COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME
var lsofRe = regexp.MustCompile(`^(.+?)\s+(\d+)\s+(\S+)\s+\S+\s+(?:IPv4|IPv6)\s+\S+\s+\S+\s+(TCP|UDP)\s+(\S+)(?:\s+\((\w+)\))?\s*$`)

// lsofName undoes lsof's \xNN escapes in command names ("Google\x20Chrome").
var lsofEscRe = regexp.MustCompile(`\\x([0-9a-fA-F]{2})`)

func lsofName(s string) string {
	return lsofEscRe.ReplaceAllStringFunc(s, func(m string) string {
		n, _ := strconv.ParseUint(m[2:], 16, 8)
		return string([]byte{byte(n)})
	})
}

// ParseLsof reads lsof output. Established or connected sockets (those with "->") and
// non-listening TCP states are skipped.
func ParseLsof(text, location string) []Listener {
	var out []Listener
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		m := lsofRe.FindStringSubmatch(line)
		if m == nil || strings.Contains(m[5], "->") {
			continue
		}
		proto := strings.ToLower(m[4])
		if proto == "tcp" && m[6] != "LISTEN" {
			continue
		}
		ip, port, ok := splitHostPort(m[5])
		if !ok || port == 0 {
			continue
		}
		pid, _ := strconv.Atoi(m[2])
		out = append(out, Listener{IP: ip, Port: port, Protocol: proto, PID: pid, Name: lsofName(m[1]), User: m[3], Location: location})
	}
	return out
}
