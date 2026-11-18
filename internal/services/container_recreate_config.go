package services

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/mount"

	"docklet/internal/model"
)

// resolveKeptEnv replaces every env row marked Keep with the value the existing
// container has. A posted Value of a kept row is ignored, so the old secret is
// preserved server-side and never has to reach the UI.
func resolveKeptEnv(spec model.ContainerCreateSpec, oldEnv []string) (model.ContainerCreateSpec, error) {
	old := map[string]string{}
	for _, e := range oldEnv {
		k, v, _ := strings.Cut(e, "=")
		old[k] = v
	}
	env := make([]model.CreateEnvVar, len(spec.Env))
	for i, e := range spec.Env {
		if e.Keep {
			v, ok := old[strings.TrimSpace(e.Key)]
			if !ok {
				return spec, fmt.Errorf("Environment variable %q is marked unchanged but the container does not have it. Enter a value.", e.Key)
			}
			e = model.CreateEnvVar{Key: e.Key, Value: v}
		}
		env[i] = e
	}
	spec.Env = env
	return spec, nil
}

// overlayConfig builds the replacement's configs from the old container's: the
// settings the form edits come from the spec (already mapped into cfg/host by
// buildCreateConfig); everything else the form does not expose (tty, healthcheck,
// privileged, capabilities, devices, DNS, log config, tmpfs...) is carried over so
// recreating never silently drops it.
func overlayConfig(old container.InspectResponse, img *image.InspectResponse, spec model.ContainerCreateSpec,
	cfg *container.Config, host *container.HostConfig) (*container.Config, *container.HostConfig) {
	nc := container.Config{}
	if old.Config != nil {
		nc = *old.Config
	}
	nc.Image, nc.Cmd, nc.Entrypoint, nc.Env, nc.Labels = cfg.Image, cfg.Cmd, cfg.Entrypoint, cfg.Env, cfg.Labels
	nc.WorkingDir, nc.User, nc.Hostname = cfg.WorkingDir, cfg.User, cfg.Hostname
	nc.StopSignal, nc.StopTimeout, nc.ExposedPorts = cfg.StopSignal, cfg.StopTimeout, cfg.ExposedPorts
	nc.Volumes = nil // image VOLUMEs are re-derived from the (new) image
	if img != nil && img.Config != nil && reflect.DeepEqual(nc.Healthcheck, img.Config.Healthcheck) {
		nc.Healthcheck = nil // an image default: let the new image decide
	}

	nh := container.HostConfig{}
	if old.HostConfig != nil {
		nh = *old.HostConfig
	}
	nh.PortBindings, nh.RestartPolicy, nh.NanoCPUs, nh.NetworkMode = host.PortBindings, host.RestartPolicy, host.NanoCPUs, host.NetworkMode
	nh.Memory = host.Memory
	if old.HostConfig != nil && spec.MemoryMiB == old.HostConfig.Memory/mib {
		nh.Memory = old.HostConfig.Memory // the form rounds to MiB; keep the exact value when untouched
	}
	nh.Binds = nil
	nh.Mounts = mergeMounts(host.Mounts, nh.Mounts)
	return &nc, &nh
}

// mergeMounts returns the spec's bind/volume mounts (keeping the options of the
// matching old mount) plus the old mounts of other types (tmpfs, npipe...).
func mergeMounts(fresh, old []mount.Mount) []mount.Mount {
	out := make([]mount.Mount, 0, len(fresh)+len(old))
	for _, m := range fresh {
		for _, o := range old {
			if o.Type == m.Type && o.Target == m.Target {
				m.BindOptions, m.VolumeOptions = o.BindOptions, o.VolumeOptions
			}
		}
		out = append(out, m)
	}
	for _, o := range old {
		if o.Type != mount.TypeBind && o.Type != mount.TypeVolume {
			out = append(out, o)
		}
	}
	return out
}
