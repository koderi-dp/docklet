package services

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"

	"docklet/internal/model"
)

const (
	composeProjectLabel = "com.docker.compose.project"
	composeServiceLabel = "com.docker.compose.service"
	composeLabelPrefix  = "com.docker.compose."
	mib                 = int64(1024 * 1024)
)

var (
	anonymousVolumeRe = regexp.MustCompile(`^[a-f0-9]{64}$`)
	shellSafeRe       = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)
)

// RecreateSpec derives an editable create spec from an existing container. Only
// values the user (or their tooling) set explicitly are included: command,
// entrypoint, env, labels, user and working dir that merely equal the image's
// defaults are omitted, so a newer image can change them. Secret-looking env
// values are never returned; they come back masked with Keep set.
func (s *ContainerService) RecreateSpec(ctx context.Context, connectionID, id string) (model.ContainerRecreateInfo, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.ContainerRecreateInfo{}, err
	}
	res, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return model.ContainerRecreateInfo{}, friendlyError(err)
	}
	var img *image.InspectResponse
	if ir, ierr := cli.ImageInspect(ctx, res.Container.Image); ierr == nil {
		img = &ir.InspectResponse
	}
	return buildRecreateInfo(res.Container, img), nil
}

// buildRecreateInfo is the pure mapping behind RecreateSpec. img may be nil when
// the original image cannot be inspected; nothing is then treated as a default.
func buildRecreateInfo(in container.InspectResponse, img *image.InspectResponse) model.ContainerRecreateInfo {
	info := model.ContainerRecreateInfo{ID: in.ID, Name: trimName(in.Name), AnonymousVolumes: []model.AnonymousVolume{}, Warnings: []string{}}
	info.Running = in.State != nil && (in.State.Running || in.State.Restarting || in.State.Paused)
	c := in.Config
	if c == nil {
		c = &container.Config{}
	}
	info.ComposeProject, info.ComposeService = c.Labels[composeProjectLabel], c.Labels[composeServiceLabel]
	info.ComposeManaged = info.ComposeProject != ""

	var def imageDefaults
	if img != nil && img.Config != nil {
		def = imageDefaults{env: img.Config.Env, cmd: img.Config.Cmd, entrypoint: img.Config.Entrypoint, workDir: img.Config.WorkingDir,
			user: img.Config.User, labels: img.Config.Labels, stopSignal: img.Config.StopSignal}
	} else {
		info.Warnings = append(info.Warnings, "The original image could not be inspected, so image defaults (environment, command, labels) are copied into the container.")
	}

	sp := &info.Spec
	sp.Name, sp.Image = info.Name, c.Image
	if imageIDRe.MatchString(c.Image) {
		info.Warnings = append(info.Warnings, "This container was created from an image ID, not a tag. Enter the image reference to use.")
	}
	sp.Entrypoint, sp.Command = diffCommand(c, def, &info)
	sp.Env = diffEnv(c.Env, def.env)
	sp.Labels = diffLabels(c.Labels, def.labels)
	if c.WorkingDir != def.workDir {
		sp.WorkDir = c.WorkingDir
	}
	if c.User != def.user {
		sp.User = c.User
	}
	if c.Hostname != shortID(in.ID) {
		sp.Hostname = c.Hostname
	}
	if c.StopSignal != def.stopSignal {
		sp.StopSignal = c.StopSignal
	}
	sp.StopTimeout = c.StopTimeout
	if hc := in.HostConfig; hc != nil {
		sp.Ports = portsFromHostConfig(hc)
		sp.RestartPolicy = string(hc.RestartPolicy.Name)
		sp.MaxRetries = hc.RestartPolicy.MaximumRetryCount
		sp.MemoryMiB = hc.Memory / mib
		sp.CPUs = float64(hc.NanoCPUs) / 1e9
		sp.NetworkMode = normalizeNetworkMode(string(hc.NetworkMode))
	}
	sp.Mounts, info.AnonymousVolumes = mountsFromInspect(in.Mounts)
	for _, v := range info.AnonymousVolumes {
		info.Warnings = append(info.Warnings, fmt.Sprintf("Anonymous volume at %s is not carried over: the new container starts with an empty %s. The old volume (%s) stays on the Engine until you remove it.", v.Target, v.Target, shortID(v.Name)))
	}
	sp.Networks = networksFromInspect(in, sp.NetworkMode)
	sp.AutoStart = info.Running
	return info
}

