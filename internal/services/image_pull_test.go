package services

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
)

func TestConsumePull_AggregatesLayerProgressMonotonically(t *testing.T) {
	a, b, c := "aaaaaaaaaaaa", "bbbbbbbbbbbb", "cccccccccccc"
	stream := []jsonstream.Message{
		{Status: "Pulling from library/x", ID: "latest"},
		dl(a, "Pulling fs layer", 0, 0),
		dl(b, "Pulling fs layer", 0, 0),
		dl(c, "Already exists", 0, 0),
		dl(a, "Waiting", 0, 0),
		dl(b, "Downloading", 0, 300), // a's total still unknown: no progress yet
		dl(a, "Downloading", 0, 100),
		dl(a, "Downloading", 50, 100),
		dl(b, "Downloading", 150, 300),
		dl(a, "Downloading", 100, 100),
		dl(a, "Download complete", 0, 0),
		dl(b, "Downloading", 300, 300),
		dl(b, "Download complete", 0, 0),
		dl(a, "Extracting", 0, 0),
		dl(a, "Pull complete", 0, 0),
		dl(b, "Pull complete", 0, 0),
		{Status: "Digest: sha256:abc"},
		{Status: "Status: Downloaded newer image for x:latest"},
	}
	s := &sink{}
	if err := consumePull(context.Background(), seq(stream), s); err != nil {
		t.Fatal(err)
	}
	if len(s.progress) == 0 {
		t.Fatal("no progress reported")
	}
	// First reported value happens only after both layer totals are known: (0+0)/400.
	if s.progress[0] != 0 {
		t.Errorf("first progress = %v, want 0", s.progress[0])
	}
	prev := -1.0
	for _, p := range s.progress {
		if p < prev-1e-9 {
			t.Errorf("progress went backwards: %v after %v", p, prev)
		}
		prev = p
	}
	if math.Abs(s.progress[len(s.progress)-1]-1) > 1e-9 {
		t.Errorf("final progress = %v, want 1", s.progress[len(s.progress)-1])
	}
	// 200/400 = 0.5 must have been seen (a at 50/100, b at 150/300).
	seen := false
	for _, p := range s.progress {
		if math.Abs(p-0.5) < 1e-9 {
			seen = true
		}
	}
	if !seen {
		t.Errorf("expected 0.5 aggregate, got %v", s.progress)
	}
	joined := strings.Join(s.logs, "\n")
	for _, want := range []string{a + ": Pulling fs layer", c + ": Already exists", "Digest: sha256:abc", b + ": Pull complete"} {
		if !strings.Contains(joined, want) {
			t.Errorf("log missing %q in:\n%s", want, joined)
		}
	}
	if n := strings.Count(joined, b+": Downloading"); n != 1 {
		t.Errorf("per-tick progress must not be logged repeatedly, got %d", n)
	}
}

func TestConsumePull_LeavesProgressUnsetWithoutTotals(t *testing.T) {
	a := "aaaaaaaaaaaa"
	s := &sink{}
	err := consumePull(context.Background(), seq([]jsonstream.Message{dl(a, "Pulling fs layer", 0, 0), dl(a, "Downloading", 10, 0), dl(a, "Pull complete", 0, 0)}), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.progress) != 0 {
		t.Errorf("progress must stay unset without totals, got %v", s.progress)
	}
}

func TestConsumePull_SurfacesRegistryErrorAndCancellation(t *testing.T) {
	s := &sink{}
	err := consumePull(context.Background(), seq([]jsonstream.Message{{Error: &jsonstream.Error{Message: "pull access denied for x, repository does not exist"}}}), s)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("want denied error, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = consumePull(ctx, func(yield func(jsonstream.Message, error) bool) {
		yield(jsonstream.Message{}, errors.New("read: context canceled"))
	}, s)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel must return ctx error, got %v", err)
	}
}
