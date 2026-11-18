package runspec

import (
	"strconv"
	"strings"
)

// Options controls rendering. Secret-looking environment values are replaced by a shell/Compose
// variable reference unless IncludeSecrets is set.
type Options struct {
	Shell          Shell
	IncludeSecrets bool
}

type runBuilder struct {
	sh    Shell
	lines []string
}

// flag adds "name value..." as one line; values are shell-quoted.
func (b *runBuilder) flag(name string, vals ...string) {
	parts := []string{name}
	for _, v := range vals {
		parts = append(parts, b.sh.Quote(v))
	}
	b.lines = append(b.lines, strings.Join(parts, " "))
}

func (b *runBuilder) flags(name string, vals []string) {
	for _, v := range vals {
		b.flag(name, v)
	}
}

func (b *runBuilder) raw(line string) { b.lines = append(b.lines, line) }

func (b *runBuilder) on(cond bool, flag string) {
	if cond {
		b.raw(flag)
	}
}

// ToDockerRun renders a multi-line `docker run` command (one flag per line, stable order),
// followed by `docker network connect` lines and a trailing "# not included" comment.
func ToDockerRun(s Spec, o Options) string {
	b := &runBuilder{sh: o.Shell}
	if s.Name != "" {
		b.flag("--name", s.Name)
	}
	b.on(s.AutoRemove, "--rm")
	switch {
	case s.Stdin && s.TTY:
		b.raw("-it")
	case s.Stdin:
		b.raw("-i")
	case s.TTY:
		b.raw("-t")
	}
	if s.Restart != "" {
		b.flag("--restart", s.Restart)
	}
	b.on(s.Init, "--init")
	b.on(s.Privileged, "--privileged")
	b.on(s.ReadOnly, "--read-only")
	identityFlags(b, s)
	networkFlags(b, s)
	for _, p := range s.Ports {
		b.flag("-p", portSpec(p))
	}
	b.flags("--expose", s.Expose)
	b.on(s.PublishAll, "-P")
	for _, m := range s.Mounts {
		if m.Type == "tmpfs" {
			b.flag("--tmpfs", mountSpec(m))
		} else {
			b.flag("-v", mountSpec(m))
		}
	}
	b.flags("--volumes-from", s.VolumesFrom)
	for _, e := range s.Env {
		if e.Secret && !o.IncludeSecrets {
			b.raw("-e " + b.sh.secretEnvArg(e.Key))
		} else {
			b.flag("-e", e.Key+"="+e.Value)
		}
	}
	for _, l := range s.Labels {
		b.flag("--label", l.Key+"="+l.Value)
	}
	resourceFlags(b, s)
	securityFlags(b, s)
	for _, kv := range s.Sysctls {
		b.flag("--sysctl", kv.Key+"="+kv.Value)
	}
	hostFlags(b, s)
	b.flags("--ulimit", s.Ulimits)
	healthFlags(b, s)
	if s.StopSignal != "" {
		b.flag("--stop-signal", s.StopSignal)
	}
	if s.StopTimeout != nil {
		b.flag("--stop-timeout", strconv.Itoa(*s.StopTimeout))
	}
	b.raw(b.sh.Quote(s.Image))

	var args []string
	if s.EntrypointSet && len(s.Entrypoint) > 1 {
		args = append(args, s.Entrypoint[1:]...)
	}
	args = append(args, s.Cmd...)
	if len(args) > 0 {
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = b.sh.Quote(a)
		}
		b.raw(strings.Join(quoted, " "))
	}

	var out strings.Builder
	out.WriteString("docker run -d")
	for _, l := range b.lines {
		out.WriteString(o.Shell.continuation() + "\n  " + l)
	}
	out.WriteString("\n")
	for _, n := range s.extraNetworks() {
		line := "docker network connect"
		for _, a := range n.Aliases {
			line += " --alias " + b.sh.Quote(a)
		}
		out.WriteString(line + " " + b.sh.Quote(n.Name) + " " + b.sh.Quote(s.Name) + "\n")
	}
	if len(s.NotIncluded) > 0 {
		out.WriteString("# not included: " + strings.Join(s.NotIncluded, "; ") + "\n")
	}
	return out.String()
}

func identityFlags(b *runBuilder, s Spec) {
	if s.User != "" {
		b.flag("--user", s.User)
	}
	if s.WorkDir != "" {
		b.flag("--workdir", s.WorkDir)
	}
	if s.Hostname != "" {
		b.flag("--hostname", s.Hostname)
	}
	if s.Domainname != "" {
		b.flag("--domainname", s.Domainname)
	}
	if s.EntrypointSet {
		first := ""
		if len(s.Entrypoint) > 0 {
			first = s.Entrypoint[0]
		}
		b.flag("--entrypoint", first)
	}
}

