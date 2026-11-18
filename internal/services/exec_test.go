package services

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"docklet/internal/model"
)

type pipeStream struct {
	r      *io.PipeReader
	w      *io.PipeWriter // output written by the "container"
	mu     sync.Mutex
	in     []byte
	closed bool
}

func newPipeStream() *pipeStream {
	r, w := io.Pipe()
	return &pipeStream{r: r, w: w}
}

func (p *pipeStream) Read(b []byte) (int, error) { return p.r.Read(b) }

func (p *pipeStream) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.in = append(p.in, b...)
	return len(b), nil
}

func (p *pipeStream) written() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return string(p.in)
}

func (p *pipeStream) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	_ = p.r.Close()
	return p.w.Close()
}

type fakeExecAPI struct {
	running   bool
	missing   map[string]bool // shells whose attach fails
	tried     []string
	stream    *pipeStream
	exitCode  int
	resizedTo [2]int
}

func (f *fakeExecAPI) ContainerRunning(context.Context, string) (bool, error) { return f.running, nil }

func (f *fakeExecAPI) ExecCreate(_ context.Context, _ string, shell []string, _ model.ExecOptions) (string, error) {
	f.tried = append(f.tried, shell[0])
	return "exec-" + shell[0], nil
}

func (f *fakeExecAPI) ExecAttach(_ context.Context, id string, _, _ int) (execStream, error) {
	if f.missing[strings.TrimPrefix(id, "exec-")] {
		return nil, errors.New("OCI runtime exec failed: executable file not found")
	}
	return f.stream, nil
}

func (f *fakeExecAPI) ExecResize(_ context.Context, _ string, c, r int) error {
	f.resizedTo = [2]int{c, r}
	return nil
}

func (f *fakeExecAPI) ExecExit(context.Context, string) (int, bool, error) {
	return f.exitCode, false, nil
}

func newExecSvc(api *fakeExecAPI) (*ExecService, *recEmitter) {
	em := &recEmitter{}
	return &ExecService{Events: em, newAPI: func(context.Context, string) (execAPI, error) { return api, nil }}, em
}

func (s *ExecService) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func TestExecStart_FallsBackFromBashToShAndRejectsUnsafeShell(t *testing.T) {
	api := &fakeExecAPI{running: true, missing: map[string]bool{"/bin/bash": true}, stream: newPipeStream()}
	s, _ := newExecSvc(api)
	sess, err := s.Start(context.Background(), "c1", "ctr", model.ExecOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background(), sess.ID)
	if sess.Shell != "/bin/sh" || strings.Join(api.tried, ",") != "/bin/bash,/bin/sh" {
		t.Fatalf("shell=%s tried=%v", sess.Shell, api.tried)
	}

	api2 := &fakeExecAPI{running: true, missing: map[string]bool{"/bin/bash": true, "/bin/sh": true}, stream: newPipeStream()}
	s2, _ := newExecSvc(api2)
	if _, err := s2.Start(context.Background(), "c1", "ctr", model.ExecOptions{}); err == nil || !strings.Contains(err.Error(), "No usable shell") {
		t.Fatalf("want no-shell error, got %v", err)
	}
	if _, err := s2.Start(context.Background(), "c1", "ctr", model.ExecOptions{Shell: "bash; rm -rf /"}); err == nil {
		t.Fatal("unsafe shell must be rejected")
	}
}

func TestExecStart_RefusesStoppedContainer(t *testing.T) {
	s, _ := newExecSvc(&fakeExecAPI{running: false})
	_, err := s.Start(context.Background(), "c1", "ctr", model.ExecOptions{})
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("got %v", err)
	}
}

