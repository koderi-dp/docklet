package services

import (
	"context"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// sdkDebugAPI adapts the Docker SDK client.
type sdkDebugAPI struct{ cli *client.Client }

func (a sdkDebugAPI) EngineOS(ctx context.Context) (string, error) {
	r, err := a.cli.Info(ctx, client.InfoOptions{})
	return r.Info.OSType, err
}

func (a sdkDebugAPI) InspectTarget(ctx context.Context, id string) (debugTarget, error) {
	r, err := a.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return debugTarget{}, err
	}
	c := r.Container
	t := debugTarget{ID: c.ID, Name: trimName(c.Name)}
	if c.State != nil {
		t.Running, t.Paused = c.State.Running, c.State.Paused
	}
	if c.HostConfig != nil {
		t.IPCMode = string(c.HostConfig.IpcMode)
	}
	return t, nil
}

func (a sdkDebugAPI) ImageExists(ctx context.Context, ref string) (bool, error) {
	_, err := a.cli.ImageInspect(ctx, ref)
	if err == nil {
		return true, nil
	}
	if cerrdefs.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

func (a sdkDebugAPI) CreateSidecar(ctx context.Context, name string, cfg *container.Config, host *container.HostConfig) (string, error) {
	r, err := a.cli.ContainerCreate(ctx, client.ContainerCreateOptions{Name: name, Config: cfg, HostConfig: host})
	return r.ID, err
}

func (a sdkDebugAPI) StartContainer(ctx context.Context, id string) error {
	_, err := a.cli.ContainerStart(ctx, id, client.ContainerStartOptions{})
	return err
}

func (a sdkDebugAPI) RemoveContainer(ctx context.Context, id string) error {
	_, err := a.cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})
	if err != nil && (cerrdefs.IsNotFound(err) || strings.Contains(strings.ToLower(err.Error()), "already in progress")) {
		return nil // AutoRemove or a sweep got there first
	}
	return err
}

func (a sdkDebugAPI) ListDebug(ctx context.Context) ([]debugInfo, error) {
	res, err := a.cli.ContainerList(ctx, client.ContainerListOptions{
		All: true, Filters: make(client.Filters).Add("label", debugLabel+"=true"),
	})
	if err != nil {
		return nil, err
	}
	out := make([]debugInfo, 0, len(res.Items))
	for _, c := range res.Items {
		out = append(out, debugInfo{ID: c.ID, Created: c.Created, Labels: c.Labels})
	}
	return out, nil
}

// HasRunningExec asks the engine for the container's exec processes. Finished
// ones stay listed for a few minutes, so each is checked.
func (a sdkDebugAPI) HasRunningExec(ctx context.Context, id string) (bool, error) {
	r, err := a.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	for _, execID := range r.Container.ExecIDs {
		e, err := a.cli.ExecInspect(ctx, execID, client.ExecInspectOptions{})
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				continue
			}
			return false, err
		}
		if e.Running {
			return true, nil
		}
	}
	return false, nil
}
