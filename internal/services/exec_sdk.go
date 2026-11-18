package services

import (
	"context"
	"docklet/internal/model"

	"github.com/moby/moby/client"
)

// sdkExecAPI adapts the Docker SDK client.
type sdkExecAPI struct{ cli *client.Client }

func (a sdkExecAPI) ContainerRunning(ctx context.Context, id string) (bool, error) {
	r, err := a.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return false, err
	}
	st := r.Container.State
	return st != nil && st.Running && !st.Paused, nil
}

func (a sdkExecAPI) ExecCreate(ctx context.Context, id string, shell []string, o model.ExecOptions) (string, error) {
	r, err := a.cli.ExecCreate(ctx, id, client.ExecCreateOptions{
		User: o.User, WorkingDir: o.WorkDir, TTY: true,
		AttachStdin: true, AttachStdout: true, AttachStderr: true,
		Env: []string{"TERM=xterm-256color"}, Cmd: shell,
		ConsoleSize: client.ConsoleSize{Height: uint(o.Rows), Width: uint(o.Cols)},
	})
	return r.ID, err
}

func (a sdkExecAPI) ExecAttach(ctx context.Context, execID string, cols, rows int) (execStream, error) {
	r, err := a.cli.ExecAttach(ctx, execID, client.ExecAttachOptions{
		TTY: true, ConsoleSize: client.ConsoleSize{Height: uint(rows), Width: uint(cols)},
	})
	if err != nil {
		return nil, err
	}
	return &hijackedStream{r.HijackedResponse}, nil
}

func (a sdkExecAPI) ExecResize(ctx context.Context, execID string, cols, rows int) error {
	_, err := a.cli.ExecResize(ctx, execID, client.ExecResizeOptions{Height: uint(rows), Width: uint(cols)})
	return err
}

func (a sdkExecAPI) ExecExit(ctx context.Context, execID string) (int, bool, error) {
	r, err := a.cli.ExecInspect(ctx, execID, client.ExecInspectOptions{})
	return r.ExitCode, r.Running, err
}

type hijackedStream struct{ h client.HijackedResponse }

func (s *hijackedStream) Read(p []byte) (int, error) { return s.h.Reader.Read(p) }

func (s *hijackedStream) Write(p []byte) (int, error) { return s.h.Conn.Write(p) }

func (s *hijackedStream) Close() error { s.h.Close(); return nil }
