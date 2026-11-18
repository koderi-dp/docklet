package runspec

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	composeNameBad = regexp.MustCompile(`[^A-Za-z0-9._-]`)
	windowsPathRe  = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
)

// escDollar protects literal dollars from Compose interpolation.
func escDollar(s string) string { return strings.ReplaceAll(s, "$", "$$") }

func scalarNode(v any) *yaml.Node {
	n := &yaml.Node{}
	_ = n.Encode(v)
	return n
}

func mapNode() *yaml.Node { return &yaml.Node{Kind: yaml.MappingNode} }

func put(m *yaml.Node, key string, v any) {
	val, ok := v.(*yaml.Node)
	if !ok {
		val = scalarNode(v)
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, val)
}

func flowList(items []string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, s := range items {
		n.Content = append(n.Content, scalarNode(escDollar(s)))
	}
	return n
}

func escAll(items []string) []string {
	out := make([]string, len(items))
	for i, s := range items {
		out[i] = escDollar(s)
	}
	return out
}

func kvMap(kv []KV) *yaml.Node {
	m := mapNode()
	for _, e := range kv {
		put(m, e.Key, escDollar(e.Value))
	}
	return m
}

// composeNetKey returns the key a network gets in the snippet and whether it is external.
func (s Spec) composeNetKey(name string) (key string, external bool) {
	if s.ComposeProject != "" {
		if suffix, ok := strings.CutPrefix(name, s.ComposeProject+"_"); ok && suffix != "" {
			return suffix, false
		}
		if suffix, ok := strings.CutPrefix(name, s.ComposeProject+"-"); ok && suffix != "" {
			return suffix, false
		}
	}
	return name, true
}

func (s Spec) serviceName() string {
	n := s.ComposeService
	if n == "" {
		n = composeNameBad.ReplaceAllString(s.Name, "-")
	}
	if n == "" {
		n = "app"
	}
	return n
}

// ToCompose renders a `services:` snippet (plus the top-level volumes and networks it needs).
func ToCompose(s Spec, o Options) string {
	svc := mapNode()
	put(svc, "image", s.Image)
	if s.Name != "" && s.ComposeProject == "" {
		put(svc, "container_name", s.Name)
	}
	for _, kv := range [][2]string{{"hostname", s.Hostname}, {"domainname", s.Domainname}, {"user", s.User}, {"working_dir", s.WorkDir}} {
		if kv[1] != "" {
			put(svc, kv[0], escDollar(kv[1]))
		}
	}
	if s.EntrypointSet {
		put(svc, "entrypoint", flowList(s.Entrypoint)) // [] clears the image entrypoint
	}
	if len(s.Cmd) > 0 {
		put(svc, "command", flowList(s.Cmd))
	}
	if len(s.Env) > 0 {
		env := mapNode()
		for _, e := range s.Env {
			if e.Secret && !o.IncludeSecrets {
				put(env, e.Key, "${"+secretVarName(e.Key)+"}")
			} else {
				put(env, e.Key, escDollar(e.Value))
			}
		}
		put(svc, "environment", env)
	}
	if len(s.Labels) > 0 {
		put(svc, "labels", kvMap(s.Labels))
	}
	if len(s.Ports) > 0 {
		var ports []string
		for _, p := range s.Ports {
			ports = append(ports, portSpec(p))
		}
		put(svc, "ports", ports)
	}
	if len(s.Expose) > 0 {
		put(svc, "expose", s.Expose)
	}
	vols, topVolumes := composeVolumes(s)
	if len(vols.Content) > 0 {
		put(svc, "volumes", vols)
	}
	var tmpfs []string
	for _, m := range s.Mounts {
		if m.Type == "tmpfs" {
			tmpfs = append(tmpfs, strings.TrimSuffix(mountSpec(m), ":"))
		}
	}
	if len(tmpfs) > 0 {
		put(svc, "tmpfs", escAll(tmpfs))
	}
	topNetworks := composeNetworks(svc, s)
	composeSettings(svc, s)

	root := mapNode()
	services := mapNode()
	put(services, s.serviceName(), svc)
	put(root, "services", services)
	if topVolumes != nil {
		put(root, "volumes", topVolumes)
	}
	if topNetworks != nil {
		put(root, "networks", topNetworks)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(root)
	_ = enc.Close()
	if notes := composeNotIncluded(s); len(notes) > 0 {
		buf.WriteString("# not included: " + strings.Join(notes, "; ") + "\n")
	}
	return buf.String()
}

func composeNotIncluded(s Spec) []string {
	out := append([]string{}, s.NotIncluded...)
	if s.AutoRemove {
		out = append(out, "auto-remove (--rm)")
	}
	if s.PublishAll {
		out = append(out, "publish all exposed ports (-P)")
	}
	return out
}

func composeVolumes(s Spec) (*yaml.Node, *yaml.Node) {
	list := &yaml.Node{Kind: yaml.SequenceNode}
	var top *yaml.Node
	for _, m := range s.Mounts {
		switch {
		case m.Type == "tmpfs":
			continue
		case m.Type == "bind" && windowsPathRe.MatchString(m.Source):
			long := mapNode()
			put(long, "type", "bind")
			put(long, "source", escDollar(m.Source))
			put(long, "target", m.Target)
			if m.ReadOnly {
				put(long, "read_only", true)
			}
			list.Content = append(list.Content, long)
		case m.Type == "volume" && m.Source != "":
			key, named := m.Source, true
			if suffix, ok := strings.CutPrefix(m.Source, s.ComposeProject+"_"); ok && s.ComposeProject != "" && suffix != "" {
				key, named = suffix, false
			}
			if top == nil {
				top = mapNode()
			}
			if !hasKey(top, key) {
				def := mapNode()
				if named {
					put(def, "name", m.Source) // reuse the existing volume instead of a project-prefixed one
				}
				put(top, key, def)
			}
			list.Content = append(list.Content, scalarNode(escDollar(mountSpec(Mount{Type: "volume", Source: key, Target: m.Target, ReadOnly: m.ReadOnly}))))
		default:
			list.Content = append(list.Content, scalarNode(escDollar(mountSpec(m))))
		}
	}
	return list, top
}

func hasKey(m *yaml.Node, key string) bool {
	for i := 0; i < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return true
		}
	}
	return false
}

