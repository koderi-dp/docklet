package runspec

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
)

func mapHostConfig(s *Spec, hc *container.HostConfig, h Hints) {
	s.Privileged, s.ReadOnly, s.AutoRemove, s.PublishAll = hc.Privileged, hc.ReadonlyRootfs, hc.AutoRemove, hc.PublishAllPorts
	s.Init = hc.Init != nil && *hc.Init
	if n := hc.RestartPolicy.Name; n != "" && n != container.RestartPolicyDisabled {
		s.Restart = string(n)
		if n == container.RestartPolicyOnFailure && hc.RestartPolicy.MaximumRetryCount > 0 {
			s.Restart += ":" + strconv.Itoa(hc.RestartPolicy.MaximumRetryCount)
		}
	}
	s.Ports = mapPorts(hc)

	r := hc.Resources
	s.MemoryBytes, s.MemoryReservation, s.NanoCPUs = r.Memory, r.MemoryReservation, r.NanoCPUs
	s.CPUShares, s.CPUQuota, s.CPUPeriod, s.CpusetCpus = r.CPUShares, r.CPUQuota, r.CPUPeriod, r.CpusetCpus
	if r.MemorySwap != 0 && r.MemorySwap != 2*r.Memory { // 2x memory is what the daemon fills in when unset
		s.MemorySwap = r.MemorySwap
	}
	if r.PidsLimit != nil && *r.PidsLimit > 0 && !(h.Podman && *r.PidsLimit == podmanPidsLimit) {
		s.PidsLimit = *r.PidsLimit
	}
	if hc.ShmSize != 0 && hc.ShmSize != 64*mib && !(h.Podman && hc.ShmSize == podmanShmSize) {
		s.ShmSize = hc.ShmSize
	}
	for _, d := range r.Devices {
		dev := d.PathOnHost + ":" + d.PathInContainer
		if p := d.CgroupPermissions; p != "" && p != "rwm" {
			dev += ":" + p
		}
		s.Devices = append(s.Devices, dev)
	}
	for _, u := range r.Ulimits {
		name := ulimitName(u.Name)
		if u.Soft == u.Hard {
			s.Ulimits = append(s.Ulimits, fmt.Sprintf("%s=%d", name, u.Soft))
		} else {
			s.Ulimits = append(s.Ulimits, fmt.Sprintf("%s=%d:%d", name, u.Soft, u.Hard))
		}
	}
	s.GPUs = gpus(r.DeviceRequests, s)

	s.CapAdd, s.CapDrop, s.SecurityOpt, s.GroupAdd = trimCapPrefix(hc.CapAdd), trimCapPrefix(hc.CapDrop), hc.SecurityOpt, hc.GroupAdd
	for _, a := range hc.DNS {
		s.DNS = append(s.DNS, a.String())
	}
	s.DNSSearch, s.DNSOptions, s.ExtraHosts, s.Links, s.VolumesFrom = hc.DNSSearch, hc.DNSOptions, hc.ExtraHosts, hc.Links, hc.VolumesFrom
	s.Sysctls = mapKV(hc.Sysctls)
	daemonLog := h.DaemonLogDriver
	if daemonLog == "" {
		daemonLog = "json-file"
	}
	if t := hc.LogConfig.Type; t != "" && t != daemonLog {
		s.LogDriver = t
	}
	s.LogOpts = mapKV(hc.LogConfig.Config)
	if hc.Runtime != "" && hc.Runtime != "runc" && hc.Runtime != "oci" { // "oci" is Podman's name for its default
		s.Runtime = hc.Runtime
	}
	s.PidMode = hostOnly(string(hc.PidMode))
	if m := string(hc.IpcMode); m != "" && m != "private" && m != "shareable" {
		s.IpcMode = m
	}
	s.UTSMode, s.UsernsMode, s.OomScoreAdj = hostOnly(string(hc.UTSMode)), string(hc.UsernsMode), hc.OomScoreAdj
	notIncluded(s, hc)
}

func notIncluded(s *Spec, hc *container.HostConfig) {
	add := func(cond bool, what string) {
		if cond {
			s.NotIncluded = append(s.NotIncluded, what)
		}
	}
	add(len(hc.Annotations) > 0, "annotations")
	add(len(hc.StorageOpt) > 0, "storage options")
	add(hc.CgroupParent != "", "cgroup parent")
	add(len(hc.DeviceCgroupRules) > 0, "device cgroup rules")
	add(hc.BlkioWeight != 0 || len(hc.BlkioDeviceReadBps) > 0 || len(hc.BlkioDeviceWriteBps) > 0, "block IO limits")
	add(hc.VolumeDriver != "" && hc.VolumeDriver != "local", "volume driver "+hc.VolumeDriver)
	add(hc.Isolation != "" && hc.Isolation != "default", "isolation "+string(hc.Isolation))
}

func mapPorts(hc *container.HostConfig) []Port {
	var out []Port
	for port, bindings := range hc.PortBindings {
		for _, b := range bindings {
			p := Port{HostPort: b.HostPort, ContainerPort: int(port.Num()), Proto: string(port.Proto())}
			if b.HostIP.IsValid() {
				p.HostIP = b.HostIP.String()
			}
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.ContainerPort != b.ContainerPort:
			return a.ContainerPort < b.ContainerPort
		case a.Proto != b.Proto:
			return a.Proto < b.Proto
		case a.HostPort != b.HostPort:
			return a.HostPort < b.HostPort
		}
		return a.HostIP < b.HostIP
	})
	return out
}

// gpus renders simple GPU requests; anything else is reported as not included.
func gpus(reqs []container.DeviceRequest, s *Spec) string {
	if len(reqs) == 0 {
		return ""
	}
	r := reqs[0]
	simple := len(reqs) == 1 && len(r.Options) == 0 && len(r.Capabilities) == 1 && len(r.Capabilities[0]) == 1 &&
		r.Capabilities[0][0] == "gpu" && (r.Driver == "" || r.Driver == "nvidia")
	switch {
	case !simple:
		s.NotIncluded = append(s.NotIncluded, "device requests (GPU/driver options)")
		return ""
	case len(r.DeviceIDs) > 0:
		return "device=" + strings.Join(r.DeviceIDs, ",")
	case r.Count < 0:
		return "all"
	}
	return strconv.Itoa(r.Count)
}

// trimCapPrefix drops the CAP_ prefix newer Engines add (docker run accepts both spellings).
func trimCapPrefix(caps []string) []string {
	out := make([]string, len(caps))
	for i, c := range caps {
		out[i] = strings.TrimPrefix(c, "CAP_")
	}
	return out
}

// Podman's own defaults, spelled out in its inspect output.
const (
	podmanPidsLimit = 2048
	podmanShmSize   = 65536000 // "64000k"
)

// ulimitName turns Podman's "RLIMIT_NOFILE" into the name docker run accepts ("nofile").
func ulimitName(n string) string {
	return strings.ToLower(strings.TrimPrefix(n, "RLIMIT_"))
}

// hostOnly keeps the one non-default namespace mode docker run takes for --pid and --uts ("host");
// Podman spells the default "private".
func hostOnly(mode string) string {
	if mode == "private" {
		return ""
	}
	return mode
}
