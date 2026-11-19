package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"docklet/internal/model"
)

func (s *ExecService) debugAPIFor(ctx context.Context, connectionID string) (debugAPI, error) {
	if s.newDebugAPI != nil {
		return s.newDebugAPI(ctx, connectionID)
	}
	if s.Engines == nil {
		return nil, errors.New("no engine provider configured")
	}
	cli, err := s.Engines.Client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if cli == nil {
		return nil, fmt.Errorf("connection %q is not available", connectionID)
	}
	return sdkDebugAPI{cli}, nil
}

func debugSidecarName(targetID string) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return "docklet-debug-" + shortID(targetID) + "-" + hex.EncodeToString(b)
}

// DebugShellStart creates a temporary toolbox container that shares the target's
// namespaces and opens a terminal session in it. The toolbox is removed when the
// session ends. It never pulls: when the image is missing the result says so
// (ImageMissing) and nothing is created.
func (s *ExecService) DebugShellStart(ctx context.Context, connectionID, containerID string, opts model.DebugShellOptions) (model.DebugShellResult, error) {
	if strings.TrimSpace(containerID) == "" {
		return model.DebugShellResult{}, errors.New("container is required")
	}
	image := strings.TrimSpace(opts.Image)
	if image == "" {
		image = debugDefaultImage
	}
	image, err := validateRepoTag(image)
	if err != nil {
		return model.DebugShellResult{}, err
	}
	api, err := s.debugAPIFor(ctx, connectionID)
	if err != nil {
		return model.DebugShellResult{}, err
	}
	if osType, err := api.EngineOS(ctx); err == nil && osType == "windows" {
		return model.DebugShellResult{}, errors.New("Debug shells are not supported on Windows container engines.")
	}
	target, err := api.InspectTarget(ctx, containerID)
	if err != nil {
		return model.DebugShellResult{}, friendlyError(err)
	}
	if !target.Running || target.Paused {
		return model.DebugShellResult{}, errors.New("The container is not running. Start it to open a debug shell.")
	}
	res := model.DebugShellResult{Image: image, Notices: []string{}}
	ok, err := api.ImageExists(ctx, image)
	if err != nil {
		return res, friendlyError(err)
	}
	if !ok {
		res.ImageMissing = true
		return res, nil
	}
	s.sweepDebug(ctx, connectionID, api, false)

	so := debugSpecOptions{Image: image, ShareIPC: target.IPCMode == "shareable", SysPtrace: opts.SysPtrace}
	if !so.ShareIPC {
		res.Notices = append(res.Notices, fmt.Sprintf("IPC namespace not shared: the container's IPC mode is %q (it must be shareable).", orDefault(target.IPCMode, "private")))
	}
	name := debugSidecarName(target.ID)
	id, shared, err := s.createSidecar(ctx, api, name, target.ID, so, &res)
	if err != nil {
		if isMissingImage(err) {
			res.ImageMissing = true
			return res, nil
		}
		return res, friendlyError(err)
	}
	sc := &debugSidecar{connectionID: connectionID, id: id, name: name, targetID: target.ID, targetName: target.Name, api: api}
	s.debug.add(connectionID, id)
	if err := api.StartContainer(ctx, id); err != nil {
		s.finishSidecar(sc)
		return res, fmt.Errorf("The toolbox container could not start (the image needs a `sleep` command): %w", friendlyError(err))
	}
	sess, err := s.startSession(ctx, connectionID, id, model.ExecOptions{Cols: opts.Cols, Rows: opts.Rows}, sc)
	if err != nil {
		s.finishSidecar(sc)
		return res, err
	}
	res.SessionID, res.Shell, res.SidecarName = sess.ID, sess.Shell, name
	res.SharesIPC = shared
	return res, nil
}

// createSidecar creates the toolbox, retrying without IPC sharing when the engine refuses it.
func (s *ExecService) createSidecar(ctx context.Context, api debugAPI, name, targetID string, so debugSpecOptions, res *model.DebugShellResult) (string, bool, error) {
	cfg, host := buildDebugSpec(targetID, so)
	id, err := api.CreateSidecar(ctx, name, cfg, host)
	if err != nil && so.ShareIPC && strings.Contains(strings.ToLower(err.Error()), "ipc") {
		so.ShareIPC = false
		res.Notices = append(res.Notices, "IPC namespace not shared: the Engine refused to join it.")
		cfg, host = buildDebugSpec(targetID, so)
		id, err = api.CreateSidecar(ctx, name, cfg, host)
	}
	return id, so.ShareIPC, err
}

func isMissingImage(err error) bool {
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "no such image") || strings.Contains(low, "image") && strings.Contains(low, "not found")
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// releaseSidecar removes the sidecar of an ended session (once) and returns the
// notice for the UI when the end was caused by the target stopping.
func (s *ExecService) releaseSidecar(sess *execSession) string {
	if sess.sidecar == nil {
		return ""
	}
	return s.finishSidecar(sess.sidecar)
}

func (s *ExecService) finishSidecar(sc *debugSidecar) string {
	sc.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), debugRemoveWait)
		defer cancel()
		if t, err := sc.api.InspectTarget(ctx, sc.targetID); err != nil || !t.Running {
			sc.notice = fmt.Sprintf("%s stopped, so the debug shell ended and its toolbox was removed.", orDefault(sc.targetName, "The container"))
		}
		_ = sc.api.RemoveContainer(ctx, sc.id)
		s.debug.del(sc.connectionID, sc.id)
	})
	return sc.notice
}

// SweepDebugContainers removes leftover toolbox containers (our label only) that
// are older than a few minutes, belong to no live session of this process and
// have no shell running in them (which would be another Docklet's session).
// It returns how many were removed. Calls are throttled per connection.
func (s *ExecService) SweepDebugContainers(ctx context.Context, connectionID string) (int, error) {
	api, err := s.debugAPIFor(ctx, connectionID)
	if err != nil {
		return 0, err
	}
	return s.sweepDebug(ctx, connectionID, api, false), nil
}

func (s *ExecService) sweepDebug(ctx context.Context, connectionID string, api debugAPI, force bool) int {
	s.debug.mu.Lock()
	if s.debug.swept == nil {
		s.debug.swept = map[string]time.Time{}
	}
	if last, ok := s.debug.swept[connectionID]; ok && !force && time.Since(last) < debugSweepEvery {
		s.debug.mu.Unlock()
		return 0
	}
	s.debug.swept[connectionID] = time.Now()
	s.debug.mu.Unlock()

	list, err := api.ListDebug(ctx)
	if err != nil {
		return 0
	}
	n := 0
	live := func(id string) bool { return s.debug.isLive(connectionID, id) }
	// When the engine cannot say, keep the container: removing one in use ends someone's session.
	inUse := func(id string) bool {
		busy, err := api.HasRunningExec(ctx, id)
		return err != nil || busy
	}
	for _, id := range staleDebugContainers(list, live, inUse, time.Now(), debugStaleAge) {
		if api.RemoveContainer(ctx, id) == nil {
			n++
		}
	}
	return n
}
