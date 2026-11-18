package services

import (
	"context"
	"sort"

	"github.com/moby/moby/client"

	"docklet/internal/model"
)

// maxBaseCandidates caps how many other local images are inspected when
// looking for a shared base.
const maxBaseCandidates = 40

// Layers explains where an image's size comes from: the Engine history joined
// with inspect data, plus text-based hints and a shared-base hint. Sizes are
// exactly what the Engine reports.
func (s *ImageService) Layers(ctx context.Context, connectionID, ref string) (model.ImageLayersReport, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	ref, err := validateImageRef(ref)
	if err != nil {
		return model.ImageLayersReport{}, err
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.ImageLayersReport{}, err
	}
	ins, err := cli.ImageInspect(ctx, ref)
	if err != nil {
		return model.ImageLayersReport{}, imageError(err)
	}
	hist, err := cli.ImageHistory(ctx, ref)
	if err != nil {
		return model.ImageLayersReport{}, imageError(err)
	}
	items := make([]model.ImageLayer, 0, len(hist.Items))
	for _, it := range hist.Items {
		items = append(items, model.ImageLayer{ID: it.ID, Created: it.Created, CreatedBy: it.CreatedBy, Size: it.Size})
	}
	rep := buildLayersReport(ins.ID, ins.Size, items)
	rep.Architecture, rep.OS, rep.Created = ins.Architecture, ins.Os, ins.Created
	rep.RootFSLayers = len(ins.RootFS.Layers)
	rep.SharedBase = s.findSharedBase(ctx, cli, ins.ID, ins.RootFS.Layers)
	return rep, nil
}

// buildLayersReport maps history entries (newest first) to build-ordered steps.
func buildLayersReport(imageID string, totalSize int64, newestFirst []model.ImageLayer) model.ImageLayersReport {
	rep := model.ImageLayersReport{ImageID: imageID, TotalSize: totalSize, Steps: []model.ImageLayerStep{}, Hints: []model.ImageLayerHint{}}
	n := len(newestFirst)
	for i := n - 1; i >= 0; i-- {
		l := newestFirst[i]
		kind, instr, cmd := parseCreatedBy(l.CreatedBy)
		size := l.Size
		if size < 0 {
			size = 0
		}
		rep.Steps = append(rep.Steps, model.ImageLayerStep{
			Index: n - i, ID: l.ID, Kind: kind, Instruction: instr, Command: cmd, Size: size, Created: l.Created,
		})
		rep.HistorySize += size
	}
	var largest int64
	for i := range rep.Steps {
		st := &rep.Steps[i]
		if st.Size > 0 {
			rep.ContentSteps++
			if rep.HistorySize > 0 {
				st.Share = float64(st.Size) / float64(rep.HistorySize)
			}
			if st.Size > largest {
				largest, rep.LargestStep = st.Size, st.Index
			}
		}
	}
	rep.Hints = layerHints(rep.Steps)
	return rep
}

type baseCandidate struct {
	ref    string
	id     string
	layers []string
}

// bestSharedBase returns the candidate sharing the longest leading run of layer
// digests with mine (nil when none shares even the first layer). Ties go to the
// alphabetically first ref so the result is stable.
func bestSharedBase(mine []string, cands []baseCandidate) *model.ImageSharedBase {
	var best *model.ImageSharedBase
	for _, c := range cands {
		n := 0
		for n < len(mine) && n < len(c.layers) && mine[n] == c.layers[n] {
			n++
		}
		if n == 0 {
			continue
		}
		if best == nil || n > best.Count || (n == best.Count && c.ref < best.Ref) {
			best = &model.ImageSharedBase{Count: n, Ref: c.ref, ImageID: c.id}
		}
	}
	return best
}

// findSharedBase compares the leading RootFS layers with other local images. It
// is best effort: any failure yields nil.
func (s *ImageService) findSharedBase(ctx context.Context, cli *client.Client, selfID string, mine []string) *model.ImageSharedBase {
	if len(mine) == 0 {
		return nil
	}
	list, err := cli.ImageList(ctx, client.ImageListOptions{})
	if err != nil {
		return nil
	}
	imgs := list.Items
	sort.SliceStable(imgs, func(i, j int) bool { return imgs[i].ID < imgs[j].ID })
	var cands []baseCandidate
	for _, im := range imgs {
		if im.ID == selfID {
			continue
		}
		if len(cands) >= maxBaseCandidates || ctx.Err() != nil {
			break
		}
		res, err := cli.ImageInspect(ctx, im.ID)
		if err != nil || len(res.RootFS.Layers) == 0 {
			continue
		}
		name := shortID(im.ID)
		for _, t := range im.RepoTags {
			if t != "" && t != "<none>:<none>" {
				name = t
				break
			}
		}
		cands = append(cands, baseCandidate{ref: name, id: im.ID, layers: res.RootFS.Layers})
	}
	return bestSharedBase(mine, cands)
}
