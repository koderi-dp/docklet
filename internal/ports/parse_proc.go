package ports

import (
	"encoding/hex"
	"net/netip"
	"strconv"
	"strings"
)

// ProcSocket is one listening socket from /proc/net/{tcp,tcp6,udp,udp6}.
type ProcSocket struct {
	Listener
	UID   int
	Inode uint64
}

// ParseProcNet reads one /proc/net table. proto is "tcp" or "udp"; the address family
// follows from the width of the hex address. TCP keeps state 0A (LISTEN); UDP keeps
// unconnected sockets (state 07).
func ParseProcNet(text, proto, location string) []ProcSocket {
	var out []ProcSocket
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		f := strings.Fields(line)
		if i == 0 || len(f) < 10 || !strings.HasSuffix(f[0], ":") {
			continue // header, blank or marker lines
		}
		want := "0A"
		if proto == "udp" {
			want = "07"
		}
		if !strings.EqualFold(f[3], want) {
			continue
		}
		ip, port, ok := parseProcAddr(f[1])
		if !ok || port == 0 {
			continue
		}
		uid, _ := strconv.Atoi(f[7])
		inode, _ := strconv.ParseUint(f[9], 10, 64)
		out = append(out, ProcSocket{Listener: Listener{IP: ip, Port: port, Protocol: proto, Location: location, Hidden: true}, UID: uid, Inode: inode})
	}
	return out
}

// parseProcAddr decodes "0100007F:1F90" (IPv4) and the 32-hex-digit IPv6 form, where
// each 32-bit word is stored in host (little endian) order.
func parseProcAddr(s string) (string, int, bool) {
	a, p, found := strings.Cut(s, ":")
	if !found {
		return "", 0, false
	}
	port, err := strconv.ParseUint(p, 16, 16)
	if err != nil {
		return "", 0, false
	}
	raw, err := hex.DecodeString(a)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return "", 0, false
	}
	for i := 0; i+3 < len(raw); i += 4 {
		raw[i], raw[i+1], raw[i+2], raw[i+3] = raw[i+3], raw[i+2], raw[i+1], raw[i]
	}
	addr, _ := netip.AddrFromSlice(raw)
	return addr.String(), int(port), true
}

// ParseProcDump reads the output of the WSL fallback script: "## tcp", "## tcp6", ...
// markers each followed by the matching /proc/net table.
func ParseProcDump(text, location string) []Listener {
	var out []Listener
	sections := strings.Split(text, "## ")
	for _, sec := range sections[min(1, len(sections)):] {
		name, body, _ := strings.Cut(sec, "\n")
		proto := strings.TrimSuffix(strings.TrimSpace(name), "6")
		if proto != "tcp" && proto != "udp" {
			continue
		}
		for _, s := range ParseProcNet(body, proto, location) {
			out = append(out, s.Listener)
		}
	}
	return out
}
