package services

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"docklet/internal/model"
)

var (
	createNameRe       = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
	createVolumeNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	envKeyRe           = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
)

// Create creates (and optionally starts) a container from the spec. It never pulls:
// a missing image yields a friendly error. On success it emits "containers:changed".
// If creation worked but the start failed, the result is OK with Error describing it.
func (s *ContainerService) Create(ctx context.Context, connectionID string, spec model.ContainerCreateSpec) (model.ActionResult, error) {
	cfg, host, err := buildCreateConfig(spec)
	if err != nil {
		return model.ActionResult{}, err
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.ActionResult{}, err
	}
	image := strings.TrimSpace(spec.Image)
	if _, err := cli.ImageInspect(ctx, image); err != nil {
		if cerrdefs.IsNotFound(err) {
			return model.ActionResult{}, fmt.Errorf("Image %q is not available on this Engine. Pull it first from the Images page, then create the container.", image)
		}
		return model.ActionResult{}, friendlyError(err)
	}
	name := strings.TrimSpace(spec.Name)
	res, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{Config: cfg, HostConfig: host, Name: name})
	if err != nil {
		return model.ActionResult{}, friendlyError(err)
	}
	out := model.ActionResult{ID: res.ID, Name: name, OK: true}
	if out.Name == "" {
		out.Name = shortID(res.ID)
	}
	if spec.AutoStart {
		if _, err := cli.ContainerStart(ctx, res.ID, client.ContainerStartOptions{}); err != nil {
			out.Error = "Container was created but could not be started: " + friendlyError(err).Error()
		}
	}
	if s.Events != nil {
		s.Events.Emit("containers:changed", map[string]string{"connectionId": connectionID})
	}
	return out, nil
}

