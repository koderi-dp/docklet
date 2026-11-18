package buildkit

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"docklet/internal/model"
)

const (
	maxStepLog   = 200 // log lines kept per step
	maxTrailer   = 40  // non-step lines (final error text) kept
	maxLineChars = 2000
)

// Tracker turns the CLI's progress output into a step list. It understands both
// `--progress=rawjson` (one JSON object per line) and `--progress=plain`
// (`#N ...` lines), chosen per line, so a CLI that ignores rawjson still works.
// It is safe for concurrent use.
type Tracker struct {
	mu      sync.Mutex
	now     func() time.Time
	scrub   *Scrubber
	steps   []*step
	byID    map[string]*step
	trailer []string
	plain   bool
	partial map[string]string // rawjson: unfinished last log line per step
	sink    func(string)
}

type step struct {
	id        string
	name      string
	started   bool
	completed bool
	cached    bool
	canceled  bool
	err       string
	startedAt int64 // local clock, unix ms
	durMs     int64
	log       []string
}

// NewTracker creates a Tracker. scrub (may be nil) is applied to every text
// before it is stored.
func NewTracker(scrub *Scrubber) *Tracker {
	return &Tracker{now: time.Now, scrub: scrub, byID: map[string]*step{}, partial: map[string]string{}}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func (t *Tracker) clean(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = t.scrub.Apply(s)
	if len(s) > maxLineChars {
		s = s[:maxLineChars] + "..."
	}
	return s
}

func (t *Tracker) get(id string) *step {
	if s, ok := t.byID[id]; ok {
		return s
	}
	s := &step{id: id}
	t.byID[id] = s
	t.steps = append(t.steps, s)
	return s
}

// SetSink registers a callback that receives a readable line for every step start,
// step log line, step error and loose CLI output line (it runs under the tracker's lock).
func (t *Tracker) SetSink(f func(string)) { t.mu.Lock(); t.sink = f; t.mu.Unlock() }

func (t *Tracker) emit(line string) {
	if t.sink != nil {
		t.sink(line)
	}
}

func (t *Tracker) addLog(s *step, line string) {
	s.log = append(s.log, line)
	if len(s.log) > maxStepLog {
		s.log = s.log[len(s.log)-maxStepLog:]
	}
	t.emit("    " + line)
}

func (t *Tracker) startStep(s *step) {
	s.started, s.startedAt = true, t.now().UnixMilli()
	t.emit("> " + s.name)
}

func (t *Tracker) addTrailer(line string) {
	t.trailer = append(t.trailer, line)
	if len(t.trailer) > maxTrailer {
		t.trailer = t.trailer[len(t.trailer)-maxTrailer:]
	}
}

// rawMessage is one `--progress=rawjson` object (a BuildKit StatusResponse).
type rawMessage struct {
	Vertexes []struct {
		Digest    string `json:"digest"`
		Name      string `json:"name"`
		Started   string `json:"started"`
		Completed string `json:"completed"`
		Cached    bool   `json:"cached"`
		Error     string `json:"error"`
	} `json:"vertexes"`
	Logs []struct {
		Vertex string `json:"vertex"`
		Data   string `json:"data"`
	} `json:"logs"`
	Warnings []struct {
		Vertex string `json:"vertex"`
		Short  string `json:"short"`
	} `json:"warnings"`
}

// Feed consumes one line of CLI output.
func (t *Tracker) Feed(line string) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case strings.HasPrefix(line, "{"):
		var m rawMessage
		if json.Unmarshal([]byte(line), &m) == nil {
			t.feedRaw(m)
			return
		}
	case plainLineRe.MatchString(line):
		t.plain = true
		t.feedPlain(line)
		return
	}
	clean := t.clean(line)
	t.addTrailer(clean)
	t.emit(clean)
}

func (t *Tracker) feedRaw(m rawMessage) {
	for _, v := range m.Vertexes {
		if v.Digest == "" {
			continue
		}
		s := t.get(v.Digest)
		if v.Name != "" {
			s.name = t.clean(v.Name)
		}
		if v.Started != "" && !s.started {
			t.startStep(s)
		}
		if v.Completed != "" && !s.completed {
			s.completed = true
			if !s.started {
				t.startStep(s)
			}
			s.cached = v.Cached
			s.durMs = durationBetween(v.Started, v.Completed)
		}
		if v.Error != "" && s.err == "" {
			s.err = t.clean(v.Error)
			t.emit("    ERROR: " + s.err)
		}
	}
	for _, l := range m.Logs {
		raw, err := base64.StdEncoding.DecodeString(l.Data)
		if err != nil || l.Vertex == "" {
			continue
		}
		s := t.get(l.Vertex)
		text := t.partial[l.Vertex] + string(raw)
		parts := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
		t.partial[l.Vertex] = parts[len(parts)-1]
		for _, p := range parts[:len(parts)-1] {
			if i := strings.LastIndex(p, "\r"); i >= 0 {
				p = p[i+1:] // progress bars redraw with \r: keep the final state
			}
			if p = t.clean(p); strings.TrimSpace(p) != "" {
				t.addLog(s, p)
			}
		}
	}
	for _, w := range m.Warnings {
		raw, err := base64.StdEncoding.DecodeString(w.Short)
		if err != nil || w.Vertex == "" {
			continue
		}
		t.addLog(t.get(w.Vertex), "warning: "+t.clean(string(raw)))
	}
}

