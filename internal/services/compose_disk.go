package services

import (
	"context"
	"sort"

	"github.com/moby/moby/client"

	"docklet/internal/model"
)

type diskAcc struct {
	d      model.ComposeProjectDisk
	images map[string]int64 // unique image ID -> size (-1 unknown)
}

func addSize(dst **int64, n int64) {
	if *dst == nil {
		z := int64(0)
		*dst = &z
	}
	**dst += n
}

// projectDiskFromUsage groups the Engine's verbose disk accounting by Compose project label.
func projectDiskFromUsage(du client.DiskUsageResult) []model.ComposeProjectDisk {
	accs := map[string]*diskAcc{}
	get := func(name string) *diskAcc {
		a := accs[name]
		if a == nil {
			a = &diskAcc{d: model.ComposeProjectDisk{Project: name}, images: map[string]int64{}}
			accs[name] = a
		}
		return a
	}
	imageSize := map[string]int64{}
	for _, im := range du.Images.Items {
		imageSize[im.ID] = im.Size
	}
	for _, c := range du.Containers.Items {
		p := c.Labels[composeProjectLabel]
		if p == "" {
			continue
		}
		a := get(p)
		a.d.ContainerCount++
		if c.SizeRw >= 0 {
			a.d.ContainersKnown++
			addSize(&a.d.ContainersBytes, c.SizeRw)
		}
		if c.ImageID != "" {
			sz, ok := imageSize[c.ImageID]
			if !ok {
				sz = -1
			}
			a.images[c.ImageID] = sz
		}
	}
	for _, v := range du.Volumes.Items {
		p := v.Labels[composeProjectLabel]
		if p == "" {
			continue
		}
		a := get(p)
		a.d.VolumeCount++
		if v.UsageData != nil && v.UsageData.Size >= 0 {
			a.d.VolumesKnown++
			addSize(&a.d.VolumesBytes, v.UsageData.Size)
		}
	}
	out := make([]model.ComposeProjectDisk, 0, len(accs))
	for _, a := range accs {
		a.d.ImageCount = len(a.images)
		for _, sz := range a.images {
			if sz >= 0 {
				a.d.ImagesKnown++
				addSize(&a.d.ImagesBytes, sz)
			}
		}
		out = append(out, a.d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Project < out[j].Project })
	return out
}

// ProjectDiskUsage returns the disk footprint of every Compose project of a connection from
// ONE Engine disk-usage call. That call can take a while (the Engine walks layers), so the
// frontend loads it lazily and caches it briefly.
func (s *ComposeService) ProjectDiskUsage(ctx context.Context, connectionID string) (model.ComposeDiskUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, slowCallTimeout)
	defer cancel()
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.ComposeDiskUsage{}, err
	}
	res, err := cli.DiskUsage(ctx, client.DiskUsageOptions{Containers: true, Images: true, Volumes: true, Verbose: true})
	if err != nil {
		return model.ComposeDiskUsage{}, friendlyError(err)
	}
	return model.ComposeDiskUsage{ConnectionID: connectionID, Projects: projectDiskFromUsage(res)}, nil
}