func TestExec_ForwardsInputAndResizeToTheContainer(t *testing.T) {
	ctx := context.Background()
	api := &fakeExecAPI{running: true, stream: newPipeStream()}
	s, _ := newExecSvc(api)
	sess, err := s.Start(ctx, "c1", "ctr", model.ExecOptions{Shell: "sh", Cols: 100, Rows: 30})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(ctx, sess.ID)
	if err := s.Write(ctx, sess.ID, base64.StdEncoding.EncodeToString([]byte("ls\n"))); err != nil {
		t.Fatal(err)
	}
	if got := api.stream.written(); got != "ls\n" {
		t.Fatalf("stdin = %q", got)
	}
	if err := s.Resize(ctx, sess.ID, 120, 40); err != nil || api.resizedTo != [2]int{120, 40} {
		t.Fatalf("resize: %v %v", err, api.resizedTo)
	}
}

func TestExec_StreamsOrderedOutputThenReportsExitAndCleansUp(t *testing.T) {
	ctx := context.Background()
	api := &fakeExecAPI{running: true, stream: newPipeStream(), exitCode: 3}
	s, em := newExecSvc(api)
	sess, err := s.Start(ctx, "c1", "ctr", model.ExecOptions{Shell: "sh"})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = api.stream.w.Write([]byte("he"))
		_, _ = api.stream.w.Write([]byte("llo"))
		_, _ = api.stream.w.Write([]byte(" world"))
		_ = api.stream.w.Close() // process exit
	}()
	eventually(t, "exit event", func() bool { return len(em.find("exec:exit")) == 1 })
	var got strings.Builder
	for _, d := range em.find("exec:data") {
		b, _ := base64.StdEncoding.DecodeString(d.(model.ExecDataEvent).DataBase64)
		got.Write(b)
	}
	if got.String() != "hello world" {
		t.Fatalf("output = %q", got.String())
	}
	ex := em.find("exec:exit")[0].(model.ExecExitEvent)
	if ex.ExitCode == nil || *ex.ExitCode != 3 || ex.Error != "" {
		t.Fatalf("exit = %+v", ex)
	}
	eventually(t, "session cleanup", func() bool { return s.count() == 0 })
	if err := s.Write(ctx, sess.ID, "AA=="); err == nil {
		t.Fatal("write after exit must fail")
	}
}

func TestExecClose_ReleasesStreamAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	api1 := &fakeExecAPI{running: true, stream: newPipeStream()}
	api2 := &fakeExecAPI{running: true, stream: newPipeStream()}
	s, em := newExecSvc(api1)
	s.newAPI = func(_ context.Context, id string) (execAPI, error) {
		if id == "c2" {
			return api2, nil
		}
		return api1, nil
	}
	a, _ := s.Start(ctx, "c1", "ctr", model.ExecOptions{})
	b, _ := s.Start(ctx, "c2", "ctr", model.ExecOptions{})
	if s.count() != 2 {
		t.Fatalf("want 2 concurrent sessions, got %d", s.count())
	}
	if err := s.Close(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if !api1.stream.closed || s.count() != 1 {
		t.Fatalf("closed=%v count=%d", api1.stream.closed, s.count())
	}
	if err := s.Close(ctx, a.ID); err != nil {
		t.Fatal("second close must be a no-op")
	}
	// broken stream on the other one => exit event with error, session removed
	api2.stream.r.CloseWithError(errors.New("connection reset"))
	eventually(t, "exit after stream break", func() bool { return s.count() == 0 })
	eventually(t, "exit event", func() bool { return len(em.find("exec:exit")) == 1 })
	ex := em.find("exec:exit")[0].(model.ExecExitEvent)
	if ex.SessionID != b.ID || ex.Error == "" {
		t.Fatalf("exit = %+v", ex)
	}
}

func TestExec_UnknownSessionFailsExceptClose(t *testing.T) {
	s, _ := newExecSvc(&fakeExecAPI{})
	ctx := context.Background()
	if err := s.Write(ctx, "nope", "AA=="); err == nil {
		t.Fatal("write must fail")
	}
	if err := s.Resize(ctx, "nope", 80, 24); err == nil {
		t.Fatal("resize must fail")
	}
	if err := s.Close(ctx, "nope"); err != nil {
		t.Fatalf("close unknown should be no-op: %v", err)
	}
}