func composeNetworks(svc *yaml.Node, s Spec) *yaml.Node {
	switch {
	case s.NetworkMode == "host" || s.NetworkMode == "none" || strings.HasPrefix(s.NetworkMode, "container:"):
		put(svc, "network_mode", s.NetworkMode)
		return nil
	case len(s.Networks) == 0:
		return nil
	}
	if len(s.Networks) == 1 && len(s.Networks[0].Aliases) == 0 {
		if key, external := s.composeNetKey(s.Networks[0].Name); key == "default" && !external {
			return nil // the implicit project network: nothing to write
		}
	}
	var top *yaml.Node
	hasAlias := false
	for _, n := range s.Networks {
		hasAlias = hasAlias || len(n.Aliases) > 0
	}
	var names []string
	full := mapNode()
	for _, n := range s.Networks {
		key, external := s.composeNetKey(n.Name)
		names = append(names, key)
		entry := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
		if len(n.Aliases) > 0 {
			entry = mapNode()
			put(entry, "aliases", n.Aliases)
		}
		put(full, key, entry)
		if key == "default" && !external {
			continue // the implicit project network
		}
		if top == nil {
			top = mapNode()
		}
		def := mapNode()
		if external {
			put(def, "external", true)
			if key != n.Name {
				put(def, "name", n.Name)
			}
		}
		put(top, key, def)
	}
	if hasAlias {
		put(svc, "networks", full)
	} else {
		put(svc, "networks", names)
	}
	return top
}