type imageDefaults struct {
	env, cmd, entrypoint []string
	workDir, user        string
	stopSignal           string
	labels               map[string]string
}

func diffCommand(c *container.Config, def imageDefaults, info *model.ContainerRecreateInfo) (entrypoint, command string) {
	if !slices.Equal(c.Entrypoint, def.entrypoint) {
		if len(c.Entrypoint) == 0 {
			info.Warnings = append(info.Warnings, "The entrypoint of the image was cleared on this container; that cannot be reproduced, the image entrypoint will apply.")
		}
		// An entrypoint override discards the image's default command, so keep the command explicit.
		return quoteArgv(c.Entrypoint), quoteArgv(c.Cmd)
	}
	if !slices.Equal(c.Cmd, def.cmd) {
		return "", quoteArgv(c.Cmd)
	}
	return "", ""
}

// quoteArgv renders argv so splitCommand parses it back to the same slice.
func quoteArgv(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		switch {
		case a == "":
			parts[i] = "''"
		case shellSafeRe.MatchString(a):
			parts[i] = a
		default:
			parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(parts, " ")
}

func diffEnv(env, imageEnv []string) []model.CreateEnvVar {
	out := []model.CreateEnvVar{}
	for _, e := range env {
		if slices.Contains(imageEnv, e) {
			continue
		}
		k, v, _ := strings.Cut(e, "=")
		if isSecretKey(k) {
			out = append(out, model.CreateEnvVar{Key: k, Keep: true})
		} else {
			out = append(out, model.CreateEnvVar{Key: k, Value: v})
		}
	}
	return out
}

func diffLabels(labels, imageLabels map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range labels {
		if strings.HasPrefix(k, composeLabelPrefix) {
			continue
		}
		if iv, ok := imageLabels[k]; ok && iv == v {
			continue
		}
		out[k] = v
	}
	return out
}

func portsFromHostConfig(hc *container.HostConfig) []model.PortMapping {
	out := []model.PortMapping{}
	for port, bindings := range hc.PortBindings {
		for _, b := range bindings {
			pm := model.PortMapping{ContainerPort: int(port.Num()), Protocol: string(port.Proto())}
			if b.HostIP.IsValid() {
				pm.HostIP = b.HostIP.String()
			}
			pm.HostPort, _ = strconv.Atoi(b.HostPort)
			out = append(out, pm)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ContainerPort != out[j].ContainerPort {
			return out[i].ContainerPort < out[j].ContainerPort
		}
		return out[i].HostPort < out[j].HostPort
	})
	return out
}

func mountsFromInspect(points []container.MountPoint) ([]model.MountSpec, []model.AnonymousVolume) {
	mounts, anon := []model.MountSpec{}, []model.AnonymousVolume{}
	for _, m := range points {
		switch m.Type {
		case mount.TypeBind:
			mounts = append(mounts, model.MountSpec{Type: "bind", Source: m.Source, Target: m.Destination, ReadOnly: !m.RW})
		case mount.TypeVolume:
			if anonymousVolumeRe.MatchString(m.Name) {
				anon = append(anon, model.AnonymousVolume{Name: m.Name, Target: m.Destination})
				continue
			}
			mounts = append(mounts, model.MountSpec{Type: "volume", Source: m.Name, Target: m.Destination, ReadOnly: !m.RW})
		}
	}
	sort.Slice(mounts, func(i, j int) bool { return mounts[i].Target < mounts[j].Target })
	sort.Slice(anon, func(i, j int) bool { return anon[i].Target < anon[j].Target })
	return mounts, anon
}

func normalizeNetworkMode(m string) string {
	if m == "default" || m == "bridge" {
		return ""
	}
	return m
}

func networksFromInspect(in container.InspectResponse, mode string) []model.NetworkAttach {
	out := []model.NetworkAttach{}
	if in.NetworkSettings == nil || mode == "host" || mode == "none" || strings.HasPrefix(mode, "container:") {
		return out
	}
	self := []string{shortID(in.ID), trimName(in.Name)}
	for name, ep := range in.NetworkSettings.Networks {
		if name == "bridge" && mode == "" {
			continue
		}
		var aliases []string
		if ep != nil {
			for _, a := range ep.Aliases {
				if !slices.Contains(self, a) {
					aliases = append(aliases, a)
				}
			}
		}
		out = append(out, model.NetworkAttach{Name: name, Aliases: aliases})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
