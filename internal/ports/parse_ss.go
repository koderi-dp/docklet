package ports

import (
	"regexp"
	"strconv"
	"strings"
)

// usersRe matches one entry of ss's process column: ("name",pid=123,fd=4). Names may
// contain spaces, commas and escaped quotes.
var usersRe = regexp.MustCompile(`\("((?:[^"\\]|\\.)*)",pid=(\d+)`)

// splitHostPort splits "127.0.0.1:80", "[::1]:80", "*:80", "127.0.0.53%lo:53" and
// "::ffff:10.0.0.1:80" into address and port. ok is false for anything else.
func splitHostPort(s string) (ip string, port int, ok bool) {
	i := strings.LastIndexByte(s, ':')
	if i < 0 {
		return "", 0, false
	}
	p, err := strconv.Atoi(s[i+1:])
	if err != nil || p < 0 || p > 65535 {
		return "", 0, false
	}
	host := s[:i]
	if strings.HasPrefix(host, "[") {
		host = host[1:strings.IndexByte(host+"]", ']')] // "[fe80::1]%eth0"
	}
	if j := strings.IndexByte(host, '%'); j >= 0 {
		host = host[:j]
	}
	if host == "*" {
		host = "0.0.0.0"
	}
	return host, p, true
}

// ParseSS reads `ss -H -ltnup` output (one line per listening socket). Lines it does not
// understand are skipped. Only the first process of a shared socket is reported.
func ParseSS(text, location string) []Listener {
	var out []Listener
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		f := strings.Fields(line)
		if len(f) < 6 || (f[0] != "tcp" && f[0] != "udp") {
			continue // blank, header, or another family
		}
		if f[0] == "tcp" && f[1] != "LISTEN" {
			continue
		}
		ip, port, ok := splitHostPort(f[4])
		if !ok || port == 0 {
			continue
		}
		l := Listener{IP: ip, Port: port, Protocol: f[0], Location: location}
		if m := usersRe.FindStringSubmatch(line); m != nil {
			l.Name = strings.ReplaceAll(m[1], `\"`, `"`)
			l.PID, _ = strconv.Atoi(m[2])
		} else {
			l.Hidden = true // ss only shows processes the caller may inspect
		}
		out = append(out, l)
	}
	return out
}
