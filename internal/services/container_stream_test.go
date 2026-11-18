package services

import (
	"context"
	"encoding/binary"
	"net/http"
	"strings"
	"testing"

	"docklet/internal/model"
)

func frame(t byte, s string) []byte {
	h := make([]byte, 8)
	h[0] = t
	binary.BigEndian.PutUint32(h[4:], uint32(len(s)))
	return append(h, s...)
}

// streamServer serves inspect + a logs endpoint that writes one frame, then either
// ends (endAfter=true) or blocks until the client disconnects.
func streamServer(t *testing.T, endAfter bool) (*ContainerService, *recEmitter, *counter) {
	t.Helper()
	open := &counter{}
	cli := dockerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/logs") {
			open.add(1)
			defer open.add(-1)
			w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
			_, _ = w.Write(frame(1, "2024-01-01T00:00:00Z hello\n"))
			_, _ = w.Write(frame(2, "2024-01-01T00:00:01Z oops\n"))
			w.(http.Flusher).Flush()
			if !endAfter {
				<-r.Context().Done()
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":"run","Name":"/run","State":{"Status":"running","Running":true},"Config":{"Tty":false}}`))
	}), "1.45")
	em := &recEmitter{}
	return &ContainerService{Engines: fakeProvider{cli}, Events: em}, em, open
}

// logProgress counts the log lines emitted so far and reports the end event, if any.
func logProgress(em *recEmitter) (n int, ended bool, endErr string) {
	for _, ev := range em.snapshot() {
		switch d := ev.data.(type) {
		case model.ContainerLogsData:
			n += len(d.Lines)
		case model.ContainerLogsEnd:
			ended, endErr = true, d.Error
		}
	}
	return
}

func TestLogStream_BatchesLinesAndStopsOnRequest(t *testing.T) {
	s, em, open := streamServer(t, false)
	id, err := s.StartLogStream(context.Background(), "c1", "run", 10, true)
	if err != nil || id == "" {
		t.Fatalf("start: %q %v", id, err)
	}
	eventually(t, "2 lines", func() bool { n, _, _ := logProgress(em); return n == 2 })
	if s.streams.count() != 1 {
		t.Fatalf("stream not tracked")
	}
	s.StopLogStream(context.Background(), id)
	if s.streams.count() != 0 {
		t.Errorf("stream not cleaned up")
	}
	eventually(t, "server request closed", func() bool { return open.get() == 0 })
	if _, ended, _ := logProgress(em); ended {
		t.Errorf("no end event expected after StopLogStream")
	}
	first := em.snapshot()[0].data.(model.ContainerLogsData)
	if first.StreamID != id || first.Lines[0].Text != "hello" || first.Lines[0].Stream != "stdout" || first.Lines[1].Stream != "stderr" {
		t.Errorf("batch: %+v", first)
	}
	s.StopLogStream(context.Background(), id) // idempotent
}

func TestLogStream_EmitsEndEventWhenEngineClosesStream(t *testing.T) {
	s, em, _ := streamServer(t, true)
	if _, err := s.StartLogStream(context.Background(), "c1", "run", 0, true); err != nil {
		t.Fatal(err)
	}
	eventually(t, "end event", func() bool { _, ended, _ := logProgress(em); return ended })
	if n, _, _ := logProgress(em); n != 2 {
		t.Errorf("lines before end: %d", n)
	}
	eventually(t, "cleanup", func() bool { return s.streams.count() == 0 })
}

func TestStopConnectionLogStreams_ClosesEveryStreamOfThatConnection(t *testing.T) {
	s, _, open := streamServer(t, false)
	for range 3 {
		if _, err := s.StartLogStream(context.Background(), "c1", "run", 1, false); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, "3 open", func() bool { return open.get() == 3 })
	s.StopConnectionLogStreams("c1")
	eventually(t, "all closed", func() bool { return s.streams.count() == 0 && open.get() == 0 })
}
