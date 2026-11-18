package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"docklet/internal/buildkit"
	"docklet/internal/model"
)

// BuildHistoryStore keeps the builds started from Docklet (local app database).
type BuildHistoryStore interface {
	InsertBuild(ctx context.Context, r model.BuildRecord) error
	ListBuilds(ctx context.Context, connectionID string) ([]model.BuildRecord, error)
	DeleteBuild(ctx context.Context, connectionID, id string) error
	ClearBuilds(ctx context.Context, connectionID string) error
}

const (
	logTailLines    = 200
	maxLiveTrackers = 20
)

// buildMeta is what is known about a build when it starts. It never holds argument values.
type buildMeta struct {
	connectionID string
	builder      string
	method       string
	contextDir   string
	dockerfile   string
	tags         []string
	target       string
	platform     string
	noCache      bool
	pull         bool
	argNames     []string
	command      string
	started      time.Time
	// hide removes every argument value from stored text.
	hide *buildkit.Scrubber
}

func newBuildID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// tailBuffer keeps the last lines of a build's log.
type tailBuffer struct {
	mu    sync.Mutex
	lines []string
}

func (t *tailBuffer) Add(l string) {
	t.mu.Lock()
	t.lines = append(t.lines, l)
	if len(t.lines) > logTailLines {
		t.lines = t.lines[len(t.lines)-logTailLines:]
	}
	t.mu.Unlock()
}

func (t *tailBuffer) Lines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.lines...)
}

// buildOutcome is what a finished build contributes to its record.
type buildOutcome struct {
	err         error
	cancelled   bool
	imageID     string
	steps       []model.BuildStep
	stepsTotal  int
	stepsCached int
	tail        []string
}

// makeRecord assembles a history record. Anything that could contain an argument value is scrubbed.
func (m buildMeta) makeRecord(o buildOutcome) model.BuildRecord {
	now := time.Now()
	r := model.BuildRecord{
		ID: newBuildID(), ConnectionID: m.connectionID, StartedAt: m.started.UnixMilli(), FinishedAt: now.UnixMilli(),
		DurationMs: now.Sub(m.started).Milliseconds(), Result: model.BuildSuccess, Builder: m.builder, Method: m.method,
		ContextDir: m.contextDir, Dockerfile: m.dockerfile, Tags: append([]string{}, m.tags...), Target: m.target,
		Platform: m.platform, NoCache: m.noCache, Pull: m.pull, ArgNames: append([]string{}, m.argNames...),
		ImageID: o.imageID, StepsTotal: o.stepsTotal, StepsCached: o.stepsCached, Command: m.command,
	}
	switch {
	case o.cancelled:
		r.Result = model.BuildCancelled
	case o.err != nil:
		r.Result = model.BuildFailed
		r.Error = m.hide.Apply(o.err.Error())
		for _, l := range o.tail {
			r.LogTail = append(r.LogTail, m.hide.Apply(l))
		}
	}
	for _, st := range o.steps {
		st.Name, st.Error = m.hide.Apply(st.Name), m.hide.Apply(st.Error)
		if st.State != model.StepError {
			st.Log = nil // only the failing step keeps its output
		} else {
			for i := range st.Log {
				st.Log[i] = m.hide.Apply(st.Log[i])
			}
		}
		r.Steps = append(r.Steps, st)
	}
	return r
}

// saveBuild writes a record; a failure is logged to the operation, never fails the build.
func (s *ImageService) saveBuild(m buildMeta, o buildOutcome, logf func(string)) {
	if s.Builds == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Builds.InsertBuild(ctx, m.makeRecord(o)); err != nil && logf != nil {
		logf("Could not save this build to the history: " + err.Error())
	}
}

// BuildHistory lists the builds started from Docklet on this computer for one connection, newest first.
func (s *ImageService) BuildHistory(ctx context.Context, connectionID string) ([]model.BuildRecord, error) {
	if s.Builds == nil {
		return []model.BuildRecord{}, nil
	}
	return s.Builds.ListBuilds(ctx, connectionID)
}

// RemoveBuild deletes one history record.
func (s *ImageService) RemoveBuild(ctx context.Context, connectionID, id string) error {
	if s.Builds == nil {
		return errors.New("build history is not available")
	}
	return s.Builds.DeleteBuild(ctx, connectionID, id)
}

// ClearBuildHistory deletes every history record of one connection.
func (s *ImageService) ClearBuildHistory(ctx context.Context, connectionID string) error {
	if s.Builds == nil {
		return errors.New("build history is not available")
	}
	return s.Builds.ClearBuilds(ctx, connectionID)
}

// liveBuilds remembers the trackers of recent CLI builds so the UI can poll their step lists.
type liveBuilds struct {
	mu    sync.Mutex
	byOp  map[string]*liveBuild
	order []string
}

type liveBuild struct {
	tracker *buildkit.Tracker
	running bool
}

func (l *liveBuilds) add(opID string, t *buildkit.Tracker) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byOp == nil {
		l.byOp = map[string]*liveBuild{}
	}
	l.byOp[opID] = &liveBuild{tracker: t, running: true}
	l.order = append(l.order, opID)
	for len(l.order) > maxLiveTrackers {
		delete(l.byOp, l.order[0])
		l.order = l.order[1:]
	}
}

func (l *liveBuilds) finish(opID string) {
	l.mu.Lock()
	if b := l.byOp[opID]; b != nil {
		b.running = false
	}
	l.mu.Unlock()
}

func (l *liveBuilds) get(opID string) (*liveBuild, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.byOp[opID]
	if !ok {
		return nil, false
	}
	return &liveBuild{tracker: b.tracker, running: b.running}, true
}

// BuildProgress returns the step list of a build operation started in this session. Builds through the
// Engine API (Classic builder) have no steps and report an error.
func (s *ImageService) BuildProgress(ctx context.Context, operationID string) (model.BuildProgress, error) {
	b, ok := s.live.get(operationID)
	if !ok {
		return model.BuildProgress{}, fmt.Errorf("no step list for operation %q", operationID)
	}
	return b.tracker.Snapshot(operationID, b.running), nil
}

// sortedArgNames returns the keys of the build arguments in a stable order.
func sortedArgNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// stepSummary is the "5 of 9 steps" detail line shown on the operation.
func stepSummary(done, total, cached int, current string) string {
	line := fmt.Sprintf("%d of %d steps", done, total)
	if cached > 0 {
		line += fmt.Sprintf(" - %d cached", cached)
	}
	if current = strings.TrimSpace(current); current != "" {
		line += " - " + current
	}
	return line
}

var timeNow = time.Now

func joinPath(dir, rel string) string {
	if strings.Contains(dir, `\`) {
		return strings.TrimRight(dir, `\`) + `\` + strings.ReplaceAll(rel, "/", `\`)
	}
	return strings.TrimRight(dir, "/") + "/" + rel
}
