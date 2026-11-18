package wslinfo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"docklet/internal/model"
	"docklet/internal/wsldocker"
)

// HostReader reads what only Windows knows. Both methods are read-only; ok=false means unknown.
type HostReader interface {
	// VMMemory is the working set of the vmmem/vmmemWSL process in bytes.
	VMMemory() (bytes int64, ok bool)
	// VHDX finds the distribution's ext4.vhdx through the registry and returns its path and size.
	// size is nil when the path is known but the file could not be read.
	VHDX(distro string) (path string, size *int64, ok bool)
}

// Collector gathers the state of one distribution.
type Collector struct {
	Exec wsldocker.Exec
	Host HostReader // nil leaves the Windows-side figures unknown
}

// inspectScript is constant: nothing from the user is interpolated into it.
// Docker is classified by looking for the daemon binary and process, so it also works without systemd.
const inspectScript = `
echo '@@os'; cat /etc/os-release 2>/dev/null
echo '@@init'; cat /proc/1/comm 2>/dev/null
echo '@@conf'; cat /etc/wsl.conf 2>/dev/null
echo '@@mem'; cat /proc/meminfo 2>/dev/null
echo '@@df'; df -B1 -P / 2>/dev/null
echo '@@docker'
if [ -x /usr/bin/dockerd ] || [ -x /usr/sbin/dockerd ] || command -v dockerd >/dev/null 2>&1; then
  up=
  for c in /proc/[0-9]*/comm; do
    if [ "$(cat "$c" 2>/dev/null)" = dockerd ]; then up=1; break; fi
  done
  if [ -n "$up" ]; then echo running; else echo stopped; fi
elif readlink -f "$(command -v docker 2>/dev/null)" 2>/dev/null | grep -q docker-desktop; then
  echo docker-desktop
else
  echo not-installed
fi
echo '@@end'
`

// Collect reads everything it can about d. Inside-distribution facts are read only
// when the distribution is already running: it is never booted just to be looked at.
func (c Collector) Collect(ctx context.Context, d model.WSLDistro, all []model.WSLDistro) model.WSLInfo {
	info := model.WSLInfo{
		Distro: d.Name, Running: IsRunning(d), Version: d.Version, IsDefault: d.Default,
		OtherRunning: OtherRunning(all, d.Name), Notes: []string{},
	}
	if c.Host != nil {
		if n, ok := c.Host.VMMemory(); ok && info.Running {
			info.VMMemory = &n
		}
		if p, size, ok := c.Host.VHDX(d.Name); ok {
			info.VHDXPath, info.VHDXSize = p, size
		} else {
			info.Notes = append(info.Notes, "The location of the virtual disk could not be read from the registry.")
		}
	}
	if !info.Running {
		info.Notes = append(info.Notes, "The distribution is stopped, so memory, disk and Docker are not read (that would start it).")
		info.Actions = Actions(info)
		return info
	}
	var lines []string
	err := c.Exec.Run(ctx, d.Name, wsldocker.Cmd{Argv: []string{"sh", "-c", inspectScript}}, func(l string) { lines = append(lines, l) })
	if err != nil {
		info.Notes = append(info.Notes, fmt.Sprintf("Could not read inside the distribution: %v", err))
		info.Actions = Actions(info)
		return info
	}
	applyInside(&info, lines)
	info.Actions = Actions(info)
	return info
}

func applyInside(info *model.WSLInfo, lines []string) {
	sec := map[string][]string{}
	cur := ""
	for _, l := range lines {
		if strings.HasPrefix(l, "@@") {
			cur = strings.TrimPrefix(l, "@@")
			continue
		}
		sec[cur] = append(sec[cur], l)
	}
	join := func(k string) string { return strings.Join(sec[k], "\n") }
	info.OSName = wsldocker.ParseOSRelease(join("os")).PrettyName
	if init := strings.TrimSpace(join("init")); init != "" {
		b := init == "systemd"
		info.SystemdRunning = &b
	}
	if _, seen := sec["conf"]; seen {
		b := SystemdEnabled(join("conf"))
		info.SystemdEnabled = &b
	}
	m := ParseMeminfo(join("mem"))
	info.MemTotal, info.MemUsed, info.MemCache = m.Total, m.Used, m.Cache
	d := ParseDF(join("df"))
	info.DiskSize, info.DiskUsed = d.Size, d.Used
	switch s := strings.TrimSpace(join("docker")); s {
	case model.WSLDockerRunning, model.WSLDockerStopped, model.WSLDockerNotInstalled, model.WSLDockerDesktopIntegrated:
		info.Docker = s
	}
}

// IsRunning says whether the `wsl -l -v` row is a running distribution.
func IsRunning(d model.WSLDistro) bool {
	return strings.EqualFold(strings.TrimSpace(d.State), "running")
}

// OtherRunning lists the running distributions other than name, sorted. A shutdown
// or a restart of WSL affects these too, so the confirmations name them.
func OtherRunning(all []model.WSLDistro, name string) []string {
	out := []string{}
	for _, d := range all {
		if IsRunning(d) && !strings.EqualFold(d.Name, name) {
			out = append(out, d.Name)
		}
	}
	sort.Strings(out)
	return out
}
