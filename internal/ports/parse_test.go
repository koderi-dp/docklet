package ports

import "testing"

const ssSample = `tcp   LISTEN 0      4096   127.0.0.53%lo:53         0.0.0.0:*    users:(("systemd-resolve",pid=215,fd=14))
udp   UNCONN 0      0      127.0.0.53%lo:53         0.0.0.0:*    users:(("systemd-resolve",pid=215,fd=13))
tcp   LISTEN 0      4096         0.0.0.0:3001       0.0.0.0:*    users:(("docker-proxy",pid=1234,fd=7))
tcp   LISTEN 0      4096            [::]:3001          [::]:*    users:(("docker-proxy",pid=1240,fd=7))
tcp   LISTEN 0      511                *:80                *:*    users:(("nginx",pid=900,fd=6),("nginx",pid=901,fd=6))
tcp   LISTEN 0      128            [::1]:631            [::]:*
tcp   LISTEN 0      5      0.0.0.0:9000  0.0.0.0:*  users:(("my \"odd\" app,v2",pid=4242,fd=3))
udp   UNCONN 0      0      [fe80::1]%eth0:546       [::]:*    users:(("systemd-network",pid=300,fd=18))
tcp   ESTAB  0      0      10.0.0.2:22      10.0.0.9:51000
`

func TestParseSS_ReadsListenersWithProcessesAndOddNames(t *testing.T) {
	got := ParseSS(ssSample, "WSL (Ubuntu)")
	want := []struct {
		ip    string
		port  int
		proto string
		pid   int
		name  string
	}{
		{"127.0.0.53", 53, "tcp", 215, "systemd-resolve"},
		{"127.0.0.53", 53, "udp", 215, "systemd-resolve"},
		{"0.0.0.0", 3001, "tcp", 1234, "docker-proxy"},
		{"::", 3001, "tcp", 1240, "docker-proxy"},
		{"0.0.0.0", 80, "tcp", 900, "nginx"},
		{"::1", 631, "tcp", 0, ""},
		{"0.0.0.0", 9000, "tcp", 4242, `my "odd" app,v2`},
		{"fe80::1", 546, "udp", 300, "systemd-network"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d listeners, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.IP != w.ip || g.Port != w.port || g.Protocol != w.proto || g.PID != w.pid || g.Name != w.name || g.Location != "WSL (Ubuntu)" {
			t.Errorf("row %d = %+v, want %+v", i, g, w)
		}
	}
	if !got[5].Hidden || got[0].Hidden {
		t.Error("a socket without a process column must be marked hidden")
	}
}

const procTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0
   1: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 22222 1 0000000000000000 100 0 0 10 0
   2: 0100007F:C350 0100007F:1F90 01 00000000:00000000 00:00000000 00000000  1000        0 33333 1 0000000000000000 100 0 0 10 0
`

const procTCP6 = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:0BB8 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 44444 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000001000000:0277 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000   108        0 55555 1 0000000000000000 100 0 0 10 0
   2: 0000000000000000FFFF00000100007F:1F91 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 66666 1 0000000000000000 100 0 0 10 0
`

func TestParseProcNet_DecodesAddressesAndKeepsOnlyListeningSockets(t *testing.T) {
	v4 := ParseProcNet(procTCP, "tcp", "x")
	if len(v4) != 2 {
		t.Fatalf("ipv4 rows = %+v", v4)
	}
	if v4[0].IP != "127.0.0.1" || v4[0].Port != 8080 || v4[0].UID != 1000 || v4[0].Inode != 12345 {
		t.Errorf("row 0 = %+v", v4[0])
	}
	if v4[1].IP != "0.0.0.0" || v4[1].Port != 22 {
		t.Errorf("row 1 = %+v", v4[1])
	}
	v6 := ParseProcNet(procTCP6, "tcp", "x")
	if len(v6) != 3 {
		t.Fatalf("ipv6 rows = %+v", v6)
	}
	if v6[0].IP != "::" || v6[0].Port != 3000 || v6[1].IP != "::1" || v6[1].Port != 631 {
		t.Errorf("rows = %+v %+v", v6[0], v6[1])
	}
	if !overlap(v6[2].IP, "127.0.0.1") || v6[2].Port != 8081 {
		t.Errorf("mapped address = %+v", v6[2])
	}
}

func TestParseProcNet_UDPKeepsUnconnectedSockets(t *testing.T) {
	udp := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops
  10: 00000000:0035 00000000:0000 07 00000000:00000000 00:00000000 00000000   102        0 777 2 0000000000000000 0
  11: 0100007F:0044 0100007F:0035 01 00000000:00000000 00:00000000 00000000   102        0 778 2 0000000000000000 0
`
	got := ParseProcNet(udp, "udp", "x")
	if len(got) != 1 || got[0].Port != 53 || got[0].Protocol != "udp" {
		t.Errorf("got %+v", got)
	}
}

func TestParseProcDump_ReadsTheMarkedSectionsOfTheWSLFallback(t *testing.T) {
	text := "## tcp\n" + procTCP + "## tcp6\n" + procTCP6 + "## udp\n  sl local_address\n## udp6\n"
	got := ParseProcDump(text, "WSL (Ubuntu)")
	if len(got) != 5 {
		t.Fatalf("got %d listeners: %+v", len(got), got)
	}
}

const lsofSample = `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node      12345  alice   23u  IPv4 0xabcdef0123456789      0t0  TCP 127.0.0.1:3000 (LISTEN)
rapportd    410  alice    8u  IPv6 0x1234567890abcdef      0t0  TCP *:49152 (LISTEN)
Google\x20Chrome  612 alice  30u  IPv4 0xaaaa 0t0 TCP [::1]:9222 (LISTEN)
Code Helper  777 alice 22u IPv4 0xbbbb 0t0 TCP 127.0.0.1:5037 (LISTEN)
postgres  901  _postgres  7u  IPv6 0xcccc 0t0 TCP [::1]:5432 (LISTEN)
curl      999  alice    5u  IPv4 0xdddd 0t0 TCP 10.0.0.2:51000->93.184.216.34:443 (ESTABLISHED)
mDNSRespo 300  _mdnsresponder 9u IPv4 0xeeee 0t0 UDP *:5353
`

func TestParseLsof_ReadsListenersAndSkipsConnectedSockets(t *testing.T) {
	got := ParseLsof(lsofSample, "This machine")
	want := []struct {
		name  string
		port  int
		proto string
		ip    string
		user  string
	}{
		{"node", 3000, "tcp", "127.0.0.1", "alice"},
		{"rapportd", 49152, "tcp", "0.0.0.0", "alice"},
		{"Google Chrome", 9222, "tcp", "::1", "alice"},
		{"Code Helper", 5037, "tcp", "127.0.0.1", "alice"},
		{"postgres", 5432, "tcp", "::1", "_postgres"},
		{"mDNSRespo", 5353, "udp", "0.0.0.0", "_mdnsresponder"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d: %+v", len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Name != w.name || g.Port != w.port || g.Protocol != w.proto || g.IP != w.ip || g.User != w.user {
			t.Errorf("row %d = %+v, want %+v", i, g, w)
		}
	}
}
