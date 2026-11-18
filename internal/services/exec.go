package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
)

// ExecService is bound to the frontend: interactive TTY exec sessions.
type ExecService struct {
	Engines engine.Provider
	Events  engine.Emitter
	Ops     *ops.Manager
	// Profiles resolves connection profiles for OpenExternalTerminal.
	Profiles engine.ProfileSource

	// newAPI is a test seam; nil uses the Docker SDK client of the connection.
	newAPI func(ctx context.Context, connectionID string) (execAPI, error)

	mu       sync.Mutex
	sessions map[string]*execSession
}

// execStream is the bidirectional TTY stream of an attached exec.
type execStream interface {
	io.Reader
	io.Writer
	Close() error
}

// execAPI is the narrow part of the Engine API used by terminals.
type execAPI interface {
	ContainerRunning(ctx context.Context, containerID string) (bool, error)
	ExecCreate(ctx context.Context, containerID string, shell []string, o model.ExecOptions) (string, error)
	ExecAttach(ctx context.Context, execID string, cols, rows int) (execStream, error)
	ExecResize(ctx context.Context, execID string, cols, rows int) error
	// ExecExit returns the exit code once the process has finished (running=true otherwise).
	ExecExit(ctx context.Context, execID string) (code int, running bool, err error)
}

const (
	execFlushInterval = 12 * time.Millisecond
	execFlushBytes    = 32 * 1024
	execReadBytes     = 16 * 1024
	execQueueChunks   = 64 // reader blocks when the emitter lags: backpressure
	maxExecWrite      = 1 << 20
)

type execSession struct {
	id           string
	connectionID string
	containerID  string
	execID       string
	api          execAPI
	stream       execStream

	wmu    sync.Mutex // serialises writes
	cancel context.CancelFunc
	once   sync.Once
	closed chan struct{} // closed by explicit Close
	done   chan struct{} // closed when the pump has finished
}

var shellPaths = map[string]string{
	"bash": "/bin/bash", "sh": "/bin/sh", "ash": "/bin/ash", "zsh": "/bin/zsh",
}

var absShellRe = regexp.MustCompile(`^/[A-Za-z0-9_./-]{1,200}$`)

// shellCandidates returns the argv[0] candidates to try, in order.
func shellCandidates(shell string) ([]string, error) {
	s := strings.TrimSpace(shell)
	if s == "" || s == "auto" {
		return []string{"/bin/bash", "/bin/sh"}, nil
	}
	if p, ok := shellPaths[s]; ok {
		return []string{p}, nil
	}
	if absShellRe.MatchString(s) && !strings.Contains(s, "..") {
		return []string{s}, nil
	}
	return nil, fmt.Errorf("unsupported shell %q (use auto, bash, sh, ash, zsh or an absolute path)", shell)
}

func (s *ExecService) api(ctx context.Context, connectionID string) (execAPI, error) {
	if s.newAPI != nil {
		return s.newAPI(ctx, connectionID)
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
	return sdkExecAPI{cli}, nil
}

// Start opens an interactive TTY session. Output is emitted as "exec:data" and the
// end as "exec:exit".
func (s *ExecService) Start(ctx context.Context, connectionID, containerID string, opts model.ExecOptions) (model.ExecSession, error) {
	if strings.TrimSpace(containerID) == "" {
		return model.ExecSession{}, errors.New("container is required")
	}
	cands, err := shellCandidates(opts.Shell)
	if err != nil {
		return model.ExecSession{}, err
	}
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.ExecSession{}, err
	}
	running, err := api.ContainerRunning(ctx, containerID)
	if err != nil {
		return model.ExecSession{}, friendlyError(err)
	}
	if !running {
		return model.ExecSession{}, errors.New("The container is not running. Start it to open a terminal.")
	}
	if opts.Cols <= 0 {
		opts.Cols = 80
	}
	if opts.Rows <= 0 {
		opts.Rows = 24
	}

	var lastErr error
	for i, shell := range cands {
		execID, err := api.ExecCreate(ctx, containerID, []string{shell}, opts)
		if err != nil {
			return model.ExecSession{}, friendlyError(err)
		}
		stream, err := api.ExecAttach(ctx, execID, opts.Cols, opts.Rows)
		if err != nil {
			lastErr = err
			continue // shell probably missing in the image: try the next candidate
		}
		if i < len(cands)-1 && shellMissing(ctx, api, execID) {
			// Some engines accept the attach and only then report the missing
			// binary (exit 126/127): fall back to the next candidate.
			_ = stream.Close()
			lastErr = fmt.Errorf("%s is not available in this container", shell)
			continue
		}
		return s.register(connectionID, containerID, execID, shell, api, stream), nil
	}
	if len(cands) > 1 {
		return model.ExecSession{}, fmt.Errorf("No usable shell found in the container (tried %s): %w", strings.Join(cands, ", "), friendlyError(lastErr))
	}
	return model.ExecSession{}, fmt.Errorf("Could not start %s in the container: %w", cands[0], friendlyError(lastErr))
}

// Write sends base64-encoded input (keystrokes, paste) to the session.
func (s *ExecService) Write(ctx context.Context, sessionID, dataBase64 string) error {
	sess, err := s.get(sessionID)
	if err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(dataBase64)
	if err != nil {
		return fmt.Errorf("invalid input encoding: %w", err)
	}
	if len(data) > maxExecWrite {
		return errors.New("input too large")
	}
	sess.wmu.Lock()
	defer sess.wmu.Unlock()
	if _, err := sess.stream.Write(data); err != nil {
		return fmt.Errorf("could not write to the terminal: %w", err)
	}
	return nil
}

// Resize propagates the terminal size to the Engine.
func (s *ExecService) Resize(ctx context.Context, sessionID string, cols, rows int) error {
	sess, err := s.get(sessionID)
	if err != nil {
		return err
	}
	if cols <= 0 || rows <= 0 || cols > 1000 || rows > 1000 {
		return fmt.Errorf("invalid terminal size %dx%d", cols, rows)
	}
	return friendlyError(sess.api.ExecResize(ctx, sess.execID, cols, rows))
}

// Close ends a session and releases its resources. Unknown sessions are a no-op
// (they may already have exited).
func (s *ExecService) Close(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	sess := s.sessions[sessionID]
	s.mu.Unlock()
	if sess == nil {
		return nil
	}
	s.shutdown(sess)
	return nil
}

func (s *ExecService) shutdown(sess *execSession) {
	sess.once.Do(func() {
		close(sess.closed)
		// The Engine keeps an interactive exec alive after its connection drops, so ask
		// the shell to leave (Ctrl-C, then exit) before closing; otherwise every closed
		// terminal leaves a /bin/sh behind in the container.
		sess.wmu.Lock()
		_, _ = sess.stream.Write([]byte("\x03exit\n"))
		sess.wmu.Unlock()
		_ = sess.stream.Close()
		sess.cancel()
	})
	s.remove(sess)
	<-sess.done
}

// ServiceShutdown closes every open session when the app quits.
func (s *ExecService) ServiceShutdown() error {
	s.mu.Lock()
	all := make([]*execSession, 0, len(s.sessions))
	for _, x := range s.sessions {
		all = append(all, x)
	}
	s.mu.Unlock()
	for _, x := range all {
		s.shutdown(x)
	}
	return nil
}

// shellMissing briefly watches a freshly attached exec and reports whether it
// died immediately with "command not found / not executable".
func shellMissing(ctx context.Context, api execAPI, execID string) bool {
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		code, running, err := api.ExecExit(ctx, execID)
		if err == nil && !running && (code == 126 || code == 127) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(50 * time.Millisecond):
		}
	}
	return false
}
