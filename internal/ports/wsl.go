package ports

import (
	"bytes"
	"context"
	"fmt"
)

// procDumpScript prints the four /proc/net tables with "## name" markers. It is a fixed
// string: nothing user-controlled reaches the shell.
const procDumpScript = `for f in tcp tcp6 udp udp6; do echo "## $f"; cat /proc/net/$f; done`

// WSLHost reads the listeners inside one WSL distribution, as root so every program is
// visible: `ss -ltnup`, or /proc/net (without program names) when ss is not installed.
type WSLHost struct {
	Distro string
	run    runFunc
}

func NewWSLHost(distro string) HostSource { return WSLHost{Distro: distro, run: execRun} }

func (h WSLHost) Where() string { return "WSL (" + h.Distro + ")" }

func (h WSLHost) Listeners(ctx context.Context) ([]Listener, error) {
	base := []string{"-d", h.Distro, "-u", "root", "--exec"}
	b, err := h.run(ctx, "wsl.exe", append(base, "ss", "-H", "-ltnup")...)
	if err == nil {
		return ParseSS(wslText(b), h.Where()), nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	// ss is missing (or failed): the kernel tables still tell which ports are open.
	b, err2 := h.run(ctx, "wsl.exe", append(base, "sh", "-c", procDumpScript)...)
	if err2 != nil {
		return nil, fmt.Errorf("could not run ss or read /proc in the distribution: %w", err2)
	}
	ls := ParseProcDump(wslText(b), h.Where())
	for i := range ls {
		ls[i].Hidden = true // /proc/net does not name the program
	}
	return ls, nil
}

// wslText drops the NULs of UTF-16 text wsl.exe writes itself; output of Linux programs is UTF-8.
func wslText(b []byte) string {
	return string(bytes.ReplaceAll(b, []byte{0}, nil))
}
