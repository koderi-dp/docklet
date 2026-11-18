package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/wsldocker"
	"docklet/internal/wslinfo"
)

// wslDeps are the pieces tests replace.
type wslDeps struct {
	exec   wsldocker.Exec
	reader wslinfo.HostReader
	fake   bool // reader is deliberately set (possibly nil)
}

func (s *WSLService) collector() wslinfo.Collector {
	ex, rd := s.deps.exec, s.deps.reader
	if ex == nil {
		ex = wsldocker.NewExec()
	}
	if rd == nil && !s.deps.fake {
		rd = wslinfo.NewHostReader()
	}
	return wslinfo.Collector{Exec: ex, Host: rd}
}

// wslRun carries what one action's operation needs.
type wslRun struct {
	s            *WSLService
	host         wslHost
	connectionID string
	info         model.WSLInfo
}

func (r *wslRun) exec() wsldocker.Exec { return r.s.collector().Exec }

func (r *wslRun) steps(ctx context.Context, h *ops.Handle, steps []wsldocker.Step) error {
	return wsldocker.Run(ctx, r.exec(), r.info.Distro, steps, wsldocker.Reporter{Log: h.Log})
}

// reconnect drops the engine connection and opens it again, so streams that died with the daemon start over.
func (r *wslRun) reconnect(ctx context.Context, h *ops.Handle) error {
	h.Log("Reconnecting the connection to the Docker Engine...")
	_ = r.host.Disconnect(ctx, r.connectionID)
	st, err := r.host.Connect(ctx, r.connectionID)
	if err != nil {
		return fmt.Errorf("reconnecting: %w", err)
	}
	if st.State != model.StateConnected {
		return fmt.Errorf("the connection did not come back: %s", st.Error)
	}
	h.Log("Connected.")
	return nil
}

func (r *wslRun) reclaim(ctx context.Context, h *ops.Handle) error {
	before := r.memory(ctx)
	h.Log("Before: " + describeMemory(before))
	step := wsldocker.Step{Title: "Drop the page cache", Cmd: wslinfo.ReclaimCommand(), Timeout: 30 * time.Second}
	if err := r.steps(ctx, h, []wsldocker.Step{step}); err != nil {
		return err
	}
	after := r.memory(ctx)
	h.Log("After: " + describeMemory(after))
	if before.Cache != nil && after.Cache != nil {
		h.Detail(fmt.Sprintf("Cache %s to %s", formatBytesGo(*before.Cache), formatBytesGo(*after.Cache)))
	}
	return nil
}

// memory reads /proc/meminfo; unknown fields stay nil.
func (r *wslRun) memory(ctx context.Context) wslinfo.Memory {
	var lines []string
	_ = r.exec().Run(ctx, r.info.Distro, wsldocker.Cmd{Argv: []string{"cat", "/proc/meminfo"}}, func(l string) { lines = append(lines, l) })
	return wslinfo.ParseMeminfo(strings.Join(lines, "\n"))
}

func (r *wslRun) restartDocker(ctx context.Context, h *ops.Handle) error {
	systemd := r.info.SystemdRunning != nil && *r.info.SystemdRunning
	steps := []wsldocker.Step{
		{Title: "Restart the Docker service", Cmd: wslinfo.RestartDockerCommand(systemd), Timeout: 2 * time.Minute},
		{Title: "Wait for Docker to answer", Cmd: wsldocker.Cmd{Argv: []string{"docker", "version", "--format", "{{.Server.Version}}"}, Root: true},
			Timeout: 30 * time.Second, Wait: 30 * time.Second},
	}
	if err := r.steps(ctx, h, steps); err != nil {
		return err
	}
	return r.reconnect(ctx, h)
}

func (r *wslRun) restartDistro(ctx context.Context, h *ops.Handle) error {
	name := r.info.Distro
	_ = r.host.Disconnect(ctx, r.connectionID)
	term := wsldocker.Step{Title: "Stop " + name, Cmd: wslinfo.TerminateCommand(name), Timeout: time.Minute}
	if err := r.steps(ctx, h, []wsldocker.Step{term}); err != nil {
		return err
	}
	h.Log("Starting " + name + "...")
	if err := r.host.StartWSLDistro(ctx, name); err != nil {
		return err
	}
	// Only a stopped Docker Engine service is started. Docker Desktop's integration and a
	// distribution without Docker have nothing to start here.
	if f, err := wsldocker.Detect(ctx, r.exec(), name); err == nil && f.State() == model.WSLStateDaemonStopped {
		h.Log("Starting the Docker service...")
		if _, err := wsldocker.Start(ctx, r.exec(), name); err != nil {
			return fmt.Errorf("%s restarted, but Docker did not come back: %w", name, err)
		}
	}
	return r.reconnect(ctx, h)
}

func (r *wslRun) shutdown(ctx context.Context, h *ops.Handle) error {
	if len(r.info.OtherRunning) > 0 {
		h.Log("Also stopping: " + strings.Join(r.info.OtherRunning, ", "))
	}
	_ = r.host.Disconnect(ctx, r.connectionID)
	step := wsldocker.Step{Title: "Shut down the WSL virtual machine", Cmd: wslinfo.ShutdownCommand(), Timeout: time.Minute}
	if err := r.steps(ctx, h, []wsldocker.Step{step}); err != nil {
		return err
	}
	h.Detail("All distributions stopped")
	h.Log("WSL is stopped. Connect again to start " + r.info.Distro + ".")
	return nil
}

func describeMemory(m wslinfo.Memory) string {
	f := func(p *int64) string {
		if p == nil {
			return "unknown"
		}
		return formatBytesGo(*p)
	}
	return fmt.Sprintf("used %s of %s, buff/cache %s", f(m.Used), f(m.Total), f(m.Cache))
}

func formatBytesGo(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 4; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}
