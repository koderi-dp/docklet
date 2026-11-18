// Package wslinfo reads the state of a WSL distribution (read-only) and builds
// the commands of the maintenance actions. It never changes anything by itself.
package wslinfo

import (
	"strconv"
	"strings"
)

// Memory is what /proc/meminfo says, in bytes. A field is nil when it was not present.
type Memory struct {
	Total, Used, Cache *int64
}

// ParseMeminfo reads /proc/meminfo. Used is total minus available (what `free` shows
// as "used"); Cache is buffers + page cache + reclaimable slab (`buff/cache`).
func ParseMeminfo(text string) Memory {
	kb := map[string]int64{}
	for _, line := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			continue
		}
		n, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil || n < 0 {
			continue
		}
		if len(f) > 1 && strings.EqualFold(f[1], "kB") {
			n *= 1024
		}
		kb[strings.TrimSpace(k)] = n
	}
	var m Memory
	total, okT := kb["MemTotal"]
	if !okT || total <= 0 {
		return m
	}
	m.Total = &total
	if avail, ok := kb["MemAvailable"]; ok && avail <= total {
		used := total - avail
		m.Used = &used
	}
	buf, okB := kb["Buffers"]
	cached, okC := kb["Cached"]
	if okB && okC {
		c := buf + cached + kb["SReclaimable"]
		m.Cache = &c
	}
	return m
}

// Disk is the size and use of one filesystem in bytes.
type Disk struct {
	Size, Used *int64
}

// ParseDF reads the data row of `df -B1 -P /` (POSIX format: one line per filesystem).
func ParseDF(text string) Disk {
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		size, e1 := strconv.ParseInt(f[1], 10, 64)
		used, e2 := strconv.ParseInt(f[2], 10, 64)
		if e1 != nil || e2 != nil || size <= 0 || used < 0 || used > size {
			continue // header or garbage
		}
		return Disk{Size: &size, Used: &used}
	}
	return Disk{}
}

// SystemdEnabled reports whether wsl.conf sets systemd=true in its [boot] section.
// A missing or empty file means the default, which is off.
func SystemdEnabled(conf string) bool {
	in := false
	for _, line := range strings.Split(strings.ReplaceAll(conf, "\r", ""), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
			continue
		}
		if strings.HasPrefix(t, "[") {
			in = strings.EqualFold(strings.TrimSuffix(t, "]"), "[boot")
			continue
		}
		if !in {
			continue
		}
		if k, v, ok := strings.Cut(t, "="); ok && strings.EqualFold(strings.TrimSpace(k), "systemd") {
			v, _, _ = strings.Cut(v, "#")
			return strings.EqualFold(strings.Trim(strings.TrimSpace(v), `"'`), "true")
		}
	}
	return false
}
