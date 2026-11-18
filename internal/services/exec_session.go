package services

import (
	"context"
	"crypto/rand"
	"docklet/internal/engine"
	"docklet/internal/model"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"
)

// pump reads the stream, batches chunks and emits them in order; at the end it
// reports the exit and removes the session.
func (s *ExecService) pump(ctx context.Context, sess *execSession) {
	defer close(sess.done)
	defer sess.cancel()
	chunks := make(chan []byte, execQueueChunks)
	readErr := make(chan error, 1)

	go func() { // reader
		defer close(chunks)
		buf := make([]byte, execReadBytes)
		for {
			n, err := sess.stream.Read(buf)
			if n > 0 {
				c := make([]byte, n)
				copy(c, buf[:n])
				select {
				case chunks <- c:
				case <-sess.closed:
					readErr <- nil
					return
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					err = nil
				}
				readErr <- err
				return
			}
		}
	}()

	var pending []byte
	flush := func() {
		if len(pending) == 0 {
			return
		}
		s.emit("exec:data", model.ExecDataEvent{SessionID: sess.id, DataBase64: base64.StdEncoding.EncodeToString(pending)})
		pending = pending[:0]
	}
	timer := time.NewTimer(time.Hour)
	timer.Stop()
loop:
	for {
		select {
		case c, ok := <-chunks:
			if !ok {
				break loop
			}
			if len(pending) == 0 {
				timer.Reset(execFlushInterval)
			}
			pending = append(pending, c...)
			if len(pending) >= execFlushBytes {
				timer.Stop()
				flush()
			}
		case <-timer.C:
			flush()
		}
	}
	timer.Stop()
	flush()

	err := <-readErr
	s.remove(sess)
	_ = sess.stream.Close()
	select {
	case <-sess.closed:
		return // explicit Close: the caller knows; no exit event
	default:
	}
	ev := model.ExecExitEvent{SessionID: sess.id}
	ev.Notice = s.releaseSidecar(sess) // the toolbox goes before the UI hears about the end
	if err != nil {
		ev.Error = "The terminal connection was lost: " + engine.Redact(err.Error())
	}
	if code, ok := s.exitCode(ctx, sess); ok {
		ev.ExitCode = &code
	}
	s.emit("exec:exit", ev)
}

// exitCode asks the Engine for the exit code, briefly waiting for the process to settle.
func (s *ExecService) exitCode(ctx context.Context, sess *execSession) (int, bool) {
	for i := 0; i < 10; i++ {
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		code, running, err := sess.api.ExecExit(c, sess.execID)
		cancel()
		if err != nil {
			return 0, false
		}
		if !running {
			return code, true
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return 0, false
		}
	}
	return 0, false
}

func (s *ExecService) emit(name string, data any) {
	if s.Events != nil {
		s.Events.Emit(name, data)
	}
}

func (s *ExecService) remove(sess *execSession) {
	s.mu.Lock()
	if s.sessions[sess.id] == sess {
		delete(s.sessions, sess.id)
	}
	s.mu.Unlock()
}

func (s *ExecService) get(id string) (*execSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("terminal session %q not found (it may have ended)", id)
	}
	return sess, nil
}

func newSessionID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *ExecService) register(connectionID, containerID, execID, shell string, api execAPI, stream execStream, sc *debugSidecar) model.ExecSession {
	ctx, cancel := context.WithCancel(context.Background())
	sess := &execSession{
		id: newSessionID(), connectionID: connectionID, containerID: containerID, execID: execID,
		api: api, stream: stream, cancel: cancel,
		closed: make(chan struct{}), done: make(chan struct{}), sidecar: sc,
	}
	s.mu.Lock()
	if s.sessions == nil {
		s.sessions = map[string]*execSession{}
	}
	s.sessions[sess.id] = sess
	s.mu.Unlock()
	go s.pump(ctx, sess)
	return model.ExecSession{ID: sess.id, Shell: shell}
}
