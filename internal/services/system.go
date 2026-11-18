package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
)

// SystemService is bound to the frontend. All methods are scoped by connectionID.
type SystemService struct {
	Engines engine.Provider
	Events  engine.Emitter
	Ops     *ops.Manager
}

// Operation kinds of the Cleanup tab.
const (
	KindPruneContainers = "system.prune.containers"
	KindPruneImages     = "system.prune.images"
	KindPruneNetworks   = "system.prune.networks"
	KindPruneVolumes    = "system.prune.volumes"
	KindPruneBuildCache = "system.prune.buildcache"
	KindPruneSystem     = "system.prune"
)

// Info returns the Engine overview. APIVersion is best effort (empty if the version call fails).
func (s *SystemService) Info(ctx context.Context, connectionID string) (model.EngineInfo, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return model.EngineInfo{}, err
	}
	res, err := cli.Info(ctx, client.InfoOptions{})
	if err != nil {
		return model.EngineInfo{}, friendlyError(err)
	}
	i := res.Info
	out := model.EngineInfo{
		ConnectionID: connectionID, Name: i.Name, ServerVersion: i.ServerVersion,
		OperatingSystem: i.OperatingSystem, OSType: i.OSType, KernelVersion: i.KernelVersion, Architecture: i.Architecture,
		NCPU: i.NCPU, MemTotal: i.MemTotal, StorageDriver: i.Driver, LoggingDriver: i.LoggingDriver,
		CgroupDriver: i.CgroupDriver, CgroupVersion: i.CgroupVersion, DockerRootDir: i.DockerRootDir, DefaultRuntime: i.DefaultRuntime,
		Runtimes: []string{}, SecurityOptions: append([]string{}, i.SecurityOptions...),
		Containers: i.Containers, ContainersRunning: i.ContainersRunning, ContainersPaused: i.ContainersPaused,
		ContainersStopped: i.ContainersStopped, Images: i.Images, ServerTime: i.SystemTime,
		Warnings: append([]string{}, i.Warnings...),
	}
	for name := range i.Runtimes {
		out.Runtimes = append(out.Runtimes, name)
	}
	sort.Strings(out.Runtimes)
	if v, err := cli.ServerVersion(ctx, client.ServerVersionOptions{}); err == nil {
		out.APIVersion, out.MinAPIVersion = v.APIVersion, v.MinAPIVersion
	}
	return out, nil
}

func sizePtr(n int64) *int64 {
	if n < 0 {
		return nil
	}
	return &n
}

// DiskUsage returns Docker's own accounting per category (`docker system df`).
func (s *SystemService) DiskUsage(ctx context.Context, connectionID string) (model.DiskUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, slowCallTimeout)
	defer cancel()
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return model.DiskUsage{}, err
	}
	res, err := cli.DiskUsage(ctx, client.DiskUsageOptions{Containers: true, Images: true, Volumes: true, BuildCache: true})
	if err != nil {
		return model.DiskUsage{}, friendlyError(err)
	}
	return model.DiskUsage{ConnectionID: connectionID, Categories: []model.DiskCategory{
		{Key: "images", Label: "Images", TotalCount: res.Images.TotalCount, ActiveCount: res.Images.ActiveCount, Size: sizePtr(res.Images.TotalSize), Reclaimable: sizePtr(res.Images.Reclaimable)},
		{Key: "containers", Label: "Containers (writable layers)", TotalCount: res.Containers.TotalCount, ActiveCount: res.Containers.ActiveCount, Size: sizePtr(res.Containers.TotalSize), Reclaimable: sizePtr(res.Containers.Reclaimable)},
		{Key: "volumes", Label: "Local volumes", TotalCount: res.Volumes.TotalCount, ActiveCount: res.Volumes.ActiveCount, Size: sizePtr(res.Volumes.TotalSize), Reclaimable: sizePtr(res.Volumes.Reclaimable)},
		{Key: "buildCache", Label: "Build cache", TotalCount: res.BuildCache.TotalCount, ActiveCount: res.BuildCache.ActiveCount, Size: sizePtr(res.BuildCache.TotalSize), Reclaimable: sizePtr(res.BuildCache.Reclaimable)},
	}}, nil
}

