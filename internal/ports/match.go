package ports

import (
	"net/netip"
	"strings"
)

// parseIP reads a bind address. wild is true for "", "*", 0.0.0.0 and ::, which cover
// every address (Docker treats the two wildcards as overlapping).
func parseIP(s string) (addr netip.Addr, wild, ok bool) {
	s = strings.Trim(strings.TrimSpace(s), "[]")
	if i := strings.IndexByte(s, '%'); i >= 0 {
		s = s[:i]
	}
	if s == "" || s == "*" {
		return netip.Addr{}, true, true
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false, false
	}
	a = a.Unmap()
	return a, a.IsUnspecified(), true
}

// overlap reports whether two bind addresses compete for the same port: a wildcard
// overlaps everything, two specific addresses only when equal. Unparseable text is
// compared literally.
func overlap(a, b string) bool {
	x, xw, xok := parseIP(a)
	y, yw, yok := parseIP(b)
	if !xok || !yok {
		return strings.EqualFold(a, b)
	}
	return xw || yw || x == y
}

// proxyNames are the programs Docker uses to publish ports on the host. A listener
// owned by one of them belongs to whichever container publishes that port.
var proxyNames = map[string]bool{
	"com.docker.backend": true, "com.docker.proxy": true, "com.docker.vpnkit": true,
	"vpnkit": true, "vpnkit-bridge": true, "wslrelay": true, "wslhost": true,
	"docker-proxy": true, "dockerd": true, "rootlesskit": true, "slirp4netns": true,
	"rootlesskit-docker-proxy": true,
}

func isProxy(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, ".exe")
	return proxyNames[n]
}

// anonymous is a listener whose owner could not be identified at all (no name, no PID),
// as ss shows for sockets held by Docker's own helpers in another namespace. When a
// running container publishes the same port, that container is the explanation.
func (l Listener) anonymous() bool { return l.Name == "" && l.PID == 0 }
