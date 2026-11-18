// Package runspec turns an inspected container into a neutral description (Spec) and renders it
// as a `docker run` command, a Compose service or a database connection string. It is pure: no
// Engine calls, no I/O. Only settings that differ from the image and Docker defaults are kept.
package runspec

import "time"

// Port is one published port. HostPort is empty for a dynamically assigned host port.
type Port struct {
	HostIP        string
	HostPort      string
	ContainerPort int
	Proto         string // tcp, udp, sctp
}

// Mount is a bind, named volume, anonymous volume (empty Source) or tmpfs mount.
type Mount struct {
	Type      string // bind, volume, tmpfs
	Source    string
	Target    string
	ReadOnly  bool
	TmpfsOpts string // e.g. "size=64m"
}

// EnvVar is an environment variable. Secret marks values the app treats as secret-looking.
type EnvVar struct {
	Key, Value string
	Secret     bool
}

// KV is an ordered key/value pair (labels, log options, sysctls).
type KV struct{ Key, Value string }

// NetAttach is one network the container is attached to, with its explicit aliases.
type NetAttach struct {
	Name    string
	Aliases []string
}

// Health is a health check that differs from the image's.
type Health struct {
	Disabled                                bool
	Test                                    []string
	Interval, Timeout, StartPeriod, StartIv time.Duration
	Retries                                 int
}

// Spec is the non-default configuration of one container.
type Spec struct {
	Name, Image                    string
	ComposeProject, ComposeService string

	Stdin, TTY, Init, Privileged, ReadOnly, AutoRemove, PublishAll bool
	Restart                                                        string // "always", "on-failure:3", ...

	Hostname, Domainname, User, WorkDir, StopSignal string
	StopTimeout                                     *int

	// EntrypointSet/CmdSet say the container's value differs from the image's (both may be empty).
	Entrypoint, Cmd       []string
	EntrypointSet, CmdSet bool

	Env    []EnvVar
	Labels []KV
	Ports  []Port
	Expose []string // "8080/tcp"
	Mounts []Mount

	// NetworkMode is "", host, none, container:<x> or the name of the primary user network.
	NetworkMode string
	Networks    []NetAttach // every attached network except the default bridge
	Health      *Health

	MemoryBytes, MemoryReservation, MemorySwap, NanoCPUs, CPUShares, CPUQuota, CPUPeriod, ShmSize int64
	CpusetCpus                                                                                    string
	PidsLimit                                                                                     int64

	CapAdd, CapDrop, SecurityOpt, GroupAdd, Devices, DNS, DNSSearch, DNSOptions, ExtraHosts, Links, VolumesFrom []string
	Sysctls, LogOpts                                                                                            []KV
	Ulimits                                                                                                     []string // name=soft:hard
	LogDriver, Runtime, PidMode, IpcMode, UTSMode, UsernsMode                                                   string
	OomScoreAdj                                                                                                 int
	GPUs                                                                                                        string // "all", "2", "device=0,1"

	// NotIncluded lists settings that exist on the container but cannot be expressed.
	NotIncluded []string
}

// SecretKeys returns the secret-looking variable names present on the spec.
func (s Spec) SecretKeys() []string {
	var out []string
	for _, e := range s.Env {
		if e.Secret {
			out = append(out, e.Key)
		}
	}
	return out
}
