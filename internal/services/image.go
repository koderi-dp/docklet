package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"

	"docklet/internal/compose"
	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/registryauth"
)

// ImageService is bound to the frontend. All methods are scoped by connectionID.
type ImageService struct {
	Engines engine.Provider
	Events  engine.Emitter
	Ops     *ops.Manager
	// Registry resolves registry credentials (nil = Docker CLI config on this machine).
	Registry *registryauth.Resolver
	// updates caches registry update checks (zero value ready).
	updates updateCache

	// Profiles tells builds where the docker CLI runs for a connection (nil disables BuildKit builds).
	Profiles engine.ProfileSource
	// Builds stores the build history (nil disables it).
	Builds BuildHistoryStore

	// runner executes CLI commands; nil means real processes (tests inject a fake).
	runner compose.Runner
	capMu  sync.Mutex
	caps   map[string]capEntry
	live   liveBuilds
}

func (s *ImageService) client(ctx context.Context, connectionID string) (*client.Client, error) {
	if s.Engines == nil {
		return nil, fmt.Errorf("no engine provider configured")
	}
	cli, err := s.Engines.Client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if cli == nil {
		return nil, fmt.Errorf("connection %q is not available", connectionID)
	}
	return cli, nil
}

func (s *ImageService) emitChanged(connectionID string) {
	if s.Events != nil {
		s.Events.Emit(model.EventImagesChanged, model.ImagesChanged{ConnectionID: connectionID})
	}
}

// imageError turns common Engine failures into readable messages.
func imageError(err error) error {
	if err == nil {
		return nil
	}
	low := strings.ToLower(err.Error())
	var prefix string
	switch {
	case errors.Is(err, context.Canceled):
		prefix = "Operation canceled"
	case errors.Is(err, context.DeadlineExceeded):
		prefix = "Operation timed out"
	case cerrdefs.IsNotFound(err), strings.Contains(low, "no such image"), strings.Contains(low, "manifest unknown"),
		strings.Contains(low, "not found"):
		prefix = "Image not found"
	case strings.Contains(low, "unauthorized"), strings.Contains(low, "denied"), strings.Contains(low, "authentication required"):
		prefix = "Registry denied access"
	case strings.Contains(low, "permission denied"):
		prefix = "Permission denied by the Docker Engine"
	}
	if prefix == "" {
		return err
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

// List returns one row per repo:tag plus one per untagged (dangling) image.
func (s *ImageService) List(ctx context.Context, connectionID string) ([]model.ImageRow, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	imgs, err := cli.ImageList(ctx, client.ImageListOptions{})
	if err != nil {
		return nil, imageError(err)
	}
	cs, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, friendlyError(err)
	}
	return buildImageRows(connectionID, imgs.Items, containersByImage(cs.Items)), nil
}

// Inspect returns details of one image (ID or reference).
func (s *ImageService) Inspect(ctx context.Context, connectionID, ref string) (model.ImageDetail, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	ref, err := validateImageRef(ref)
	if err != nil {
		return model.ImageDetail{}, err
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.ImageDetail{}, err
	}
	res, err := cli.ImageInspect(ctx, ref)
	if err != nil {
		return model.ImageDetail{}, imageError(err)
	}
	d := mapImageDetail(res.InspectResponse)
	if cs, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true}); err == nil {
		d.Containers = containersByImage(cs.Items)[res.ID]
	}
	if d.Containers == nil {
		d.Containers = []model.ImageContainerRef{}
	}
	return d, nil
}

