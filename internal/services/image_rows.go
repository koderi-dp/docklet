package services

import (
	"docklet/internal/model"
	"sort"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
)

// imageUse maps image ID -> containers using it.
func containersByImage(cs []container.Summary) map[string][]model.ImageContainerRef {
	m := map[string][]model.ImageContainerRef{}
	for _, c := range cs {
		name := shortID(c.ID)
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		m[c.ImageID] = append(m[c.ImageID], model.ImageContainerRef{ID: c.ID, Name: name, State: string(c.State)})
	}
	return m
}

// buildImageRows maps the Engine image list to one row per tag (plus one per
// untagged image). In-use is computed from containers of the same connection.
func buildImageRows(connectionID string, imgs []image.Summary, use map[string][]model.ImageContainerRef) []model.ImageRow {
	rows := make([]model.ImageRow, 0, len(imgs))
	for _, im := range imgs {
		n := len(use[im.ID])
		base := model.ImageRow{
			ConnectionID: connectionID, ID: im.ID, Created: im.Created, Size: im.Size,
			InUse: n > 0, ContainerCount: n,
		}
		added := false
		for _, t := range im.RepoTags {
			if t == "<none>:<none>" || t == "" {
				continue
			}
			r := base
			if i := strings.Index(t, "@"); i >= 0 {
				// Digest-only reference (containerd image store): no tag, shown as repo:<none>.
				r.Key, r.Ref, r.Repository, r.Dangling = im.ID+"|"+t, t, t[:i], true
			} else {
				repo, tag := splitRepoTag(t)
				r.Key, r.Ref, r.Repository, r.Tag = im.ID+"|"+t, t, repo, tag
			}
			rows = append(rows, r)
			added = true
		}
		if !added {
			r := base
			r.Key, r.Ref, r.Dangling = im.ID+"|", im.ID, true
			rows = append(rows, r)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Dangling != b.Dangling {
			return !a.Dangling
		}
		if a.Repository != b.Repository {
			return a.Repository < b.Repository
		}
		return a.Tag < b.Tag
	})
	return rows
}

// splitRepoTag splits at the last colon that follows the last slash.
func splitRepoTag(ref string) (repo, tag string) {
	i := strings.LastIndex(ref, ":")
	if i < 0 || strings.Contains(ref[i:], "/") {
		return ref, ""
	}
	return ref[:i], ref[i+1:]
}

func mapImageDetail(in image.InspectResponse) model.ImageDetail {
	d := model.ImageDetail{
		ID: in.ID, Tags: nonNil(in.RepoTags), Digests: nonNil(in.RepoDigests), Created: in.Created,
		Architecture: in.Architecture, OS: in.Os, Variant: in.Variant, Size: in.Size,
		Entrypoint: []string{}, Cmd: []string{}, ExposedPorts: []string{}, Labels: map[string]string{},
		LayerCount: len(in.RootFS.Layers), Containers: []model.ImageContainerRef{},
	}
	if c := in.Config; c != nil {
		d.EnvCount = len(c.Env)
		d.Entrypoint = nonNil(c.Entrypoint)
		d.Cmd = nonNil(c.Cmd)
		d.WorkingDir, d.User = c.WorkingDir, c.User
		for p := range c.ExposedPorts {
			d.ExposedPorts = append(d.ExposedPorts, p)
		}
		sort.Strings(d.ExposedPorts)
		if c.Labels != nil {
			d.Labels = c.Labels
		}
	}
	return d
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
