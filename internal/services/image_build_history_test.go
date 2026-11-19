package services

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"docklet/internal/buildkit"
	"docklet/internal/model"
)

// bhStore is an in-memory BuildHistoryStore that records the calls it receives.
type bhStore struct {
	mu       sync.Mutex
	inserted []model.BuildRecord
	listed   []string
	deleted  [][2]string
	cleared  []string
	records  []model.BuildRecord
	err      error
}

func (f *bhStore) InsertBuild(_ context.Context, r model.BuildRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.inserted = append(f.inserted, r)
	return nil
}

func (f *bhStore) ListBuilds(_ context.Context, id string) ([]model.BuildRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listed = append(f.listed, id)
	return f.records, f.err
}

func (f *bhStore) DeleteBuild(_ context.Context, conn, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, [2]string{conn, id})
	return f.err
}

func (f *bhStore) ClearBuilds(_ context.Context, conn string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleared = append(f.cleared, conn)
	return f.err
}

func TestTailBuffer_KeepsOnlyTheLastLinesInOrder(t *testing.T) {
	var b tailBuffer
	if got := b.Lines(); len(got) != 0 {
		t.Fatalf("empty buffer: %v", got)
	}
	for i := 0; i < logTailLines; i++ {
		b.Add(fmt.Sprintf("l%d", i))
	}
	if got := b.Lines(); len(got) != logTailLines || got[0] != "l0" {
		t.Fatalf("at the cap nothing is dropped: len=%d first=%q", len(got), got[0])
	}
	b.Add("extra1")
	b.Add("extra2")
	got := b.Lines()
	if len(got) != logTailLines {
		t.Fatalf("len = %d, want %d", len(got), logTailLines)
	}
	if got[0] != "l2" || got[len(got)-2] != "extra1" || got[len(got)-1] != "extra2" {
		t.Errorf("oldest lines must drop first: first=%q tail=%v", got[0], got[len(got)-2:])
	}
}

func TestTailBuffer_LinesReturnsACopy(t *testing.T) {
	var b tailBuffer
	b.Add("a")
	got := b.Lines()
	got[0] = "changed"
	if b.Lines()[0] != "a" {
		t.Error("callers must not be able to modify the buffer")
	}
}

func TestSaveBuild_StoresScrubbedRecordAndNeverFailsTheBuild(t *testing.T) {
	meta := buildMeta{
		connectionID: "c1", started: time.Now().Add(-time.Second), tags: []string{"app:1"},
		hide: buildkit.NewScrubber([]string{"s3cr3t-value"}, 4),
	}
	store := &bhStore{}
	s := &ImageService{Builds: store}
	var logged []string
	s.saveBuild(meta, buildOutcome{err: errors.New("auth failed with s3cr3t-value"), tail: []string{"token s3cr3t-value"}}, func(l string) { logged = append(logged, l) })
	if len(store.inserted) != 1 || len(logged) != 0 {
		t.Fatalf("inserted=%d logged=%v", len(store.inserted), logged)
	}
	r := store.inserted[0]
	if r.ConnectionID != "c1" || r.Result != model.BuildFailed || r.ID == "" {
		t.Errorf("record = %+v", r)
	}
	if strings.Contains(r.Error, "s3cr3t-value") || strings.Contains(strings.Join(r.LogTail, ""), "s3cr3t-value") {
		t.Errorf("argument value reached the history: %q %v", r.Error, r.LogTail)
	}

	// A failing store is reported on the operation log, and tolerates a missing logger.
	store.err = errors.New("disk full")
	s.saveBuild(meta, buildOutcome{}, func(l string) { logged = append(logged, l) })
	if len(logged) != 1 || !strings.Contains(logged[0], "disk full") {
		t.Errorf("logged = %v", logged)
	}
	s.saveBuild(meta, buildOutcome{}, nil)

	// No store configured: silently nothing.
	(&ImageService{}).saveBuild(meta, buildOutcome{}, func(l string) { t.Errorf("unexpected log %q", l) })
}

func TestBuildHistory_ListsRemovesAndClearsPerConnection(t *testing.T) {
	ctx := context.Background()
	store := &bhStore{records: []model.BuildRecord{{ID: "b2"}, {ID: "b1"}}}
	s := &ImageService{Builds: store}

	got, err := s.BuildHistory(ctx, "c1")
	if err != nil || len(got) != 2 || got[0].ID != "b2" || !reflect.DeepEqual(store.listed, []string{"c1"}) {
		t.Errorf("got=%v err=%v listed=%v", got, err, store.listed)
	}
	if err := s.RemoveBuild(ctx, "c1", "b1"); err != nil || !reflect.DeepEqual(store.deleted, [][2]string{{"c1", "b1"}}) {
		t.Errorf("err=%v deleted=%v", err, store.deleted)
	}
	if err := s.ClearBuildHistory(ctx, "c2"); err != nil || !reflect.DeepEqual(store.cleared, []string{"c2"}) {
		t.Errorf("err=%v cleared=%v", err, store.cleared)
	}

	store.err = errors.New("db locked")
	if _, err := s.BuildHistory(ctx, "c1"); err == nil {
		t.Error("list error must propagate")
	}
	if err := s.RemoveBuild(ctx, "c1", "b1"); err == nil {
		t.Error("remove error must propagate")
	}
	if err := s.ClearBuildHistory(ctx, "c1"); err == nil {
		t.Error("clear error must propagate")
	}
}

