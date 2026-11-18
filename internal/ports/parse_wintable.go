package ports

import (
	"encoding/binary"
	"net/netip"
)

// Row layouts of the OWNER_PID tables returned by GetExtendedTcpTable and
// GetExtendedUdpTable: a 4-byte entry count followed by fixed-size rows. Ports are
// stored in network byte order in the low 16 bits of a 32-bit field.
const (
	tcp4RowSize = 24 // state, localAddr, localPort, remoteAddr, remotePort, pid
	tcp6RowSize = 56 // localAddr[16], scope, localPort, remoteAddr[16], scope, remotePort, state, pid
	udp4RowSize = 12 // localAddr, localPort, pid
	udp6RowSize = 28 // localAddr[16], scope, localPort, pid
)

// WinSocket is one row of a Windows owner-PID table.
type WinSocket struct {
	IP       string
	Port     int
	PID      int
	Protocol string
}

func winPort(v uint32) int { return int(v&0xff)<<8 | int(v>>8&0xff) }

// ParseWinTable decodes a table. tcp tables must be the LISTENER class (listening
// sockets only). Truncated data yields the rows that are complete.
func ParseWinTable(b []byte, proto string, v6 bool) []WinSocket {
	if len(b) < 4 {
		return nil
	}
	size, addrOff, portOff, pidOff := winLayout(proto, v6)
	n := int(binary.LittleEndian.Uint32(b))
	var out []WinSocket
	for i := 0; i < n && 4+(i+1)*size <= len(b); i++ {
		row := b[4+i*size : 4+(i+1)*size]
		var ip netip.Addr
		if v6 {
			ip = netip.AddrFrom16([16]byte(row[addrOff : addrOff+16]))
		} else {
			ip = netip.AddrFrom4([4]byte(row[addrOff : addrOff+4]))
		}
		out = append(out, WinSocket{
			IP: ip.String(), Protocol: proto,
			Port: winPort(binary.LittleEndian.Uint32(row[portOff:])),
			PID:  int(binary.LittleEndian.Uint32(row[pidOff:])),
		})
	}
	return out
}

// winLayout returns row size and the offsets of address, port and owning pid.
func winLayout(proto string, v6 bool) (size, addr, port, pid int) {
	switch {
	case proto == "tcp" && !v6:
		return tcp4RowSize, 4, 8, 20
	case proto == "tcp":
		return tcp6RowSize, 0, 20, 52
	case !v6:
		return udp4RowSize, 0, 4, 8
	}
	return udp6RowSize, 0, 20, 24
}
