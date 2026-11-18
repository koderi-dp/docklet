package runspec

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/mount"
)

var (
	anonymousVolumeRe = regexp.MustCompile(`^[a-f0-9]{64}$`)
	// urlCredentialsRe finds a password embedded in a URL value, whatever the variable is called.
	urlCredentialsRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^/\s:@]+:[^/\s@]+@`)
	imageIDRe        = regexp.MustCompile(`^(sha256:)?[a-f0-9]{12,64}$`)
	// engineLabelPrefixes are labels written by the Engine or its tooling, never by the user.
	engineLabelPrefixes = []string{"com.docker.compose.", "com.docker.stack.", "com.docker.swarm.", "desktop.docker.io/"}
)

const mib = int64(1024 * 1024)

// imgDefaults is what the image itself provides; the zero value means "no image information".
type imgDefaults struct {
	env, entrypoint, cmd      []string
	workDir, user, stopSignal string
	labels                    map[string]string
	volumes, exposed          map[string]struct{}
	health                    *container.HealthConfig
}

func defaultsOf(img *image.InspectResponse) imgDefaults {
	if img == nil || img.Config == nil {
		return imgDefaults{}
	}
	c := img.Config
	return imgDefaults{env: c.Env, entrypoint: c.Entrypoint, cmd: c.Cmd, workDir: c.WorkingDir, user: c.User, stopSignal: c.StopSignal,
		labels: c.Labels, volumes: c.Volumes, exposed: c.ExposedPorts, health: c.Healthcheck}
}

// Hints carry what the inspect data alone cannot tell.
type Hints struct {
	// IsSecret classifies env keys (nil: none is secret).
	IsSecret func(key string) bool
	// DaemonLogDriver is the Engine's default log driver; a container using it is not "customised".
	// Empty means json-file.
	DaemonLogDriver string
}

// FromInspect maps an inspected container to a Spec. img is the container's image (nil when it
// cannot be inspected: nothing is then treated as an image default).
func FromInspect(in container.InspectResponse, img *image.InspectResponse, h Hints) Spec {
	isSecret := h.IsSecret
	c := in.Config
	if c == nil {
		c = &container.Config{}
	}
	d := defaultsOf(img)
	s := Spec{Name: strings.TrimPrefix(in.Name, "/"), Image: c.Image}
	s.ComposeProject, s.ComposeService = c.Labels["com.docker.compose.project"], c.Labels["com.docker.compose.service"]
	if imageIDRe.MatchString(c.Image) {
		s.NotIncluded = append(s.NotIncluded, "the container was created from an image ID, not a tag; replace the image reference")
	}
	s.Stdin, s.TTY = c.OpenStdin, c.Tty
	s.WorkDir, s.User, s.StopSignal = diffStr(c.WorkingDir, d.workDir), diffStr(c.User, d.user), diffStr(c.StopSignal, d.stopSignal)
	s.StopTimeout, s.Domainname = c.StopTimeout, c.Domainname
	if c.Hostname != shortID(in.ID) {
		s.Hostname = c.Hostname
	}
	mapCommand(&s, c, d)
	for _, e := range c.Env {
		if slices.Contains(d.env, e) {
			continue
		}
		k, v, _ := strings.Cut(e, "=")
		s.Env = append(s.Env, EnvVar{Key: k, Value: v, Secret: (isSecret != nil && isSecret(k)) || urlCredentialsRe.MatchString(v)})
	}
	s.Labels = userLabels(c.Labels, d.labels)
	s.Health = diffHealth(c.Healthcheck, d.health, &s)
	for p := range c.ExposedPorts {
		key := fmt.Sprintf("%d/%s", p.Num(), p.Proto())
		if _, inImage := d.exposed[key]; !inImage && !publishes(in.HostConfig, key) {
			s.Expose = append(s.Expose, key)
		}
	}
	sort.Strings(s.Expose)
	if hc := in.HostConfig; hc != nil {
		mapHostConfig(&s, hc, h)
	}
	mapMounts(&s, in, d)
	mapNetworks(&s, in)
	return s
}

func mapCommand(s *Spec, c *container.Config, d imgDefaults) {
	switch {
	case !slices.Equal(c.Entrypoint, d.entrypoint):
		// An entrypoint override discards the image's default command, so both are explicit.
		s.Entrypoint, s.EntrypointSet = c.Entrypoint, true
		s.Cmd, s.CmdSet = c.Cmd, len(c.Cmd) > 0
	case !slices.Equal(c.Cmd, d.cmd):
		if len(c.Cmd) == 0 {
			s.NotIncluded = append(s.NotIncluded, "the image command was cleared on this container; it cannot be reproduced")
		} else {
			s.Cmd, s.CmdSet = c.Cmd, true
		}
	}
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func diffStr(v, imageDefault string) string {
	if v == imageDefault {
		return ""
	}
	return v
}

func userLabels(labels, imageLabels map[string]string) []KV {
	var out []KV
	for k, v := range labels {
		if iv, ok := imageLabels[k]; ok && iv == v {
			continue
		}
		if slices.ContainsFunc(engineLabelPrefixes, func(p string) bool { return strings.HasPrefix(k, p) }) {
			continue
		}
		out = append(out, KV{k, v})
	}
	return sortKV(out)
}

func sortKV(kv []KV) []KV {
	sort.Slice(kv, func(i, j int) bool { return kv[i].Key < kv[j].Key })
	return kv
}

func mapKV(m map[string]string) []KV {
	var out []KV
	for k, v := range m {
		out = append(out, KV{k, v})
	}
	return sortKV(out)
}

func diffHealth(h, imageHealth *container.HealthConfig, s *Spec) *Health {
	if h == nil || len(h.Test) == 0 || reflect.DeepEqual(h, imageHealth) {
		return nil
	}
	if h.Test[0] == "NONE" {
		return &Health{Disabled: true}
	}
	if h.Test[0] != "CMD" && h.Test[0] != "CMD-SHELL" {
		s.NotIncluded = append(s.NotIncluded, "a health check of unknown form")
		return nil
	}
	if h.Test[0] == "CMD" {
		s.NotIncluded = append(s.NotIncluded, "the health check uses the exec (CMD) form; it is written in shell form")
	}
	return &Health{Test: h.Test, Interval: h.Interval, Timeout: h.Timeout, StartPeriod: h.StartPeriod, StartIv: h.StartInterval, Retries: h.Retries}
}

func publishes(hc *container.HostConfig, key string) bool {
	if hc == nil {
		return false
	}
	for port := range hc.PortBindings {
		if fmt.Sprintf("%d/%s", port.Num(), port.Proto()) == key {
			return true
		}
	}
	return false
}

func mapMounts(s *Spec, in container.InspectResponse, d imgDefaults) {
	var vols []Mount
	for _, m := range in.Mounts {
		switch m.Type {
		case mount.TypeBind:
			if m.Propagation != "" && m.Propagation != mount.PropagationRPrivate {
				s.NotIncluded = append(s.NotIncluded, fmt.Sprintf("bind propagation %s of %s", m.Propagation, m.Destination))
			}
			vols = append(vols, Mount{Type: "bind", Source: m.Source, Target: m.Destination, ReadOnly: !m.RW})
		case mount.TypeVolume:
			if m.Driver != "" && m.Driver != "local" {
				s.NotIncluded = append(s.NotIncluded, fmt.Sprintf("volume driver %s of %s", m.Driver, m.Destination))
			}
			if anonymousVolumeRe.MatchString(m.Name) {
				if _, imageVol := d.volumes[m.Destination]; !imageVol {
					vols = append(vols, Mount{Type: "volume", Target: m.Destination, ReadOnly: !m.RW})
				}
				continue
			}
			vols = append(vols, Mount{Type: "volume", Source: m.Name, Target: m.Destination, ReadOnly: !m.RW})
		}
	}
	sort.Slice(vols, func(i, j int) bool { return vols[i].Target < vols[j].Target })
	s.Mounts = vols
	if hc := in.HostConfig; hc != nil {
		s.Mounts = append(s.Mounts, tmpfsMounts(hc)...)
		for _, m := range hc.Mounts {
			if o := m.VolumeOptions; o != nil && (o.Subpath != "" || o.NoCopy || o.DriverConfig != nil) {
				s.NotIncluded = append(s.NotIncluded, "volume options of "+m.Target)
			}
		}
	}
}

func tmpfsMounts(hc *container.HostConfig) []Mount {
	var out []Mount
	for target, opts := range hc.Tmpfs {
		out = append(out, Mount{Type: "tmpfs", Target: target, TmpfsOpts: opts})
	}
	for _, m := range hc.Mounts {
		if m.Type != mount.TypeTmpfs {
			continue
		}
		var opts []string
		if o := m.TmpfsOptions; o != nil {
			if o.SizeBytes > 0 {
				opts = append(opts, fmt.Sprintf("size=%d", o.SizeBytes))
			}
			if o.Mode != 0 {
				opts = append(opts, fmt.Sprintf("mode=%o", o.Mode))
			}
		}
		out = append(out, Mount{Type: "tmpfs", Target: m.Target, TmpfsOpts: strings.Join(opts, ",")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

func mapNetworks(s *Spec, in container.InspectResponse) {
	mode := ""
	if in.HostConfig != nil {
		mode = string(in.HostConfig.NetworkMode)
	}
	if mode == "default" || mode == "bridge" {
		mode = ""
	}
	s.NetworkMode = mode
	if in.NetworkSettings == nil || mode == "host" || mode == "none" || strings.HasPrefix(mode, "container:") {
		return
	}
	self := []string{shortID(in.ID), s.Name, s.ComposeService}
	for name, ep := range in.NetworkSettings.Networks {
		if name == "bridge" && mode == "" {
			continue
		}
		na := NetAttach{Name: name}
		if ep != nil {
			for _, a := range ep.Aliases {
				if !slices.Contains(self, a) {
					na.Aliases = append(na.Aliases, a)
				}
			}
			sort.Strings(na.Aliases)
			if ep.IPAMConfig != nil && (ep.IPAMConfig.IPv4Address.IsValid() || ep.IPAMConfig.IPv6Address.IsValid()) {
				s.NotIncluded = append(s.NotIncluded, "static IP address on network "+name)
			}
		}
		s.Networks = append(s.Networks, na)
	}
	// The primary network (the run-time --network) comes first, the rest alphabetical.
	sort.Slice(s.Networks, func(i, j int) bool {
		a, b := s.Networks[i].Name, s.Networks[j].Name
		if (a == mode) != (b == mode) {
			return a == mode
		}
		return a < b
	})
}
