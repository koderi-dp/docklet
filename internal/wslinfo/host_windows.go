//go:build windows

package wslinfo

import (
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type winHost struct{}

// NewHostReader reads the process table and the registry; it never writes either.
func NewHostReader() HostReader { return winHost{} }

// VMMemory sums the working set of the WSL VM process. The process table is read
// with NtQuerySystemInformation, which needs no rights on the (protected) vmmem process.
func (winHost) VMMemory() (int64, bool) {
	buf := make([]byte, 1<<20)
	for range 6 {
		var need uint32
		err := windows.NtQuerySystemInformation(windows.SystemProcessInformation, unsafe.Pointer(&buf[0]), uint32(len(buf)), &need)
		if err == nil {
			return sumVM(buf)
		}
		if err != windows.STATUS_INFO_LENGTH_MISMATCH {
			return 0, false
		}
		buf = make([]byte, int(need)+(256<<10))
	}
	return 0, false
}

func sumVM(buf []byte) (int64, bool) {
	var wsl, other int64
	var foundWSL, foundOther bool
	off := uintptr(0)
	for {
		p := (*windows.SYSTEM_PROCESS_INFORMATION)(unsafe.Pointer(&buf[off]))
		switch strings.ToLower(p.ImageName.String()) {
		case "vmmemwsl":
			wsl, foundWSL = wsl+int64(p.WorkingSetSize), true
		case "vmmem":
			other, foundOther = other+int64(p.WorkingSetSize), true
		}
		if p.NextEntryOffset == 0 {
			break
		}
		off += uintptr(p.NextEntryOffset)
	}
	if foundWSL {
		return wsl, true
	}
	return other, foundOther
}

const lxssKey = `Software\Microsoft\Windows\CurrentVersion\Lxss`

// VHDX finds BasePath of the distribution (read-only registry access) and stats ext4.vhdx.
func (winHost) VHDX(distro string) (string, *int64, bool) {
	root, err := registry.OpenKey(registry.CURRENT_USER, lxssKey, registry.READ)
	if err != nil {
		return "", nil, false
	}
	defer root.Close()
	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return "", nil, false
	}
	for _, n := range names {
		k, err := registry.OpenKey(root, n, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		name, _, _ := k.GetStringValue("DistributionName")
		base, _, berr := k.GetStringValue("BasePath")
		k.Close()
		if berr != nil || !strings.EqualFold(name, distro) {
			continue
		}
		path := filepath.Join(strings.TrimPrefix(base, `\?\`), "ext4.vhdx")
		fi, err := os.Stat(path)
		if err != nil {
			return path, nil, true
		}
		size := fi.Size()
		return path, &size, true
	}
	return "", nil, false
}
