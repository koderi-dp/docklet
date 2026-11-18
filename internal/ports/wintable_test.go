package ports

import (
	"encoding/binary"
	"testing"
)

func le(vals ...uint32) []byte {
	b := make([]byte, 0, len(vals)*4)
	for _, v := range vals {
		b = binary.LittleEndian.AppendUint32(b, v)
	}
	return b
}

// netPort is a port as Windows stores it: network byte order in the low 16 bits.
func netPort(p int) uint32 { return uint32(p>>8) | uint32(p&0xff)<<8 }

func TestParseWinTable_TCP4RowsGivePortAddressAndOwner(t *testing.T) {
	// dwNumEntries, then rows: state, localAddr, localPort, remoteAddr, remotePort, pid
	b := le(2,
		2, 0x0100007F, netPort(18080), 0, 0, 4321,
		2, 0, netPort(445), 0, 0, 4)
	got := ParseWinTable(b, "tcp", false)
	if len(got) != 2 || got[0] != (WinSocket{IP: "127.0.0.1", Port: 18080, PID: 4321, Protocol: "tcp"}) || got[1].IP != "0.0.0.0" || got[1].PID != 4 {
		t.Errorf("got %+v", got)
	}
}

func TestParseWinTable_TCP6AndUDPLayouts(t *testing.T) {
	v6 := le(1, 0, 0, 0, 0, 0, netPort(8080), 0, 0, 0, 0, 0, 0, 0, 77)
	v6[4+15] = 1 // ::1
	got := ParseWinTable(v6, "tcp", true)
	if len(got) != 1 || got[0].IP != "::1" || got[0].Port != 8080 || got[0].PID != 77 {
		t.Errorf("tcp6 = %+v", got)
	}
	udp := le(1, 0x0100007F, netPort(5353), 99)
	if got := ParseWinTable(udp, "udp", false); len(got) != 1 || got[0].Port != 5353 || got[0].PID != 99 {
		t.Errorf("udp4 = %+v", got)
	}
	udp6 := le(1, 0, 0, 0, 0, 0, netPort(5354), 98)
	if got := ParseWinTable(udp6, "udp", true); len(got) != 1 || got[0].Port != 5354 || got[0].PID != 98 {
		t.Errorf("udp6 = %+v", got)
	}
}

func TestParseWinTable_TruncatedDataKeepsOnlyCompleteRows(t *testing.T) {
	b := le(3, 2, 0, netPort(80), 0, 0, 1)
	if got := ParseWinTable(b, "tcp", false); len(got) != 1 {
		t.Errorf("got %+v", got)
	}
}
