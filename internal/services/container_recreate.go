package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"docklet/internal/model"
	"docklet/internal/ops"
)

// recreatePlan is everything runRecreate needs; it is fully validated up front so
// the background part can only fail on Engine errors.
type recreatePlan struct {
	oldID, oldName, name string
	wasRunning           bool
	oldStopTimeout       *int
	start                bool
	pull                 bool
	keepOriginal         bool
	image                string
	cfg                  *container.Config
	host                 *container.HostConfig
	netCfg               *network.NetworkingConfig
	extra                []model.NetworkAttach
}

// Recreate replaces a container by a new one built from spec, as a background
// operation. Safe ordering: pull (optional), create the new container under a
// temporary name (a failure here touches nothing), then stop the old one, swap the
// names, attach networks and start the new one. A failure after the swap rolls
// back to the original. On success the old container is removed (never its
// volumes). With KeepOriginal the old container is left untouched (a copy).
// Compose-managed containers can only be copied under a new name.
func (s *ContainerService) Recreate(ctx context.Context, connectionID, id string, spec model.ContainerCreateSpec, opts model.RecreateOptions) (model.Operation, error) {
	if s.Ops == nil {
		return model.Operation{}, errors.New("operations are not available")
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	plan, err := s.prepareRecreate(ctx, cli, id, spec, opts)
	if err != nil {
		return model.Operation{}, err
	}
	title := "Recreate " + plan.name
	if plan.keepOriginal {
		title = "Copy " + plan.oldName + " as " + plan.name
	}
	return s.Ops.Start(ops.Spec{
		Kind: "container.recreate", Title: title, ConnectionID: connectionID, Resource: plan.name, Cancellable: true,
	}, func(octx context.Context, h *ops.Handle) error {
		err := s.runRecreate(octx, cli, plan, h)
		if s.Events != nil {
			s.Events.Emit(model.EventContainersChanged, model.ContainersChanged{ConnectionID: connectionID})
		}
		return err
	}), nil
}

func (s *ContainerService) prepareRecreate(ctx context.Context, cli *client.Client, id string, spec model.ContainerCreateSpec, opts model.RecreateOptions) (*recreatePlan, error) {
	cctx, cancel := callCtx(ctx)
	defer cancel()
	res, err := cli.ContainerInspect(cctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, friendlyError(err)
	}
	old := res.Container
	oldCfg := old.Config
	if oldCfg == nil {
		oldCfg = &container.Config{}
	}
	p := &recreatePlan{oldID: old.ID, oldName: trimName(old.Name), pull: opts.PullFirst, keepOriginal: opts.KeepOriginal, start: spec.AutoStart}
	p.name = strings.TrimSpace(spec.Name)
	if p.name == "" {
		p.name = p.oldName
		spec.Name = p.name
	}
	p.wasRunning = old.State != nil && (old.State.Running || old.State.Restarting || old.State.Paused)
	p.oldStopTimeout = oldCfg.StopTimeout
	if oldCfg.Labels[composeProjectLabel] != "" && (!p.keepOriginal || p.name == p.oldName) {
		return nil, errors.New("This container is managed by Docker Compose. Update it with Compose, or create a standalone copy under a new name.")
	}
	if p.keepOriginal && p.name == p.oldName {
		return nil, errors.New("A copy needs a different name than the original")
	}
	if spec, err = resolveKeptEnv(spec, oldCfg.Env); err != nil {
		return nil, err
	}
	cfg, host, err := buildCreateConfig(spec)
	if err != nil {
		return nil, err
	}
	var img *image.InspectResponse
	if ir, ierr := cli.ImageInspect(cctx, old.Image); ierr == nil {
		img = &ir.InspectResponse
	}
	p.image = cfg.Image
	p.cfg, p.host = overlayConfig(old, img, spec, cfg, host)
	p.netCfg, p.extra = networkingFor(spec)
	return p, nil
}

func (s *ContainerService) runRecreate(ctx context.Context, cli *client.Client, p *recreatePlan, h *ops.Handle) error {
	if p.pull {
		if err := s.pullForRecreate(ctx, cli, p, h); err != nil {
			h.Log("Pull failed; nothing was changed.")
			return err
		}
	}
	if _, err := cli.ImageInspect(ctx, p.image); err != nil {
		h.Log("Image " + p.image + " is not available on this Engine; nothing was changed.")
		return fmt.Errorf("Image %q is not available on this Engine. Pull it first, or tick \"Pull image first\".", p.image)
	}

	tmp := p.name + "-docklet-new"
	if p.keepOriginal {
		tmp = p.name
	}
	h.Log("Creating new container " + tmp)
	created, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{Config: p.cfg, HostConfig: p.host, NetworkingConfig: p.netCfg, Name: tmp})
	if err != nil {
		h.Log("Create failed; the original container was not touched.")
		return friendlyError(err)
	}
	newID := created.ID
	if ctx.Err() != nil {
		h.Log("Cancelled before the swap; removing the new container. The original was not touched.")
		removeQuietly(cli, newID)
		return ctx.Err()
	}

	// Past this point the swap is under way: finish or roll back, never stop halfway.
	h.Log("Point of no return: cancellation is ignored until the swap is finished or rolled back.")
	ctx = context.WithoutCancel(ctx)
	if p.keepOriginal {
		return s.finishCopy(ctx, cli, p, newID, h)
	}
	return s.swap(ctx, cli, p, newID, tmp, h)
}