var eventAttrKeys = []string{"name", "image", "container", "type", "driver", "exitCode", "signal", "com.docker.compose.project", "com.docker.compose.service"}

func mapEvent(m eventMessage) model.EngineEvent {
	attrs := map[string]string{}
	for _, k := range eventAttrKeys {
		if v, ok := m.Attrs[k]; ok && v != "" {
			attrs[k] = v
		}
	}
	name := m.Attrs["name"]
	if name == "" {
		name = shortID(m.ID)
	}
	t := m.TimeNano / int64(time.Millisecond)
	if m.TimeNano == 0 {
		t = m.Time * 1000
	}
	return model.EngineEvent{Time: t, Type: m.Type, Action: m.Action, ActorID: m.ID, ActorName: name, Attributes: attrs}
}

type eventMessage struct {
	Type, Action, ID string
	Attrs            map[string]string
	Time, TimeNano   int64
}

// RecentEvents returns up to limit events of the last sinceSeconds, newest first.
// It uses since/until so the Engine closes the stream and the call returns.
func (s *SystemService) RecentEvents(ctx context.Context, connectionID string, sinceSeconds, limit int) ([]model.EngineEvent, error) {
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return nil, err
	}
	if sinceSeconds <= 0 {
		sinceSeconds = 3600
	}
	if sinceSeconds > 7*24*3600 {
		sinceSeconds = 7 * 24 * 3600
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	now := time.Now()
	res := cli.Events(ctx, client.EventsListOptions{
		Since: strconv.FormatInt(now.Add(-time.Duration(sinceSeconds)*time.Second).Unix(), 10),
		Until: strconv.FormatInt(now.Unix(), 10),
	})
	var all []model.EngineEvent
	for done := false; !done; {
		select {
		case m, ok := <-res.Messages:
			if !ok {
				done = true
				break
			}
			all = append(all, mapEvent(eventMessage{
				Type: string(m.Type), Action: string(m.Action), ID: m.Actor.ID, Attrs: m.Actor.Attributes, Time: m.Time, TimeNano: m.TimeNano,
			}))
		case e, ok := <-res.Err:
			if ok && e != nil && !errors.Is(e, io.EOF) && !errors.Is(e, context.DeadlineExceeded) {
				if len(all) == 0 {
					return nil, friendlyError(e)
				}
			}
			done = true
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Time > all[j].Time })
	if len(all) > limit {
		all = all[:limit]
	}
	if all == nil {
		all = []model.EngineEvent{}
	}
	return all, nil
}

// PruneContainers removes all stopped containers.
func (s *SystemService) PruneContainers(ctx context.Context, connectionID string) (model.Operation, error) {
	return startPrune(ctx, s.Engines, s.Ops, s.Events, connectionID,
		pruneSpec{Kind: KindPruneContainers, Title: "Prune stopped containers", Resource: "containers", Changed: []string{model.EventContainersChanged}},
		pruneContainersStep)
}

// PruneImages removes dangling images; allUnused=true also removes every image no container uses.
func (s *SystemService) PruneImages(ctx context.Context, connectionID string, allUnused bool) (model.Operation, error) {
	title := "Prune dangling images"
	if allUnused {
		title = "Prune all unused images"
	}
	return startPrune(ctx, s.Engines, s.Ops, s.Events, connectionID,
		pruneSpec{Kind: KindPruneImages, Title: title, Resource: "images", Changed: []string{"images:changed"}},
		pruneImagesStep(allUnused))
}

// PruneNetworks removes unused networks.
func (s *SystemService) PruneNetworks(ctx context.Context, connectionID string) (model.Operation, error) {
	return startPrune(ctx, s.Engines, s.Ops, s.Events, connectionID,
		pruneSpec{Kind: KindPruneNetworks, Title: "Prune unused networks", Resource: "networks", Changed: []string{model.EventNetworksChanged}},
		pruneNetworksStep)
}