func durationBetween(a, b string) int64 {
	ta, err1 := time.Parse(time.RFC3339Nano, a)
	tb, err2 := time.Parse(time.RFC3339Nano, b)
	if err1 != nil || err2 != nil || tb.Before(ta) {
		return 0
	}
	return tb.Sub(ta).Milliseconds()
}

// Finish flushes partial log lines and, when the build was cancelled, marks the
// steps that were still running.
func (t *Tracker) Finish(cancelled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, rest := range t.partial {
		if p := t.clean(rest); strings.TrimSpace(p) != "" {
			t.addLog(t.get(id), p)
		}
		delete(t.partial, id)
	}
	if cancelled {
		for _, s := range t.steps {
			if s.started && !s.completed && s.err == "" {
				s.canceled = true
			}
		}
	}
}

// Trailer returns the output lines that did not belong to any step (the CLI's
// final error text).
func (t *Tracker) Trailer() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.trailer...)
}

// ErrorMessage is the most useful one-line reason for a failed build: the CLI's
// final "ERROR:" line, else the first failed step's error.
func (t *Tracker) ErrorMessage() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := len(t.trailer) - 1; i >= 0; i-- {
		if rest, ok := strings.CutPrefix(t.trailer[i], "ERROR: "); ok {
			rest = strings.TrimPrefix(rest, "failed to build: ")
			rest = strings.TrimPrefix(rest, "failed to solve: ")
			return rest
		}
	}
	for _, s := range t.steps {
		if s.err != "" {
			return s.err
		}
	}
	return ""
}

var internalRe = regexp.MustCompile(`^(\[internal\]|docker-image://|exporting |importing cache|resolve image config|\[auth\]|\[context )`)

func isInternal(name string) bool { return internalRe.MatchString(name) }

// Snapshot returns the current step list. Logs are copied.
func (t *Tracker) Snapshot(opID string, running bool) model.BuildProgress {
	t.mu.Lock()
	defer t.mu.Unlock()
	p := model.BuildProgress{OperationID: opID, Plain: t.plain, Running: running, Steps: make([]model.BuildStep, 0, len(t.steps))}
	for _, s := range t.ordered() {
		bs := model.BuildStep{
			ID: s.id, Name: s.name, State: s.state(), StartedAt: s.startedAt, DurationMs: s.durMs,
			Internal: isInternal(s.name), Error: s.err, Log: append([]string{}, s.log...),
		}
		if bs.Name == "" {
			bs.Name = "(unnamed step)"
		}
		p.Steps = append(p.Steps, bs)
		if bs.Internal {
			continue
		}
		p.Total++
		switch bs.State {
		case model.StepDone:
			p.Done++
		case model.StepCached:
			p.Done++
			p.Cached++
		case model.StepError:
			p.Failed++
		}
	}
	return p
}

func (s *step) state() string {
	switch {
	case s.err != "":
		return model.StepError
	case s.canceled:
		return model.StepCancelled
	case s.completed && s.cached:
		return model.StepCached
	case s.completed:
		return model.StepDone
	case s.started:
		return model.StepRunning
	}
	return model.StepWaiting
}

// Status is a cheap summary for the operation's detail line: Dockerfile steps
// done, total, cached, and the name of the step running now ("" when none).
func (t *Tracker) Status() (done, total, cached int, current string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range t.steps {
		if isInternal(s.name) {
			continue
		}
		total++
		switch s.state() {
		case model.StepDone:
			done++
		case model.StepCached:
			done++
			cached++
		case model.StepRunning:
			current = s.name
		}
	}
	return
}

// ordered returns the steps in the order they ran: by start time, steps that
// started together (BuildKit announces the final stage first) with the later
// announced first, and steps that have not started yet last.
func (t *Tracker) ordered() []*step {
	out := make([]*step, len(t.steps))
	for i, s := range t.steps {
		out[len(t.steps)-1-i] = s
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].startedAt, out[j].startedAt
		if a == 0 || b == 0 {
			return a != 0 && b == 0
		}
		return a < b
	})
	return out
}
