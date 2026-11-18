package runspec

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Shell selects the quoting and line-continuation style of generated commands.
type Shell string

const (
	Bash       Shell = "bash" // POSIX shells: bash, zsh, sh
	PowerShell Shell = "powershell"
)

// ParseShell maps a UI value to a Shell; anything but "powershell" is bash.
func ParseShell(s string) Shell {
	if strings.EqualFold(s, string(PowerShell)) || strings.EqualFold(s, "pwsh") {
		return PowerShell
	}
	return Bash
}

var (
	posixSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)
	// PowerShell: no comma (array operator), no @ (splatting) and no % (alias) in bare words.
	pwshSafe  = regexp.MustCompile(`^[A-Za-z0-9_./:=-]+$`)
	envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Quote renders one argument so the shell passes it through unchanged.
func (sh Shell) Quote(s string) string {
	if sh == PowerShell {
		if s != "" && pwshSafe.MatchString(s) {
			return s
		}
		// Single quotes are literal in PowerShell; a quote (including the typographic ones PowerShell
		// also accepts) is escaped by doubling it.
		var b strings.Builder
		b.WriteByte('\'')
		for _, r := range s {
			b.WriteRune(r)
			if r == '\'' || r == '‘' || r == '’' || r == '‚' || r == '‛' {
				b.WriteRune(r)
			}
		}
		b.WriteByte('\'')
		return b.String()
	}
	if s != "" && posixSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (sh Shell) continuation() string {
	if sh == PowerShell {
		return " `"
	}
	return " \\"
}

// secretVarName turns an env key into a legal shell variable name.
func secretVarName(key string) string {
	n := regexp.MustCompile(`[^A-Za-z0-9_]`).ReplaceAllString(key, "_")
	if n == "" || (n[0] >= '0' && n[0] <= '9') {
		n = "_" + n
	}
	return n
}

// secretEnvArg is the `-e` value that reads a secret from a shell variable of the same name.
func (sh Shell) secretEnvArg(key string) string {
	v := secretVarName(key)
	if sh == PowerShell {
		if envNameRe.MatchString(key) {
			return `"` + key + `=$env:` + v + `"`
		}
		return "('" + strings.ReplaceAll(key, "'", "''") + "=' + $env:" + v + ")"
	}
	return sh.Quote(key) + `="$` + v + `"`
}

// joinPosix renders argv for a POSIX shell (used inside --health-cmd, which is always a shell string).
func joinPosix(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = Bash.Quote(a)
	}
	return strings.Join(parts, " ")
}

// formatBytes renders a size with the largest exact unit (512m, 2g), the way the Docker CLI reads it.
func formatBytes(n int64) string {
	const k, m, g = int64(1024), int64(1024 * 1024), int64(1024 * 1024 * 1024)
	switch {
	case n >= g && n%g == 0:
		return strconv.FormatInt(n/g, 10) + "g"
	case n >= m && n%m == 0:
		return strconv.FormatInt(n/m, 10) + "m"
	case n >= k && n%k == 0:
		return strconv.FormatInt(n/k, 10) + "k"
	}
	return strconv.FormatInt(n, 10)
}

func formatCPUs(nano int64) string {
	return strconv.FormatFloat(float64(nano)/1e9, 'f', -1, 64)
}

func formatDur(d time.Duration) string { return d.String() }

// portSpec renders the -p / compose short syntax: [ip:][hostPort:]containerPort[/proto].
func portSpec(p Port) string {
	var b strings.Builder
	if p.HostIP != "" {
		ip := p.HostIP
		if strings.Contains(ip, ":") {
			ip = "[" + ip + "]"
		}
		b.WriteString(ip + ":")
	}
	if p.HostIP != "" || p.HostPort != "" {
		b.WriteString(p.HostPort + ":")
	}
	b.WriteString(strconv.Itoa(p.ContainerPort))
	if p.Proto != "" && p.Proto != "tcp" {
		b.WriteString("/" + p.Proto)
	}
	return b.String()
}

func mountSpec(m Mount) string {
	switch m.Type {
	case "volume", "bind":
		if m.Source == "" {
			return m.Target
		}
		s := m.Source + ":" + m.Target
		if m.ReadOnly {
			s += ":ro"
		}
		return s
	}
	return fmt.Sprintf("%s:%s", m.Target, m.TmpfsOpts)
}
