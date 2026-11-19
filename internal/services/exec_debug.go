package services

import (
	"context"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
)

// Debug shells: a temporary toolbox container that joins a running container's
// PID, network (and, when the engine allows, IPC) namespaces. It is always
// labelled so it can be told apart from user containers, and always removed
// when its terminal session ends.

const (
	debugLabel        = "dev.docklet.debug"
	debugTargetLabel  = "dev.docklet.debug.target"
	debugDefaultImage = "busybox:latest"
	debugStaleAge     = 5 * time.Minute
	debugSweepEvery   = 2 * time.Minute
	debugRemoveWait   = 20 * time.Second
)

// debugTarget is what a debug shell needs to know about the container it joins.
type debugTarget struct {
	ID      string
	Name    string
	Running bool
	Paused  bool
	// IPCMode is the target's HostConfig.IpcMode ("private", "shareable", "host", ...).
	IPCMode string
}

// debugInfo describes a labelled sidecar found on the engine.
type debugInfo struct {
	ID      string
	Created int64 // unix seconds
	Labels  map[string]string
}

// debugAPI is the narrow part of the Engine API used by debug shells.
type debugAPI interface {
	EngineOS(ctx context.Context) (string, error)
	InspectTarget(ctx context.Context, id string) (debugTarget, error)
	ImageExists(ctx context.Context, ref string) (bool, error)
	CreateSidecar(ctx context.Context, name string, cfg *container.Config, host *container.HostConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string) error
	ListDebug(ctx context.Context) ([]debugInfo, error)
	// HasRunningExec reports whether a process started with exec (a shell session) is still running in the container.
	HasRunningExec(ctx context.Context, id string) (bool, error)
}

// debugSidecar is a live sidecar owned by one session.
type debugSidecar struct {
	connectionID string
	id           string
	name         string
	targetID     string
	targetName   string
	api          debugAPI

	once   sync.Once
	notice string
}

// debugState is the per-service bookkeeping of sidecars.
type debugState struct {
	mu    sync.Mutex
	live  map[string]struct{} // connectionID/sidecarID of sessions alive in this process
	swept map[string]time.Time
}

func debugKey(connectionID, id string) string { return connectionID + "/" + id }

func (d *debugState) add(connectionID, id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.live == nil {
		d.live = map[string]struct{}{}
	}
	d.live[debugKey(connectionID, id)] = struct{}{}
}

func (d *debugState) del(connectionID, id string) {
	d.mu.Lock()
	delete(d.live, debugKey(connectionID, id))
	d.mu.Unlock()
}

func (d *debugState) isLive(connectionID, id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.live[debugKey(connectionID, id)]
	return ok
}

// debugSpecOptions are the choices that shape the sidecar.
type debugSpecOptions struct {
	Image     string
	ShareIPC  bool
	SysPtrace bool
}

// buildDebugSpec returns the sidecar's container config. It joins the target's
// namespaces, is removed by the engine when it exits, and never gets extra
// privileges beyond the optional SYS_PTRACE capability.
func buildDebugSpec(targetID string, o debugSpecOptions) (*container.Config, *container.HostConfig) {
	join := container.NetworkMode("container:" + targetID)
	host := &container.HostConfig{
		PidMode:     container.PidMode("container:" + targetID),
		NetworkMode: join,
		AutoRemove:  true,
	}
	if o.ShareIPC {
		host.IpcMode = container.IpcMode("container:" + targetID)
	}
	if o.SysPtrace {
		host.CapAdd = []string{"SYS_PTRACE"}
	}
	cfg := &container.Config{
		Image:      o.Image,
		Entrypoint: []string{},
		// The shell is an exec in this container, so the main process only keeps it alive.
		Cmd:       []string{"sleep", "2147483647"},
		Tty:       true,
		OpenStdin: true,
		Labels:    map[string]string{debugLabel: "true", debugTargetLabel: targetID},
	}
	return cfg, host
}

// staleDebugContainers picks the sidecars to sweep: carrying our label, older
// than maxAge, not owned by a live session of this process and not in use by
// anyone else. inUse is the engine's view (a shell is still running in the
// sidecar), which is how a session of another Docklet on the same engine is
// recognised; it is asked last, and only for candidates, because it costs a
// round trip per container.
func staleDebugContainers(list []debugInfo, live, inUse func(id string) bool, now time.Time, maxAge time.Duration) []string {
	cutoff := now.Add(-maxAge).Unix()
	var out []string
	for _, c := range list {
		if c.Labels[debugLabel] != "true" || c.Created > cutoff || live(c.ID) || inUse(c.ID) {
			continue
		}
		out = append(out, c.ID)
	}
	return out
}
