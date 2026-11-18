package services

import (
	"context"

	"github.com/moby/moby/client"

	"docklet/internal/model"
)

type containerAction int

const (
	actionStart containerAction = iota
	actionStop
	actionRestart
	actionRemove
	actionForceRemove
	actionPause
	actionUnpause
	actionKill
)

// skipReason returns a non-empty reason when the action is not applicable.
func (a containerAction) skipReason(running bool) string {
	switch a {
	case actionStart:
		if running {
			return "Already running"
		}
	case actionStop:
		if !running {
			return "Not running"
		}
	case actionPause, actionKill, actionUnpause:
		if !running {
			return "Not running"
		}
	case actionRemove:
		if running {
			return "Container is running; stop it first or use force remove"
		}
	}
	return ""
}

// lifecycle runs one action per ID with bounded concurrency. It never fails as a
// whole once the connection resolves; per-container problems land in the result.
func (s *ContainerService) lifecycle(ctx context.Context, connectionID string, ids []string, action containerAction) ([]model.ActionResult, error) {
	if len(ids) == 0 {
		return []model.ActionResult{}, nil
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	results := make([]model.ActionResult, len(ids))
	forEachBounded(ctx, len(ids), func(i int) {
		results[i] = runOne(ctx, cli, ids[i], action)
	}, func(i int) {
		results[i] = model.ActionResult{ID: ids[i], Name: shortID(ids[i]), Error: friendlyError(ctx.Err()).Error()}
	})

	anyOK := false
	for _, r := range results {
		if r.OK {
			anyOK = true
			break
		}
	}
	if anyOK && s.Events != nil {
		s.Events.Emit(model.EventContainersChanged, model.ContainersChanged{ConnectionID: connectionID})
	}
	return results, nil
}

func runOne(ctx context.Context, cli *client.Client, id string, action containerAction) model.ActionResult {
	res := model.ActionResult{ID: id, Name: shortID(id)}
	insp, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		res.Error = friendlyError(err).Error()
		return res
	}
	if n := trimName(insp.Container.Name); n != "" {
		res.Name = n
	}
	running := insp.Container.State != nil && (insp.Container.State.Running || insp.Container.State.Restarting)
	paused := insp.Container.State != nil && insp.Container.State.Paused
	reason := action.skipReason(running)
	if reason == "" && action == actionUnpause && !paused {
		reason = "Not paused"
	}
	if reason == "" && action == actionPause && paused {
		reason = "Already paused"
	}
	if reason != "" {
		res.Skipped = true
		if action == actionRemove {
			res.Error = reason
		}
		return res
	}
	switch action {
	case actionStart:
		_, err = cli.ContainerStart(ctx, id, client.ContainerStartOptions{})
	case actionStop:
		_, err = cli.ContainerStop(ctx, id, client.ContainerStopOptions{})
	case actionRestart:
		_, err = cli.ContainerRestart(ctx, id, client.ContainerRestartOptions{})
	case actionPause:
		_, err = cli.ContainerPause(ctx, id, client.ContainerPauseOptions{})
	case actionUnpause:
		_, err = cli.ContainerUnpause(ctx, id, client.ContainerUnpauseOptions{})
	case actionKill:
		_, err = cli.ContainerKill(ctx, id, client.ContainerKillOptions{})
	case actionRemove:
		_, err = cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{})
	case actionForceRemove:
		_, err = cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})
	}
	if err != nil {
		res.Error = friendlyError(err).Error()
		return res
	}
	res.OK = true
	return res
}