func composeSettings(svc *yaml.Node, s Spec) {
	if s.Restart != "" {
		put(svc, "restart", s.Restart)
	}
	if h := s.Health; h != nil {
		hm := mapNode()
		if h.Disabled {
			put(hm, "disable", true)
		} else {
			test := append([]string{}, h.Test...)
			put(hm, "test", flowList(test))
			for _, d := range []struct {
				k string
				v int64
				t string
			}{{"interval", int64(h.Interval), formatDur(h.Interval)}, {"timeout", int64(h.Timeout), formatDur(h.Timeout)},
				{"start_period", int64(h.StartPeriod), formatDur(h.StartPeriod)}, {"start_interval", int64(h.StartIv), formatDur(h.StartIv)}} {
				if d.v != 0 {
					put(hm, d.k, d.t)
				}
			}
			if h.Retries != 0 {
				put(hm, "retries", h.Retries)
			}
		}
		put(svc, "healthcheck", hm)
	}
	for _, f := range []struct {
		k string
		v bool
	}{{"stdin_open", s.Stdin}, {"tty", s.TTY}, {"init", s.Init}, {"privileged", s.Privileged}, {"read_only", s.ReadOnly}} {
		if f.v {
			put(svc, f.k, true)
		}
	}
	lists := []struct {
		k string
		v []string
	}{{"cap_add", s.CapAdd}, {"cap_drop", s.CapDrop}, {"security_opt", s.SecurityOpt}, {"group_add", s.GroupAdd}, {"devices", s.Devices}}
	for _, l := range lists {
		if len(l.v) > 0 {
			put(svc, l.k, escAll(l.v))
		}
	}
	if len(s.Sysctls) > 0 {
		put(svc, "sysctls", kvMap(s.Sysctls))
	}
	if len(s.Ulimits) > 0 {
		put(svc, "ulimits", composeUlimits(s.Ulimits))
	}
	for _, l := range []struct {
		k string
		v []string
	}{{"extra_hosts", s.ExtraHosts}, {"dns", s.DNS}, {"dns_search", s.DNSSearch}, {"dns_opt", s.DNSOptions}, {"links", s.Links}, {"volumes_from", s.VolumesFrom}} {
		if len(l.v) > 0 {
			put(svc, l.k, escAll(l.v))
		}
	}
	composeResources(svc, s)
	if s.LogDriver != "" || len(s.LogOpts) > 0 {
		lg := mapNode()
		if s.LogDriver != "" {
			put(lg, "driver", s.LogDriver)
		}
		if len(s.LogOpts) > 0 {
			put(lg, "options", kvMap(s.LogOpts))
		}
		put(svc, "logging", lg)
	}
	for _, kv := range [][2]string{{"pid", s.PidMode}, {"ipc", s.IpcMode}, {"uts", s.UTSMode}, {"userns_mode", s.UsernsMode}, {"runtime", s.Runtime}, {"stop_signal", s.StopSignal}} {
		if kv[1] != "" {
			put(svc, kv[0], kv[1])
		}
	}
	if s.OomScoreAdj != 0 {
		put(svc, "oom_score_adj", s.OomScoreAdj)
	}
	if s.StopTimeout != nil {
		put(svc, "stop_grace_period", strconv.Itoa(*s.StopTimeout)+"s")
	}
	if s.GPUs != "" {
		put(svc, "deploy", composeGPU(s.GPUs))
	}
}

func composeUlimits(ul []string) *yaml.Node {
	m := mapNode()
	for _, u := range ul {
		name, val, _ := strings.Cut(u, "=")
		soft, hard, split := strings.Cut(val, ":")
		if !split {
			n, _ := strconv.ParseInt(val, 10, 64)
			put(m, name, n)
			continue
		}
		s, _ := strconv.ParseInt(soft, 10, 64)
		h, _ := strconv.ParseInt(hard, 10, 64)
		lim := mapNode()
		put(lim, "soft", s)
		put(lim, "hard", h)
		put(m, name, lim)
	}
	return m
}

func composeResources(svc *yaml.Node, s Spec) {
	if s.MemoryBytes != 0 {
		put(svc, "mem_limit", formatBytes(s.MemoryBytes))
	}
	if s.MemoryReservation != 0 {
		put(svc, "mem_reservation", formatBytes(s.MemoryReservation))
	}
	if s.MemorySwap != 0 {
		put(svc, "memswap_limit", formatSwap(s.MemorySwap))
	}
	if s.NanoCPUs != 0 {
		f, _ := strconv.ParseFloat(formatCPUs(s.NanoCPUs), 64)
		put(svc, "cpus", f)
	}
	if s.CPUShares != 0 {
		put(svc, "cpu_shares", s.CPUShares)
	}
	if s.CPUQuota != 0 {
		put(svc, "cpu_quota", s.CPUQuota)
	}
	if s.CPUPeriod != 0 {
		put(svc, "cpu_period", s.CPUPeriod)
	}
	if s.CpusetCpus != "" {
		put(svc, "cpuset", s.CpusetCpus)
	}
	if s.PidsLimit != 0 {
		put(svc, "pids_limit", s.PidsLimit)
	}
	if s.ShmSize != 0 {
		put(svc, "shm_size", formatBytes(s.ShmSize))
	}
}

func composeGPU(gpus string) *yaml.Node {
	dev := mapNode()
	put(dev, "driver", "nvidia")
	switch {
	case strings.HasPrefix(gpus, "device="):
		put(dev, "device_ids", strings.Split(strings.TrimPrefix(gpus, "device="), ","))
	case gpus == "all":
		put(dev, "count", "all")
	default:
		n, _ := strconv.Atoi(gpus)
		put(dev, "count", n)
	}
	put(dev, "capabilities", &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle, Content: []*yaml.Node{scalarNode("gpu")}})
	devices := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{dev}}
	res := mapNode()
	rsv := mapNode()
	put(rsv, "devices", devices)
	put(res, "reservations", rsv)
	deploy := mapNode()
	put(deploy, "resources", res)
	return deploy
}
