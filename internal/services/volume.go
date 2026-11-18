package services

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
)

// VolumeService is bound to the frontend. All methods are scoped by connectionID.
type VolumeService struct {
	Engines engine.Provider
	Events  engine.Emitter
	Ops     *ops.Manager
}

var volumeNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

// KindVolumePrune is the operation kind of Prune Unused on the Volumes page.
const KindVolumePrune = "volume.prune"

// volumeUsers maps volume name -> containers mounting it (stopped ones included).
func volumeUsers(ctx context.Context, cli *client.Client) (map[string][]model.ResourceUser, error) {
	res, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, friendlyError(err)
	}
	out := map[string][]model.ResourceUser{}
	for _, c := range res.Items {
		for _, m := range c.Mounts {
			if m.Type != "volume" || m.Name == "" {
				continue
			}
			out[m.Name] = append(out[m.Name], model.ResourceUser{
				ID: c.ID, Name: containerName(c.Names, c.ID), State: string(c.State), Destination: m.Destination, ReadOnly: !m.RW,
			})
		}
	}
	for k := range out {
		u := out[k]
		sort.Slice(u, func(i, j int) bool { return u[i].Name < u[j].Name })
	}
	return out, nil
}

func mapVolume(connectionID string, v volume.Volume, users map[string][]model.ResourceUser) model.Volume {
	u := users[v.Name]
	if u == nil {
		u = []model.ResourceUser{}
	}
	labels, opts := v.Labels, v.Options
	if labels == nil {
		labels = map[string]string{}
	}
	if opts == nil {
		opts = map[string]string{}
	}
	return model.Volume{
		ConnectionID: connectionID, Name: v.Name, Driver: v.Driver, Mountpoint: v.Mountpoint, Scope: v.Scope,
		Labels: labels, Options: opts, Created: v.CreatedAt, InUseBy: u,
	}
}

// List returns all volumes with the containers that mount each one.
func (s *VolumeService) List(ctx context.Context, connectionID string) ([]model.Volume, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return nil, err
	}
	res, err := cli.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		return nil, friendlyError(err)
	}
	users, err := volumeUsers(ctx, cli)
	if err != nil {
		return nil, err
	}
	out := make([]model.Volume, 0, len(res.Items))
	for _, v := range res.Items {
		out = append(out, mapVolume(connectionID, v, users))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Inspect returns one volume with its users.
func (s *VolumeService) Inspect(ctx context.Context, connectionID, name string) (model.Volume, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return model.Volume{}, err
	}
	res, err := cli.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	if err != nil {
		return model.Volume{}, friendlyVolumeError(err, name, nil)
	}
	users, err := volumeUsers(ctx, cli)
	if err != nil {
		return model.Volume{}, err
	}
	return mapVolume(connectionID, res.Volume, users), nil
}

// validateVolume checks a create request; labels must have non-empty keys.
func validateVolume(name, driver string, labels map[string]string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("Volume name is required")
	}
	if !volumeNameRe.MatchString(name) {
		return "", fmt.Errorf("Invalid volume name %q: use at least 2 characters from letters, digits, '_', '.' and '-', starting with a letter or digit", name)
	}
	driver = strings.TrimSpace(driver)
	if driver == "" {
		driver = "local"
	}
	if strings.ContainsAny(driver, " \t\r\n/") {
		return "", fmt.Errorf("Invalid volume driver %q", driver)
	}
	for k := range labels {
		if strings.TrimSpace(k) == "" {
			return "", fmt.Errorf("Label keys must not be empty")
		}
	}
	return driver, nil
}

// Create makes a new volume (driver defaults to local).
func (s *VolumeService) Create(ctx context.Context, connectionID, name, driver string, labels map[string]string) (model.Volume, error) {
	driver, err := validateVolume(name, driver, labels)
	if err != nil {
		return model.Volume{}, err
	}
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return model.Volume{}, err
	}
	name = strings.TrimSpace(name)
	// The Engine silently returns an existing volume for a repeated create; refuse instead.
	if _, err := cli.VolumeInspect(ctx, name, client.VolumeInspectOptions{}); err == nil {
		return model.Volume{}, fmt.Errorf("A volume named %q already exists", name)
	} else if !isNotFound(err) {
		return model.Volume{}, friendlyError(err)
	}
	res, err := cli.VolumeCreate(ctx, client.VolumeCreateOptions{Name: name, Driver: driver, Labels: labels})
	if err != nil {
		return model.Volume{}, friendlyVolumeError(err, name, nil)
	}
	emit(s.Events, model.EventVolumesChanged, model.VolumesChanged{ConnectionID: connectionID})
	return mapVolume(connectionID, res.Volume, nil), nil
}

// Remove deletes volumes one by one; ActionResult.ID and Name are the volume name.
// force is passed to the Engine (never set by default UI paths).
func (s *VolumeService) Remove(ctx context.Context, connectionID string, names []string, force bool) ([]model.ActionResult, error) {
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return nil, err
	}
	var users map[string][]model.ResourceUser
	usersLoaded := false
	results := make([]model.ActionResult, 0, len(names))
	anyOK := false
	for _, n := range names {
		r := model.ActionResult{ID: n, Name: n}
		_, err := cli.VolumeRemove(ctx, n, client.VolumeRemoveOptions{Force: force})
		if err == nil {
			r.OK = true
			anyOK = true
		} else {
			if isConflict(err) && !usersLoaded {
				users, _ = volumeUsers(ctx, cli) // best effort: only used for the message
				usersLoaded = true
			}
			r.Error = friendlyVolumeError(err, n, users[n]).Error()
		}
		results = append(results, r)
	}
	if anyOK {
		emit(s.Events, model.EventVolumesChanged, model.VolumesChanged{ConnectionID: connectionID})
	}
	return results, nil
}

// PruneUnused removes ALL unused volumes (including named ones): persistent data
// is lost, so confirmDataLoss must be true or nothing happens.
func (s *VolumeService) PruneUnused(ctx context.Context, connectionID string, confirmDataLoss bool) (model.Operation, error) {
	if !confirmDataLoss {
		return model.Operation{}, fmt.Errorf("Pruning volumes deletes data permanently; explicit confirmation is required")
	}
	return startPrune(ctx, s.Engines, s.Ops, s.Events, connectionID,
		pruneSpec{Kind: KindVolumePrune, Title: "Prune unused volumes", Resource: "volumes", Changed: []string{model.EventVolumesChanged}},
		pruneVolumesStep)
}

func pruneVolumesStep(ctx context.Context, cli *client.Client) (pruneOutcome, error) {
	res, err := cli.VolumePrune(ctx, client.VolumePruneOptions{All: true})
	if err != nil {
		return pruneOutcome{}, err
	}
	return pruneOutcome{Noun: "volumes", Items: res.Report.VolumesDeleted, Reclaimed: res.Report.SpaceReclaimed, HasSpace: true}, nil
}

// friendlyVolumeError turns Engine failures into readable messages. users are the
// containers known to mount the volume (may be nil).
func friendlyVolumeError(err error, name string, users []model.ResourceUser) error {
	switch {
	case isNotFound(err):
		return fmt.Errorf("Volume %q not found (it may have been removed)", name)
	case isConflict(err):
		if len(users) > 0 {
			return fmt.Errorf("Volume %q is in use by %s", name, userNames(users))
		}
		return fmt.Errorf("Volume %q is in use by a container: %w", name, err)
	}
	return friendlyError(err)
}