// buildCreateConfig validates the spec and maps it to SDK configs.
func buildCreateConfig(spec model.ContainerCreateSpec) (*container.Config, *container.HostConfig, error) {
	image := strings.TrimSpace(spec.Image)
	if image == "" {
		return nil, nil, errors.New("Image is required")
	}
	if strings.ContainsAny(image, " \t\r\n") {
		return nil, nil, errors.New("Image reference must not contain spaces")
	}
	name := strings.TrimSpace(spec.Name)
	if name != "" && !createNameRe.MatchString(name) {
		return nil, nil, errors.New("Name may only contain letters, digits, '_', '.' and '-' and must start with a letter or digit")
	}
	argv, err := splitCommand(spec.Command)
	if err != nil {
		return nil, nil, err
	}

	cfg := &container.Config{Image: image, Cmd: argv}
	host := &container.HostConfig{}

	seenEnv := map[string]bool{}
	for _, e := range spec.Env {
		k := strings.TrimSpace(e.Key)
		if k == "" && e.Value == "" {
			continue // empty row
		}
		if !envKeyRe.MatchString(k) {
			return nil, nil, fmt.Errorf("Invalid environment variable name %q", e.Key)
		}
		if seenEnv[k] {
			return nil, nil, fmt.Errorf("Environment variable %q is set twice", k)
		}
		seenEnv[k] = true
		cfg.Env = append(cfg.Env, k+"="+e.Value)
	}

	for _, p := range spec.Ports {
		if p.ContainerPort == 0 && p.HostPort == 0 && strings.TrimSpace(p.HostIP) == "" {
			continue
		}
		if p.ContainerPort < 1 || p.ContainerPort > 65535 {
			return nil, nil, fmt.Errorf("Container port %d is out of range (1-65535)", p.ContainerPort)
		}
		if p.HostPort < 0 || p.HostPort > 65535 {
			return nil, nil, fmt.Errorf("Host port %d is out of range (1-65535)", p.HostPort)
		}
		proto := strings.ToLower(strings.TrimSpace(p.Protocol))
		if proto == "" {
			proto = "tcp"
		}
		if proto != "tcp" && proto != "udp" && proto != "sctp" {
			return nil, nil, fmt.Errorf("Unsupported protocol %q (tcp, udp or sctp)", p.Protocol)
		}
		var ip netip.Addr
		if h := strings.TrimSpace(p.HostIP); h != "" {
			ip, err = netip.ParseAddr(h)
			if err != nil {
				return nil, nil, fmt.Errorf("Invalid host IP %q", p.HostIP)
			}
		}
		port, err := network.ParsePort(fmt.Sprintf("%d/%s", p.ContainerPort, proto))
		if err != nil {
			return nil, nil, err
		}
		hp := ""
		if p.HostPort > 0 {
			hp = fmt.Sprint(p.HostPort)
		}
		if cfg.ExposedPorts == nil {
			cfg.ExposedPorts = network.PortSet{}
			host.PortBindings = network.PortMap{}
		}
		cfg.ExposedPorts[port] = struct{}{}
		host.PortBindings[port] = append(host.PortBindings[port], network.PortBinding{HostIP: ip, HostPort: hp})
	}

	seenTarget := map[string]bool{}
	for _, m := range spec.Mounts {
		if strings.TrimSpace(m.Source) == "" && strings.TrimSpace(m.Target) == "" {
			continue
		}
		target := strings.TrimSpace(m.Target)
		if !strings.HasPrefix(target, "/") || strings.ContainsRune(target, 0) {
			return nil, nil, fmt.Errorf("Mount target %q must be an absolute container path (start with /)", m.Target)
		}
		if seenTarget[target] {
			return nil, nil, fmt.Errorf("Mount target %q is used twice", target)
		}
		seenTarget[target] = true
		src := strings.TrimSpace(m.Source)
		switch m.Type {
		case "bind":
			if src == "" {
				return nil, nil, errors.New("Bind mounts need a host path")
			}
			host.Mounts = append(host.Mounts, mount.Mount{Type: mount.TypeBind, Source: src, Target: target, ReadOnly: m.ReadOnly})
		case "volume":
			if !createVolumeNameRe.MatchString(src) {
				return nil, nil, fmt.Errorf("Invalid volume name %q", m.Source)
			}
			host.Mounts = append(host.Mounts, mount.Mount{Type: mount.TypeVolume, Source: src, Target: target, ReadOnly: m.ReadOnly})
		default:
			return nil, nil, fmt.Errorf("Unknown mount type %q (bind or volume)", m.Type)
		}
	}

	switch spec.RestartPolicy {
	case "", "no":
	case "always":
		host.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyAlways}
	case "unless-stopped":
		host.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyUnlessStopped}
	case "on-failure":
		if spec.MaxRetries < 0 {
			return nil, nil, errors.New("Maximum retries must not be negative")
		}
		host.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyOnFailure, MaximumRetryCount: spec.MaxRetries}
	default:
		return nil, nil, fmt.Errorf("Unknown restart policy %q", spec.RestartPolicy)
	}

	if spec.MemoryMiB < 0 || (spec.MemoryMiB > 0 && spec.MemoryMiB < 6) {
		return nil, nil, errors.New("Memory limit must be at least 6 MiB (or empty for unlimited)")
	}
	if spec.MemoryMiB > 0 {
		host.Memory = spec.MemoryMiB * 1024 * 1024
	}
	if spec.CPUs < 0 || spec.CPUs > 1024 {
		return nil, nil, errors.New("CPU limit must be a positive number (or empty for unlimited)")
	}
	if spec.CPUs > 0 {
		host.NanoCPUs = int64(spec.CPUs * 1e9)
	}
	return cfg, host, nil
}

// splitCommand splits a command string into argv with shell-words rules
// (single quotes literal, double quotes with backslash escapes, backslash outside
// quotes). Nothing is ever executed through a shell.
func splitCommand(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inWord := false
	const (
		none = iota
		single
		double
	)
	state := none
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch state {
		case single:
			if c == '\'' {
				state = none
			} else {
				cur.WriteRune(c)
			}
		case double:
			switch {
			case c == '"':
				state = none
			case c == '\\' && i+1 < len(rs) && strings.ContainsRune("\"\\$`", rs[i+1]):
				i++
				cur.WriteRune(rs[i])
			default:
				cur.WriteRune(c)
			}
		default:
			switch {
			case c == ' ' || c == '\t' || c == '\n' || c == '\r':
				if inWord {
					out = append(out, cur.String())
					cur.Reset()
					inWord = false
				}
			case c == '\'':
				state, inWord = single, true
			case c == '"':
				state, inWord = double, true
			case c == '\\':
				if i+1 >= len(rs) {
					return nil, errors.New("Command ends with a dangling backslash")
				}
				i++
				cur.WriteRune(rs[i])
				inWord = true
			default:
				cur.WriteRune(c)
				inWord = true
			}
		}
	}
	if state != none {
		return nil, errors.New("Command has an unterminated quote")
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out, nil
}
