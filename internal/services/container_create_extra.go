package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"docklet/internal/model"
)

var (
	hostnameRe      = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]{0,251}[a-zA-Z0-9])?$`)
	containerModeRe = regexp.MustCompile(`^container:[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
)

// applyExtraSpec maps the optional spec fields (entrypoint, working dir, user,
// hostname, labels, stop settings, network mode) onto the SDK configs.
func applyExtraSpec(spec model.ContainerCreateSpec, cfg *container.Config, host *container.HostConfig) error {
	if strings.TrimSpace(spec.Entrypoint) != "" {
		argv, err := splitCommand(spec.Entrypoint)
		if err != nil {
			return fmt.Errorf("Entrypoint: %w", err)
		}
		cfg.Entrypoint = argv
	}
	if wd := strings.TrimSpace(spec.WorkDir); wd != "" {
		if !strings.HasPrefix(wd, "/") {
			return fmt.Errorf("Working directory %q must be an absolute container path", spec.WorkDir)
		}
		cfg.WorkingDir = wd
	}
	cfg.User = strings.TrimSpace(spec.User)
	if h := strings.TrimSpace(spec.Hostname); h != "" {
		if !hostnameRe.MatchString(h) {
			return fmt.Errorf("Invalid hostname %q", spec.Hostname)
		}
		cfg.Hostname = h
	}
	for k, v := range spec.Labels {
		if strings.TrimSpace(k) == "" {
			return errors.New("Label names must not be empty")
		}
		if cfg.Labels == nil {
			cfg.Labels = map[string]string{}
		}
		cfg.Labels[k] = v
	}
	cfg.StopSignal = strings.TrimSpace(spec.StopSignal)
	if spec.StopTimeout != nil {
		if *spec.StopTimeout < 0 {
			return errors.New("Stop timeout must not be negative")
		}
		t := *spec.StopTimeout
		cfg.StopTimeout = &t
	}
	mode := strings.TrimSpace(spec.NetworkMode)
	if mode != "" && !isBuiltinNetworkMode(mode) && !containerModeRe.MatchString(mode) && !createVolumeNameRe.MatchString(mode) {
		return fmt.Errorf("Invalid network mode %q", spec.NetworkMode)
	}
	host.NetworkMode = container.NetworkMode(mode)
	for _, n := range spec.Networks {
		if !createVolumeNameRe.MatchString(strings.TrimSpace(n.Name)) {
			return fmt.Errorf("Invalid network name %q", n.Name)
		}
	}
	if len(spec.Networks) > 0 && (mode == "host" || mode == "none" || containerModeRe.MatchString(mode)) {
		return fmt.Errorf("Network mode %q cannot be combined with additional networks", mode)
	}
	return nil
}

func isBuiltinNetworkMode(m string) bool {
	return m == "default" || m == "bridge" || m == "host" || m == "none"
}

// networkingFor splits the spec's networks: the one named by the network mode is
// created with the container (so its aliases apply from the start); the rest are
// attached afterwards by connectExtra.
func networkingFor(spec model.ContainerCreateSpec) (*network.NetworkingConfig, []model.NetworkAttach) {
	mode := strings.TrimSpace(spec.NetworkMode)
	primary := ""
	if mode != "" && !isBuiltinNetworkMode(mode) && !containerModeRe.MatchString(mode) {
		primary = mode
	}
	var cfg *network.NetworkingConfig
	var extra []model.NetworkAttach
	for _, n := range spec.Networks {
		name := strings.TrimSpace(n.Name)
		if name == primary && primary != "" {
			cfg = &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{
				name: {Aliases: n.Aliases},
			}}
			continue
		}
		extra = append(extra, model.NetworkAttach{Name: name, Aliases: n.Aliases})
	}
	return cfg, extra
}

func connectExtra(ctx context.Context, cli *client.Client, id string, extra []model.NetworkAttach) error {
	for _, n := range extra {
		_, err := cli.NetworkConnect(ctx, n.Name, client.NetworkConnectOptions{
			Container:      id,
			EndpointConfig: &network.EndpointSettings{Aliases: n.Aliases},
		})
		if err != nil {
			return fmt.Errorf("connect to network %q: %w", n.Name, friendlyError(err))
		}
	}
	return nil
}
