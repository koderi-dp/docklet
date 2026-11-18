package services

import (
	"bufio"
	"bytes"
	"context"
	"docklet/internal/model"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
)

// Logs returns the last `tail` lines with timestamps (non-streaming snapshot).
func (s *ContainerService) Logs(ctx context.Context, connectionID string, id string, tail int) ([]model.LogLine, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if tail <= 0 {
		tail = defaultLogTail
	}
	if tail > maxLogTail {
		tail = maxLogTail
	}
	insp, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, friendlyError(err)
	}
	tty := insp.Container.Config != nil && insp.Container.Config.Tty
	rc, err := cli.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true, ShowStderr: true, Timestamps: true, Tail: strconv.Itoa(tail),
	})
	if err != nil {
		return nil, friendlyError(err)
	}
	defer rc.Close()
	return parseLogs(rc, tty)
}

func parseLogs(r io.Reader, tty bool) ([]model.LogLine, error) {
	lines := []model.LogLine{}
	if tty {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			lines = append(lines, parseLogLine("stdout", sc.Text()))
		}
		return lines, sc.Err()
	}
	out := &logWriter{stream: "stdout", lines: &lines}
	errw := &logWriter{stream: "stderr", lines: &lines}
	_, err := stdcopy.StdCopy(out, errw, r)
	out.flush()
	errw.flush()
	return lines, err
}

// logWriter collects complete lines in arrival order across both streams.
type logWriter struct {
	stream string
	buf    []byte
	lines  *[]model.LogLine
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		*w.lines = append(*w.lines, parseLogLine(w.stream, string(w.buf[:i])))
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

func (w *logWriter) flush() {
	if len(w.buf) > 0 {
		*w.lines = append(*w.lines, parseLogLine(w.stream, string(w.buf)))
		w.buf = nil
	}
}

func parseLogLine(stream, line string) model.LogLine {
	line = strings.TrimSuffix(line, "\r")
	ll := model.LogLine{Stream: stream, Text: line}
	if ts, rest, ok := strings.Cut(line, " "); ok {
		if _, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			ll.Timestamp, ll.Text = ts, rest
		}
	}
	return ll
}
