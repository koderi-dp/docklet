package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"

	"docklet/internal/model"
)

const (
	streamFlushInterval = 100 * time.Millisecond
	streamBatchLines    = 200
	maxStreamTail       = 10000
)

// logStream is one live log follower.
type logStream struct {
	connectionID string
	cancel       context.CancelFunc
	done         chan struct{}
	stopped      chan struct{} // closed by StopLogStream so no "end" event is emitted
	stopOnce     sync.Once
}

// logStreams tracks live streams. The zero value is ready to use.
type logStreams struct {
	mu sync.Mutex
	m  map[string]*logStream
}

func (r *logStreams) add(id string, st *logStream) {
	r.mu.Lock()
	if r.m == nil {
		r.m = map[string]*logStream{}
	}
	r.m[id] = st
	r.mu.Unlock()
}

func (r *logStreams) remove(id string) {
	r.mu.Lock()
	delete(r.m, id)
	r.mu.Unlock()
}

func (r *logStreams) get(id string) *logStream {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.m[id]
}

func (r *logStreams) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.m)
}

func (r *logStreams) forConnection(connectionID string) []*logStream {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*logStream
	for _, st := range r.m {
		if st.connectionID == connectionID {
			out = append(out, st)
		}
	}
	return out
}

func (st *logStream) stop() {
	st.stopOnce.Do(func() { close(st.stopped); st.cancel() })
}

func newStreamID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "ls-" + hex.EncodeToString(b)
}

// StartLogStream follows a container's logs. tail<0 means all existing lines,
// tail==0 the default. Lines arrive as "containerlogs:data" batches; when the
// stream ends by itself "containerlogs:end" is emitted.
func (s *ContainerService) StartLogStream(ctx context.Context, connectionID string, id string, tail int, timestamps bool) (string, error) {
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return "", err
	}
	insp, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return "", friendlyError(err)
	}
	tty := insp.Container.Config != nil && insp.Container.Config.Tty

	tailArg := "all"
	switch {
	case tail == 0:
		tailArg = strconv.Itoa(defaultLogTail)
	case tail > 0:
		if tail > maxStreamTail {
			tail = maxStreamTail
		}
		tailArg = strconv.Itoa(tail)
	}

	// The request ctx ends when this call returns; the stream needs its own.
	sctx, cancel := context.WithCancel(context.Background())
	rc, err := cli.ContainerLogs(sctx, id, client.ContainerLogsOptions{
		ShowStdout: true, ShowStderr: true, Timestamps: timestamps, Follow: true, Tail: tailArg,
	})
	if err != nil {
		cancel()
		return "", friendlyError(err)
	}
	streamID := newStreamID()
	st := &logStream{connectionID: connectionID, cancel: cancel, done: make(chan struct{}), stopped: make(chan struct{})}
	s.streams.add(streamID, st)
	go s.runLogStream(streamID, st, rc, tty, timestamps)
	return streamID, nil
}

// StopLogStream cancels a stream and waits (bounded) for its goroutine to exit.
// Unknown IDs are ignored.
func (s *ContainerService) StopLogStream(_ context.Context, streamID string) {
	st := s.streams.get(streamID)
	if st == nil {
		return
	}
	st.stop()
	select {
	case <-st.done:
	case <-time.After(2 * time.Second):
	}
}

// StopConnectionLogStreams cancels all streams of one connection (for when its client errors/closes).
func (s *ContainerService) StopConnectionLogStreams(connectionID string) {
	for _, st := range s.streams.forConnection(connectionID) {
		st.stop()
	}
}

// stopAll cancels every live stream (service shutdown).
func (r *logStreams) stopAll() {
	r.mu.Lock()
	all := make([]*logStream, 0, len(r.m))
	for _, st := range r.m {
		all = append(all, st)
	}
	r.mu.Unlock()
	for _, st := range all {
		st.stop()
	}
}

type streamSink struct {
	mu      sync.Mutex
	pending []model.LogLine
	kick    chan struct{}
}

func (k *streamSink) push(l model.LogLine) {
	k.mu.Lock()
	k.pending = append(k.pending, l)
	n := len(k.pending)
	k.mu.Unlock()
	if n >= streamBatchLines {
		select {
		case k.kick <- struct{}{}:
		default:
		}
	}
}

func (k *streamSink) take() []model.LogLine {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := k.pending
	k.pending = nil
	return out
}

// lineWriter splits writes into lines for one stream.
type lineWriter struct {
	stream     string
	timestamps bool
	sink       *streamSink
	buf        []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.emit(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	if len(w.buf) > 0 {
		w.emit(string(w.buf))
		w.buf = nil
	}
}

func (w *lineWriter) emit(line string) {
	if w.timestamps {
		w.sink.push(parseLogLine(w.stream, line))
		return
	}
	w.sink.push(model.LogLine{Stream: w.stream, Text: strings.TrimSuffix(line, "\r")})
}

func (s *ContainerService) runLogStream(streamID string, st *logStream, rc io.ReadCloser, tty, timestamps bool) {
	defer close(st.done)
	defer s.streams.remove(streamID)
	defer st.cancel()

	sink := &streamSink{kick: make(chan struct{}, 1)}
	outW := &lineWriter{stream: "stdout", timestamps: timestamps, sink: sink}
	errW := &lineWriter{stream: "stderr", timestamps: timestamps, sink: sink}
	copyDone := make(chan error, 1)
	go func() {
		defer rc.Close()
		var err error
		if tty {
			_, err = io.Copy(outW, rc)
		} else {
			_, err = stdcopy.StdCopy(outW, errW, rc)
		}
		outW.flush()
		errW.flush()
		copyDone <- err
	}()

	flush := func() {
		lines := sink.take()
		for len(lines) > 0 && !isStopped(st) {
			n := min(len(lines), streamBatchLines)
			if s.Events != nil {
				s.Events.Emit(model.EventContainerLogsData, model.ContainerLogsData{StreamID: streamID, Lines: lines[:n]})
			}
			lines = lines[n:]
		}
	}
	tick := time.NewTicker(streamFlushInterval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			flush()
		case <-sink.kick:
			flush()
		case err := <-copyDone:
			flush()
			if isStopped(st) {
				return
			}
			end := model.ContainerLogsEnd{StreamID: streamID}
			if err != nil && !errors.Is(err, context.Canceled) {
				end.Error = friendlyError(err).Error()
			}
			if s.Events != nil {
				s.Events.Emit(model.EventContainerLogsEnd, end)
			}
			return
		}
	}
}

func isStopped(st *logStream) bool {
	select {
	case <-st.stopped:
		return true
	default:
		return false
	}
}
