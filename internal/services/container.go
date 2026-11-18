package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/moby/moby/client"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/registryauth"
)

// ContainerService is bound to the frontend. All methods are scoped by connectionID.
// Batch lifecycle methods never fail as a whole: they return one ActionResult per ID
// (skipped / failed entries carry reasons).
type ContainerService struct {
	Engines engine.Provider
	Events  engine.Emitter
	// Ops runs Recreate in the background (nil disables it).
	Ops *ops.Manager
	// Registry resolves registry credentials for "pull first" (nil = Docker CLI config).
	Registry *registryauth.Resolver

	streams logStreams   // live log streams (zero value ready)
	watches statsWatches // live stats watches (zero value ready)
}

// maxConcurrency bounds parallel Engine calls in batch operations.
const maxConcurrency = 8

// forEachBounded runs run(i) for every i in [0,n) with at most maxConcurrency in flight and waits
// for all of them. Indexes that never got a slot because ctx ended go to cancelled(i) (may be nil).
func forEachBounded(ctx context.Context, n int, run func(i int), cancelled func(i int)) {
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				if cancelled != nil {
					cancelled(i)
				}
				return
			}
			run(i)
		}()
	}
	wg.Wait()
}

func (s *ContainerService) client(ctx context.Context, connectionID string) (*client.Client, error) {
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

// List returns ALL containers (including stopped) of the connection, with raw labels
// for Compose grouping mapped into model.Container. Must not be mocked.
func (s *ContainerService) List(ctx context.Context, connectionID string) ([]model.Container, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	res, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, friendlyError(err)
	}
	out := make([]model.Container, 0, len(res.Items))
	for _, it := range res.Items {
		out = append(out, mapSummary(connectionID, it))
	}
	classifyExited(ctx, cli, connectionID, out, func(i int) bool { return isOneOffLabels(res.Items[i].Labels) })
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ComposeProject != b.ComposeProject {
			return a.ComposeProject < b.ComposeProject
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
	return out, nil
}

func (s *ContainerService) Start(ctx context.Context, connectionID string, ids []string) ([]model.ActionResult, error) {
	return s.lifecycle(ctx, connectionID, ids, actionStart)
}

func (s *ContainerService) Stop(ctx context.Context, connectionID string, ids []string) ([]model.ActionResult, error) {
	return s.lifecycle(ctx, connectionID, ids, actionStop)
}

func (s *ContainerService) Restart(ctx context.Context, connectionID string, ids []string) ([]model.ActionResult, error) {
	return s.lifecycle(ctx, connectionID, ids, actionRestart)
}

// Remove deletes containers. force=true kills running ones. The UI must have confirmed.
func (s *ContainerService) Remove(ctx context.Context, connectionID string, ids []string, force bool) ([]model.ActionResult, error) {
	action := actionRemove
	if force {
		action = actionForceRemove
	}
	return s.lifecycle(ctx, connectionID, ids, action)
}

func shortID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

var errEmptyName = errors.New("container name must not be empty")