// PruneVolumes removes all unused volumes; requires confirmDataLoss because data is deleted permanently.
func (s *SystemService) PruneVolumes(ctx context.Context, connectionID string, confirmDataLoss bool) (model.Operation, error) {
	if !confirmDataLoss {
		return model.Operation{}, fmt.Errorf("Pruning volumes deletes data permanently; explicit confirmation is required")
	}
	return startPrune(ctx, s.Engines, s.Ops, s.Events, connectionID,
		pruneSpec{Kind: KindPruneVolumes, Title: "Prune unused volumes", Resource: "volumes", Changed: []string{model.EventVolumesChanged}},
		pruneVolumesStep)
}

// PruneBuildCache removes unused build cache; all=true removes the whole cache.
func (s *SystemService) PruneBuildCache(ctx context.Context, connectionID string, all bool) (model.Operation, error) {
	title := "Prune build cache"
	if all {
		title = "Prune all build cache"
	}
	return startPrune(ctx, s.Engines, s.Ops, s.Events, connectionID,
		pruneSpec{Kind: KindPruneBuildCache, Title: title, Resource: "build cache"},
		func(ctx context.Context, cli *client.Client) (pruneOutcome, error) {
			res, err := cli.BuildCachePrune(ctx, client.BuildCachePruneOptions{All: all})
			if err != nil {
				return pruneOutcome{}, err
			}
			return pruneOutcome{Noun: "build cache records", Items: res.Report.CachesDeleted, Reclaimed: res.Report.SpaceReclaimed, HasSpace: true}, nil
		})
}

// SystemPrune runs container, network and image prune (dangling only unless allImages)
// and, ONLY when includeVolumes AND confirmDataLoss are both true, volume prune.
func (s *SystemService) SystemPrune(ctx context.Context, connectionID string, allImages, includeVolumes, confirmDataLoss bool) (model.Operation, error) {
	if includeVolumes && !confirmDataLoss {
		return model.Operation{}, fmt.Errorf("Including volumes deletes data permanently; explicit confirmation is required")
	}
	steps := []pruneStep{pruneContainersStep, pruneNetworksStep, pruneImagesStep(allImages)}
	changed := []string{model.EventContainersChanged, model.EventNetworksChanged, "images:changed"}
	title := "System prune"
	if includeVolumes {
		steps = append(steps, pruneVolumesStep)
		changed = append(changed, model.EventVolumesChanged)
		title = "System prune (including volumes)"
	}
	return startPrune(ctx, s.Engines, s.Ops, s.Events, connectionID,
		pruneSpec{Kind: KindPruneSystem, Title: title, Resource: "system", Changed: changed}, steps...)
}

func pruneContainersStep(ctx context.Context, cli *client.Client) (pruneOutcome, error) {
	res, err := cli.ContainerPrune(ctx, client.ContainerPruneOptions{})
	if err != nil {
		return pruneOutcome{}, err
	}
	return pruneOutcome{Noun: "containers", Items: res.Report.ContainersDeleted, Reclaimed: res.Report.SpaceReclaimed, HasSpace: true}, nil
}

func pruneImagesStep(allUnused bool) pruneStep {
	return func(ctx context.Context, cli *client.Client) (pruneOutcome, error) {
		f := client.Filters{}.Add("dangling", strconv.FormatBool(!allUnused))
		res, err := cli.ImagePrune(ctx, client.ImagePruneOptions{Filters: f})
		if err != nil {
			return pruneOutcome{}, err
		}
		var items []string
		for _, d := range res.Report.ImagesDeleted {
			switch {
			case d.Untagged != "":
				items = append(items, "untagged "+d.Untagged)
			case d.Deleted != "":
				items = append(items, shortID(d.Deleted))
			}
		}
		return pruneOutcome{Noun: "images", Items: items, Reclaimed: res.Report.SpaceReclaimed, HasSpace: true}, nil
	}
}
