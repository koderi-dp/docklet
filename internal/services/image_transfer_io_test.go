package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestProgressReader_ReportsFractionOfFileSizeAndPassesDataThrough(t *testing.T) {
	s := &sink{}
	pr := &progressReader{ctx: context.Background(), r: strings.NewReader("0123456789"), total: 10, h: s}
	buf := make([]byte, 4)
	n, err := pr.Read(buf)
	if n != 4 || err != nil || string(buf) != "0123" {
		t.Fatalf("n=%d err=%v buf=%q", n, err, buf)
	}
	if len(s.progress) != 1 || s.progress[0] != 0.4 {
		t.Errorf("progress = %v, want [0.4]", s.progress)
	}
	if s.detail != "Uploaded 4 B / 10 B" {
		t.Errorf("detail = %q", s.detail)
	}
	// Reads inside the 200 ms window are not reported again.
	if _, err := pr.Read(buf); err != nil || len(s.progress) != 1 {
		t.Errorf("err=%v progress=%v", err, s.progress)
	}
	rest, _ := io.ReadAll(pr)
	if string(rest) != "89" {
		t.Errorf("remaining data %q", rest)
	}
}

func TestProgressReader_UnknownTotalNeverReportsAndCancelledContextStopsReading(t *testing.T) {
	s := &sink{}
	pr := &progressReader{ctx: context.Background(), r: strings.NewReader("abc"), total: 0, h: s}
	if _, err := io.ReadAll(pr); err != nil {
		t.Fatal(err)
	}
	if len(s.progress) != 0 || s.detail != "" {
		t.Errorf("total 0 must not divide or report: %v %q", s.progress, s.detail)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := strings.NewReader("abc")
	pr = &progressReader{ctx: ctx, r: src, total: 3, h: s}
	if n, err := pr.Read(make([]byte, 3)); n != 0 || !errors.Is(err, context.Canceled) {
		t.Errorf("n=%d err=%v", n, err)
	}
	if src.Len() != 3 {
		t.Error("a cancelled upload must not consume the file")
	}
}

func TestCountingWriter_CountsBytesAndThrottlesDetail(t *testing.T) {
	s := &sink{}
	var out bytes.Buffer
	cw := &countingWriter{w: &out, h: s, prefix: "Written "}
	if _, err := cw.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if s.detail != "Written 5 B" {
		t.Errorf("detail = %q", s.detail)
	}
	s.detail = ""
	if _, err := cw.Write([]byte("!!")); err != nil {
		t.Fatal(err)
	}
	if s.detail != "" {
		t.Errorf("second write within 200 ms must not update detail, got %q", s.detail)
	}
	cw.last = time.Now().Add(-time.Second)
	_, _ = cw.Write([]byte("?"))
	if cw.n != 8 || s.detail != "Written 8 B" || out.String() != "hello!!?" {
		t.Errorf("n=%d detail=%q out=%q", cw.n, s.detail, out.String())
	}
}