func TestBuildHistory_WithoutStoreListsEmptyAndRefusesChanges(t *testing.T) {
	s := &ImageService{}
	got, err := s.BuildHistory(context.Background(), "c1")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("want empty non-nil list (the UI renders it as JSON []), got %#v err=%v", got, err)
	}
	if err := s.RemoveBuild(context.Background(), "c1", "x"); err == nil {
		t.Error("RemoveBuild without a store must fail")
	}
	if err := s.ClearBuildHistory(context.Background(), "c1"); err == nil {
		t.Error("ClearBuildHistory without a store must fail")
	}
}

func TestLiveBuilds_TracksRunningStateAndReturnsSnapshots(t *testing.T) {
	var l liveBuilds
	if _, ok := l.get("none"); ok {
		t.Error("empty registry must not find anything")
	}
	l.finish("none") // unknown id: no panic

	tr := buildkit.NewTracker(nil)
	l.add("op1", tr)
	b, ok := l.get("op1")
	if !ok || !b.running || b.tracker != tr {
		t.Fatalf("got %+v ok=%v", b, ok)
	}
	l.finish("op1")
	if b2, _ := l.get("op1"); b2.running {
		t.Error("finished build must report running=false")
	}
	if !b.running {
		t.Error("an earlier snapshot must not change")
	}
	b2, _ := l.get("op1")
	b2.running = true
	if b3, _ := l.get("op1"); b3.running {
		t.Error("modifying a returned value must not change the registry")
	}
}

func TestLiveBuilds_KeepsOnlyTheMostRecentTrackers(t *testing.T) {
	var l liveBuilds
	for i := 0; i < maxLiveTrackers+3; i++ {
		l.add(fmt.Sprintf("op%d", i), buildkit.NewTracker(nil))
	}
	for i := 0; i < 3; i++ {
		if _, ok := l.get(fmt.Sprintf("op%d", i)); ok {
			t.Errorf("op%d should have been evicted", i)
		}
	}
	for i := 3; i < maxLiveTrackers+3; i++ {
		if _, ok := l.get(fmt.Sprintf("op%d", i)); !ok {
			t.Errorf("op%d must still be known", i)
		}
	}
	if len(l.byOp) != maxLiveTrackers || len(l.order) != maxLiveTrackers {
		t.Errorf("registry size map=%d order=%d", len(l.byOp), len(l.order))
	}
}

func TestBuildProgress_ReturnsStepsOfKnownOperationsOnly(t *testing.T) {
	s := &ImageService{}
	if _, err := s.BuildProgress(context.Background(), "op-x"); err == nil || !strings.Contains(err.Error(), "op-x") {
		t.Fatalf("unknown operation must error naming it, got %v", err)
	}

	tr := buildkit.NewTracker(nil)
	tr.Feed(`{"vertexes":[{"digest":"sha256:aa","name":"[1/2] FROM alpine","started":"2026-01-01T00:00:00Z","completed":"2026-01-01T00:00:01Z"},` +
		`{"digest":"sha256:bb","name":"[2/2] RUN make","started":"2026-01-01T00:00:01Z"}]}`)
	s.live.add("op1", tr)
	p, err := s.BuildProgress(context.Background(), "op1")
	if err != nil {
		t.Fatal(err)
	}
	if p.OperationID != "op1" || !p.Running || p.Total != 2 || p.Done != 1 {
		t.Errorf("progress = %+v", p)
	}
	s.live.finish("op1")
	if p, _ = s.BuildProgress(context.Background(), "op1"); p.Running {
		t.Error("finished operation must report running=false")
	}
}

func TestSortedArgNames_ReturnsNamesOnlyInStableOrder(t *testing.T) {
	got := sortedArgNames(map[string]string{"ZED": "secret", "ALPHA": "", "MID": "x"})
	if !reflect.DeepEqual(got, []string{"ALPHA", "MID", "ZED"}) {
		t.Errorf("got %v", got)
	}
	if got := sortedArgNames(nil); got == nil || len(got) != 0 {
		t.Errorf("nil map must give an empty non-nil slice, got %#v", got)
	}
}

func TestStepSummary(t *testing.T) {
	cases := []struct {
		done, total, cached int
		current, want       string
	}{
		{0, 0, 0, "", "0 of 0 steps"},
		{5, 9, 0, "", "5 of 9 steps"},
		{5, 9, 2, "", "5 of 9 steps - 2 cached"},
		{5, 9, 0, "  RUN make  ", "5 of 9 steps - RUN make"},
		{5, 9, 2, "RUN make", "5 of 9 steps - 2 cached - RUN make"},
		{5, 9, 0, "   ", "5 of 9 steps"},
	}
	for _, c := range cases {
		if got := stepSummary(c.done, c.total, c.cached, c.current); got != c.want {
			t.Errorf("stepSummary(%d,%d,%d,%q) = %q, want %q", c.done, c.total, c.cached, c.current, got, c.want)
		}
	}
}

func TestJoinPath_UsesTheSeparatorStyleOfTheDirectory(t *testing.T) {
	cases := []struct{ dir, rel, want string }{
		{"/home/me/app", "Dockerfile", "/home/me/app/Dockerfile"},
		{"/home/me/app/", "sub/Dockerfile", "/home/me/app/sub/Dockerfile"},
		{"/", "Dockerfile", "/Dockerfile"},
		{`C:\src\app`, "Dockerfile", `C:\src\app\Dockerfile`},
		{`C:\src\app\`, "sub/dir/Dockerfile", `C:\src\app\sub\dir\Dockerfile`},
		{`\\wsl$\Ubuntu\home\me`, "a/b", `\\wsl$\Ubuntu\home\me\a\b`},
	}
	for _, c := range cases {
		if got := joinPath(c.dir, c.rel); got != c.want {
			t.Errorf("joinPath(%q,%q) = %q, want %q", c.dir, c.rel, got, c.want)
		}
	}
}
