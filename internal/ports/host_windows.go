//go:build windows

package ports

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iphlpapi          = windows.NewLazySystemDLL("iphlpapi.dll")
	procTCPTable      = iphlpapi.NewProc("GetExtendedTcpTable")
	procUDPTable      = iphlpapi.NewProc("GetExtendedUdpTable")
	errTableTruncated = errors.New("socket table changed while it was read")
)

const (
	tcpTableOwnerPIDListener = 3
	udpTableOwnerPID         = 1
	errInsufficientBuffer    = 122
)

// WindowsHost reads the listening sockets of this Windows machine through the IP
// helper API (no netstat parsing) and names their owners.
type WindowsHost struct{ Label string }

// NewLocalHost is the host source for an Engine running on this computer.
func NewLocalHost() HostSource { return WindowsHost{Label: "This machine"} }

func (h WindowsHost) Where() string { return h.Label }

func (h WindowsHost) Listeners(ctx context.Context) ([]Listener, error) {
	var socks []WinSocket
	for _, t := range []struct {
		proto string
		v6    bool
		af    uint32
	}{{"tcp", false, windows.AF_INET}, {"tcp", true, windows.AF_INET6}, {"udp", false, windows.AF_INET}, {"udp", true, windows.AF_INET6}} {
		b, err := readTable(t.proto, t.af)
		if err != nil {
			return nil, fmt.Errorf("reading the %s socket table: %w", t.proto, err)
		}
		socks = append(socks, ParseWinTable(b, t.proto, t.v6)...)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	names := processNames()
	info := map[int]procInfo{}
	out := make([]Listener, 0, len(socks))
	for _, s := range socks {
		pi, ok := info[s.PID]
		if !ok {
			pi = describeProcess(s.PID, names[s.PID])
			info[s.PID] = pi
		}
		out = append(out, Listener{IP: s.IP, Port: s.Port, Protocol: s.Protocol, PID: s.PID, Name: pi.name, Path: pi.path,
			User: pi.user, Location: h.Label, System: pi.system, Hidden: pi.hidden})
	}
	return out, nil
}

// readTable calls GetExtendedTcpTable / GetExtendedUdpTable, growing the buffer as needed.
func readTable(proto string, af uint32) ([]byte, error) {
	proc, class := procTCPTable, uintptr(tcpTableOwnerPIDListener)
	if proto == "udp" {
		proc, class = procUDPTable, udpTableOwnerPID
	}
	size := uint32(64 * 1024)
	for range 5 {
		buf := make([]byte, size)
		r, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, uintptr(af), class, 0)
		switch syscall.Errno(r) {
		case 0:
			return buf[:size], nil
		case errInsufficientBuffer:
			size += 16 * 1024 // the table may grow between the two calls
		default:
			return nil, syscall.Errno(r)
		}
	}
	return nil, errTableTruncated
}

type procInfo struct {
	name, path, user string
	system, hidden   bool
}

// processNames maps PID to executable name for every process (works without access to
// the process itself).
func processNames() map[int]string {
	out := map[int]string{}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return out
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out[int(e.ProcessID)] = windows.UTF16ToString(e.ExeFile[:])
	}
	return out
}

// describeProcess reads path and owner when the process may be opened. PID 0 and 4 are
// the system itself (HTTP.sys, IIS and similar); a process that cannot be opened keeps
// its name and is marked hidden rather than guessed at.
func describeProcess(pid int, name string) procInfo {
	pi := procInfo{name: name}
	switch pid {
	case 0:
		return procInfo{name: "System Idle Process", system: true}
	case 4:
		return procInfo{name: "System", system: true}
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		pi.hidden = true
		return pi
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 32768)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) == nil {
		pi.path = windows.UTF16ToString(buf[:n])
		if pi.name == "" {
			pi.name = filepath.Base(pi.path)
		}
	}
	pi.user = processUser(h)
	return pi
}

func processUser(h windows.Handle) string {
	var tok windows.Token
	if windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok) != nil {
		return ""
	}
	defer tok.Close()
	u, err := tok.GetTokenUser()
	if err != nil {
		return ""
	}
	account, domain, _, err := u.User.Sid.LookupAccount("")
	if err != nil {
		return ""
	}
	if domain != "" && !strings.EqualFold(domain, "NT AUTHORITY") {
		return domain + `\` + account
	}
	return account
}

// NewWindowsHost is the Windows side of a WSL engine: WSL forwards published ports to
// Windows localhost, so a Windows program on the same port breaks access from Windows.
func NewWindowsHost() HostSource { return WindowsHost{Label: "Windows"} }