func (s *ContainerService) pullForRecreate(ctx context.Context, cli *client.Client, p *recreatePlan, h *ops.Handle) error {
	ref, err := validateRepoTag(p.image)
	if err != nil {
		return err
	}
	return (&ImageService{Registry: s.Registry}).runPull(ctx, cli, ref, h)
}

func (s *ContainerService) finishCopy(ctx context.Context, cli *client.Client, p *recreatePlan, newID string, h *ops.Handle) error {
	if err := connectExtra(ctx, cli, newID, p.extra); err != nil {
		h.Log("Attaching networks failed: " + err.Error() + "; removing the copy.")
		removeQuietly(cli, newID)
		return err
	}
	if p.start {
		h.Log("Starting " + p.name)
		if _, err := cli.ContainerStart(ctx, newID, client.ContainerStartOptions{}); err != nil {
			h.Log("Start failed: " + friendlyError(err).Error() + "; removing the copy. The original is untouched.")
			removeQuietly(cli, newID)
			return friendlyError(err)
		}
	}
	h.Log("Created " + p.name + "; the original " + p.oldName + " was left as it is.")
	return nil
}

// swap replaces old by new. Every failure after the old container was stopped or
// renamed ends in rollback().
func (s *ContainerService) swap(ctx context.Context, cli *client.Client, p *recreatePlan, newID, tmp string, h *ops.Handle) error {
	oldTmpName := p.oldName + "-docklet-old-" + shortID(p.oldID)
	stopped, oldRenamed := false, false
	rollback := func(cause error) error {
		h.Log("Rolling back: " + friendlyError(cause).Error())
		if _, err := cli.ContainerRemove(ctx, newID, client.ContainerRemoveOptions{Force: true}); err != nil {
			h.Log("Warning: could not remove the new container: " + err.Error())
		} else {
			h.Log("Removed the new container")
		}
		if oldRenamed {
			if _, err := cli.ContainerRename(ctx, p.oldID, client.ContainerRenameOptions{NewName: p.oldName}); err != nil {
				h.Log("Warning: could not rename the original back to " + p.oldName + ": " + err.Error())
			} else {
				h.Log("Renamed the original back to " + p.oldName)
			}
		}
		if p.wasRunning && stopped {
			if _, err := cli.ContainerStart(ctx, p.oldID, client.ContainerStartOptions{}); err != nil {
				h.Log("Warning: could not restart the original container: " + err.Error())
			} else {
				h.Log("Restarted the original container")
			}
		}
		h.Log("Rollback finished; the original container is back.")
		note := "The original container was kept"
		if p.wasRunning && stopped {
			note = "The original container was restored and started again"
		}
		return fmt.Errorf("%w (%s)", friendlyError(cause), note)
	}

	if p.wasRunning {
		h.Log("Stopping " + p.oldName)
		stopped = true // even a failed stop may have stopped it; restarting a running container is harmless
		if _, err := cli.ContainerStop(ctx, p.oldID, client.ContainerStopOptions{Timeout: p.oldStopTimeout}); err != nil {
			return rollback(err)
		}
	}
	h.Log("Renaming " + p.oldName + " to " + oldTmpName)
	if _, err := cli.ContainerRename(ctx, p.oldID, client.ContainerRenameOptions{NewName: oldTmpName}); err != nil {
		return rollback(err)
	}
	oldRenamed = true
	h.Log("Renaming " + tmp + " to " + p.name)
	if _, err := cli.ContainerRename(ctx, newID, client.ContainerRenameOptions{NewName: p.name}); err != nil {
		return rollback(err)
	}
	if err := connectExtra(ctx, cli, newID, p.extra); err != nil {
		return rollback(err)
	}
	if p.start {
		h.Log("Starting " + p.name)
		if _, err := cli.ContainerStart(ctx, newID, client.ContainerStartOptions{}); err != nil {
			return rollback(err)
		}
	}
	h.Log("Removing the old container " + oldTmpName + " (its volumes are kept)")
	if _, err := cli.ContainerRemove(ctx, p.oldID, client.ContainerRemoveOptions{RemoveVolumes: false}); err != nil {
		h.Log("Warning: the new container is running but the old one could not be removed: " + err.Error() + ". It is still named " + oldTmpName + ".")
	}
	h.Detail("Replaced " + p.name)
	return nil
}

func removeQuietly(cli *client.Client, id string) {
	_, _ = cli.ContainerRemove(context.WithoutCancel(context.Background()), id, client.ContainerRemoveOptions{Force: true})
}