// History returns the layers of an image, newest first.
func (s *ImageService) History(ctx context.Context, connectionID, ref string) ([]model.ImageLayer, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	ref, err := validateImageRef(ref)
	if err != nil {
		return nil, err
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	res, err := cli.ImageHistory(ctx, ref)
	if err != nil {
		return nil, imageError(err)
	}
	out := make([]model.ImageLayer, 0, len(res.Items))
	for _, it := range res.Items {
		out = append(out, model.ImageLayer{ID: it.ID, Created: it.Created, CreatedBy: it.CreatedBy, Size: it.Size, Tags: nonNil(it.Tags)})
	}
	return out, nil
}

// Remove removes images/tags one by one. It never fails as a whole once the
// connection resolves: each item has its own ActionResult. The UI must have confirmed.
func (s *ImageService) Remove(ctx context.Context, connectionID string, refs []string, force bool) ([]model.ActionResult, error) {
	if len(refs) == 0 {
		return []model.ActionResult{}, nil
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	results := make([]model.ActionResult, 0, len(refs))
	anyOK := false
	for _, raw := range refs {
		res := model.ActionResult{ID: raw, Name: displayRef(raw)}
		ref, verr := validateImageRef(raw)
		if verr != nil {
			res.Error = verr.Error()
			results = append(results, res)
			continue
		}
		if ctx.Err() != nil {
			res.Error = imageError(ctx.Err()).Error()
			results = append(results, res)
			continue
		}
		_, rerr := cli.ImageRemove(ctx, ref, client.ImageRemoveOptions{Force: force, PruneChildren: true})
		if rerr != nil {
			res.Error = s.removeError(ctx, cli, ref, rerr).Error()
		} else {
			res.OK = true
			anyOK = true
		}
		results = append(results, res)
	}
	if anyOK {
		s.emitChanged(connectionID)
	}
	return results, nil
}

func displayRef(ref string) string {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "sha256:")
	if imageIDRe.MatchString(ref) {
		return shortID(ref)
	}
	return ref
}

// removeError produces the friendly message for a failed removal, naming the
// containers that use the image when the Engine reports a conflict.
func (s *ImageService) removeError(ctx context.Context, cli *client.Client, ref string, err error) error {
	low := strings.ToLower(err.Error())
	switch {
	case cerrdefs.IsConflict(err) && (strings.Contains(low, "being used by") || strings.Contains(low, "is using its referenced image")):
		var users []string
		if ins, ierr := cli.ImageInspect(ctx, ref); ierr == nil {
			if cs, cerr := cli.ContainerList(ctx, client.ContainerListOptions{All: true}); cerr == nil {
				for _, c := range containersByImage(cs.Items)[ins.ID] {
					users = append(users, c.Name)
				}
			}
		}
		if len(users) > 0 {
			noun := "container"
			if len(users) > 1 {
				noun = "containers"
			}
			return fmt.Errorf("image is used by %s %s; remove the %s first", noun, strings.Join(users, ", "), noun)
		}
		return errors.New("image is used by a container; remove the container first")
	case cerrdefs.IsConflict(err) && strings.Contains(low, "child images"):
		return errors.New("image has dependent child images; remove those first")
	case cerrdefs.IsConflict(err) && strings.Contains(low, "multiple repositories"):
		return errors.New("image is tagged in multiple repositories; remove the other tags too or use force")
	case cerrdefs.IsConflict(err):
		return fmt.Errorf("conflict: %w", err)
	}
	return imageError(err)
}

// Tag adds targetRef as a new tag of the source image (ID or reference).
func (s *ImageService) Tag(ctx context.Context, connectionID, source, targetRef string) error {
	source, err := validateImageRef(source)
	if err != nil {
		return err
	}
	target, err := validateTarget(targetRef)
	if err != nil {
		return err
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return err
	}
	if _, err := cli.ImageTag(ctx, client.ImageTagOptions{Source: source, Target: target}); err != nil {
		return imageError(err)
	}
	s.emitChanged(connectionID)
	return nil
}

// PruneDangling removes untagged images that no container uses, as a
// background operation reporting the reclaimed space.
func (s *ImageService) PruneDangling(ctx context.Context, connectionID string) (model.Operation, error) {
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	if s.Ops == nil {
		return model.Operation{}, fmt.Errorf("operations are not available")
	}
	op := s.Ops.Start(ops.Spec{
		Kind: "image.prune", Title: "Remove dangling images", ConnectionID: connectionID, Resource: "dangling images",
	}, func(octx context.Context, h *ops.Handle) error {
		res, err := cli.ImagePrune(octx, client.ImagePruneOptions{Filters: client.Filters{}.Add("dangling", "true")})
		if err != nil {
			return imageError(err)
		}
		for _, d := range res.Report.ImagesDeleted {
			switch {
			case d.Deleted != "":
				h.Log("Deleted " + d.Deleted)
			case d.Untagged != "":
				h.Log("Untagged " + d.Untagged)
			}
		}
		h.Log(fmt.Sprintf("Reclaimed %s (%d entries removed)", humanBytes(int64(res.Report.SpaceReclaimed)), len(res.Report.ImagesDeleted)))
		s.emitChanged(connectionID)
		return nil
	})
	return op, nil
}