// primaryNetwork is the user network passed to --network (nil when the mode is not a network name).
func (s Spec) primaryNetwork() *NetAttach {
	switch {
	case s.NetworkMode == "", s.NetworkMode == "host", s.NetworkMode == "none", strings.HasPrefix(s.NetworkMode, "container:"):
		return nil
	}
	for i := range s.Networks {
		if s.Networks[i].Name == s.NetworkMode {
			return &s.Networks[i]
		}
	}
	return &NetAttach{Name: s.NetworkMode}
}

// extraNetworks are the networks beyond the --network one; `docker run` can only start on one.
func (s Spec) extraNetworks() []NetAttach {
	p := s.primaryNetwork()
	var out []NetAttach
	for _, n := range s.Networks {
		if p == nil || n.Name != p.Name {
			out = append(out, n)
		}
	}
	return out
}

func networkFlags(b *runBuilder, s Spec) {
	if s.NetworkMode != "" {
		b.flag("--network", s.NetworkMode)
	}
	if p := s.primaryNetwork(); p != nil {
		b.flags("--network-alias", p.Aliases)
	}
}

func resourceFlags(b *runBuilder, s Spec) {
	num := func(flag string, n int64, val string) {
		if n != 0 {
			b.flag(flag, val)
		}
	}
	num("--memory", s.MemoryBytes, formatBytes(s.MemoryBytes))
	num("--memory-reservation", s.MemoryReservation, formatBytes(s.MemoryReservation))
	num("--memory-swap", s.MemorySwap, formatSwap(s.MemorySwap))
	num("--cpus", s.NanoCPUs, formatCPUs(s.NanoCPUs))
	num("--cpu-shares", s.CPUShares, strconv.FormatInt(s.CPUShares, 10))
	num("--cpu-quota", s.CPUQuota, strconv.FormatInt(s.CPUQuota, 10))
	num("--cpu-period", s.CPUPeriod, strconv.FormatInt(s.CPUPeriod, 10))
	num("--pids-limit", s.PidsLimit, strconv.FormatInt(s.PidsLimit, 10))
	num("--shm-size", s.ShmSize, formatBytes(s.ShmSize))
	if s.CpusetCpus != "" {
		b.flag("--cpuset-cpus", s.CpusetCpus)
	}
	if s.GPUs != "" {
		b.flag("--gpus", s.GPUs)
	}
}

func formatSwap(n int64) string {
	if n < 0 {
		return "-1"
	}
	return formatBytes(n)
}

func securityFlags(b *runBuilder, s Spec) {
	b.flags("--cap-add", s.CapAdd)
	b.flags("--cap-drop", s.CapDrop)
	b.flags("--security-opt", s.SecurityOpt)
	b.flags("--group-add", s.GroupAdd)
	b.flags("--device", s.Devices)
	if s.PidMode != "" {
		b.flag("--pid", s.PidMode)
	}
	if s.IpcMode != "" {
		b.flag("--ipc", s.IpcMode)
	}
	if s.UTSMode != "" {
		b.flag("--uts", s.UTSMode)
	}
	if s.UsernsMode != "" {
		b.flag("--userns", s.UsernsMode)
	}
	if s.Runtime != "" {
		b.flag("--runtime", s.Runtime)
	}
	if s.OomScoreAdj != 0 {
		b.flag("--oom-score-adj", strconv.Itoa(s.OomScoreAdj))
	}
}

func hostFlags(b *runBuilder, s Spec) {
	b.flags("--add-host", s.ExtraHosts)
	b.flags("--dns", s.DNS)
	b.flags("--dns-search", s.DNSSearch)
	b.flags("--dns-option", s.DNSOptions)
	b.flags("--link", s.Links)
	if s.LogDriver != "" {
		b.flag("--log-driver", s.LogDriver)
	}
	for _, kv := range s.LogOpts {
		b.flag("--log-opt", kv.Key+"="+kv.Value)
	}
}

func healthFlags(b *runBuilder, s Spec) {
	h := s.Health
	if h == nil {
		return
	}
	if h.Disabled {
		b.raw("--no-healthcheck")
		return
	}
	cmd := h.Test[len(h.Test)-1]
	if h.Test[0] == "CMD" {
		cmd = joinPosix(h.Test[1:])
	}
	b.flag("--health-cmd", cmd)
	durs := []struct {
		flag string
		d    int64
		text string
	}{
		{"--health-interval", int64(h.Interval), formatDur(h.Interval)},
		{"--health-timeout", int64(h.Timeout), formatDur(h.Timeout)},
		{"--health-start-period", int64(h.StartPeriod), formatDur(h.StartPeriod)},
		{"--health-start-interval", int64(h.StartIv), formatDur(h.StartIv)},
	}
	for _, d := range durs {
		if d.d != 0 {
			b.flag(d.flag, d.text)
		}
	}
	if h.Retries != 0 {
		b.flag("--health-retries", strconv.Itoa(h.Retries))
	}
}
