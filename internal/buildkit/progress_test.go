package buildkit

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"docklet/internal/model"
)

func feedFile(t *testing.T, tr *Tracker, name string) {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		tr.Feed(sc.Text())
	}
}

func find(t *testing.T, p model.BuildProgress, prefix string) model.BuildStep {
	t.Helper()
	for _, s := range p.Steps {
		if strings.HasPrefix(s.Name, prefix) {
			return s
		}
	}
	t.Fatalf("no step starting with %q in %+v", prefix, p.Steps)
	return model.BuildStep{}
}

func TestRawJSON_SuccessfulBuild(t *testing.T) {
	tr := NewTracker(nil)
	feedFile(t, tr, "raw-ok.jsonl")
	tr.Finish(false)
	p := tr.Snapshot("op", false)
	if p.Failed != 0 || p.Plain {
		t.Fatalf("%+v", p)
	}
	if p.Total < 6 || p.Done != p.Total {
		t.Fatalf("total=%d done=%d", p.Total, p.Done)
	}
	for _, s := range p.Steps {
		if s.State == model.StepRunning || s.State == model.StepWaiting {
			t.Errorf("unfinished step %q (%s)", s.Name, s.State)
		}
	}
	if s := find(t, p, "[internal] load build definition"); !s.Internal {
		t.Error("[internal] steps must be marked internal")
	}
	if s := find(t, p, "exporting to image"); !s.Internal {
		t.Error("export step must be internal")
	}
	run := find(t, p, "[deps 4/4] RUN echo")
	if got := strings.Join(run.Log, "|"); got != "greet hi|token-len 0" {
		t.Errorf("log of RUN step = %q", got)
	}
	if run.DurationMs <= 0 {
		t.Errorf("duration should come from the vertex timestamps: %d", run.DurationMs)
	}
	if hd := find(t, p, "[deps 1/4] RUN --mount"); strings.Join(hd.Log, "") != "deps" {
		t.Errorf("mount step log: %v", hd.Log)
	}
	warned := false
	for _, s := range p.Steps {
		for _, l := range s.Log {
			if strings.HasPrefix(l, "warning: SecretsUsedInArgOrEnv") {
				warned = true
			}
		}
	}
	if !warned {
		t.Error("BuildKit lint warnings should be kept in the step log")
	}
}

func TestRawJSON_CachedStepsAreMarked(t *testing.T) {
	tr := NewTracker(nil)
	feedFile(t, tr, "raw-cached.jsonl")
	p := tr.Snapshot("op", false)
	if p.Cached == 0 || p.Cached > p.Done {
		t.Fatalf("cached=%d done=%d", p.Cached, p.Done)
	}
	if s := find(t, p, "[base 2/2] RUN echo base"); s.State != model.StepCached {
		t.Errorf("state = %s", s.State)
	}
}

func TestRawJSON_FailedStepKeepsErrorAndLog(t *testing.T) {
	tr := NewTracker(nil)
	feedFile(t, tr, "raw-fail.jsonl")
	feedFile(t, tr, "raw-fail.trailer.txt")
	tr.Finish(false)
	p := tr.Snapshot("op", false)
	if p.Failed != 1 {
		t.Fatalf("failed = %d", p.Failed)
	}
	s := find(t, p, "[3/4] RUN echo failing-output")
	if s.State != model.StepError || !strings.Contains(s.Error, "exit code: 3") {
		t.Fatalf("%+v", s)
	}
	if len(s.Log) != 2 {
		t.Errorf("log = %v", s.Log)
	}
	msg := tr.ErrorMessage()
	if !strings.HasPrefix(msg, `process "/bin/sh -c echo failing-output`) || !strings.Contains(msg, "exit code: 3") {
		t.Errorf("error message = %q", msg)
	}
}

func TestRawJSON_CancelledBuildMarksRunningStep(t *testing.T) {
	tr := NewTracker(nil)
	feedFile(t, tr, "raw-cancel.jsonl")
	if p := tr.Snapshot("op", true); find(t, p, "[3/3] RUN sleep 60").State != model.StepRunning {
		t.Fatal("the sleep step is running before the cancel")
	}
	tr.Finish(true)
	p := tr.Snapshot("op", false)
	if s := find(t, p, "[3/3] RUN sleep 60"); s.State != model.StepCancelled {
		t.Errorf("state = %s", s.State)
	}
	if s := find(t, p, "[2/3] RUN echo one"); s.State != model.StepCached {
		t.Errorf("finished steps keep their state: %s", s.State)
	}
}

func TestPlain_StepsGroupedByID(t *testing.T) {
	tr := NewTracker(nil)
	feedFile(t, tr, "plain-fail.txt")
	tr.Finish(false)
	p := tr.Snapshot("op", false)
	if !p.Plain || p.Failed != 1 {
		t.Fatalf("%+v", p)
	}
	if s := find(t, p, "[1/3] FROM"); s.State != model.StepCached {
		t.Errorf("FROM state = %s", s.State)
	}
	if s := find(t, p, "[2/4] RUN echo one"); s.State != model.StepDone || s.DurationMs != 100 || strings.Join(s.Log, "") != "one" {
		t.Errorf("%+v", s)
	}
	s := find(t, p, "[3/4] RUN echo failing-output")
	if s.State != model.StepError || !strings.Contains(s.Error, "exit code: 3") || len(s.Log) != 2 {
		t.Errorf("%+v", s)
	}
	if !strings.Contains(tr.ErrorMessage(), "exit code: 3") {
		t.Errorf("error = %q", tr.ErrorMessage())
	}
}

func TestPlain_SuccessWithCache(t *testing.T) {
	tr := NewTracker(nil)
	feedFile(t, tr, "plain-ok.txt")
	p := tr.Snapshot("op", false)
	if p.Failed != 0 || p.Cached < 4 || p.Done != p.Total {
		t.Fatalf("%+v", p)
	}
}

func TestTracker_ScrubsSecretsEverywhere(t *testing.T) {
	tr := NewTracker(NewScrubber([]string{"s3cr3t-value"}, 3))
	tr.Feed(`#5 [2/2] RUN echo s3cr3t-value`)
	tr.Feed(`#5 0.2 token is s3cr3t-value`)
	tr.Feed(`#5 ERROR: failed with s3cr3t-value`)
	tr.Feed(`ERROR: failed to build: s3cr3t-value`)
	p := tr.Snapshot("op", false)
	all := tr.ErrorMessage()
	for _, s := range p.Steps {
		all += s.Name + s.Error + strings.Join(s.Log, "")
	}
	all += strings.Join(tr.Trailer(), "")
	if strings.Contains(all, "s3cr3t") || !strings.Contains(all, "***") {
		t.Errorf("leak or missing mask: %q", all)
	}
}

func TestRawJSON_LogLinesSplitAcrossMessages(t *testing.T) {
	tr := NewTracker(nil)
	tr.Feed(`{"vertexes":[{"digest":"sha256:aa","name":"[1/1] RUN x","started":"2026-01-01T00:00:00Z"}]}`)
	tr.Feed(`{"logs":[{"vertex":"sha256:aa","stream":1,"data":"aGVs"}]}`)         // "hel"
	tr.Feed(`{"logs":[{"vertex":"sha256:aa","stream":1,"data":"bG8Kd29ybGQ="}]}`) // "lo\nworld"
	if got := tr.Snapshot("op", true).Steps[0].Log; len(got) != 1 || got[0] != "hello" {
		t.Fatalf("before finish: %v", got)
	}
	tr.Finish(false)
	if got := tr.Snapshot("op", false).Steps[0].Log; len(got) != 2 || got[1] != "world" {
		t.Fatalf("after finish: %v", got)
	}
}

func TestSnapshot_StepsInRunOrder(t *testing.T) {
	tr := NewTracker(nil)
	feedFile(t, tr, "raw-cached.jsonl") // BuildKit announces the final stage first
	idx := map[string]int{}
	for i, s := range tr.Snapshot("op", false).Steps {
		idx[s.Name] = i
	}
	if !(idx["[base 2/2] RUN echo base > /base.txt"] < idx["[deps 1/4] RUN --mount=type=cache,target=/var/cache/x echo cache > /var/cache/x/a && echo deps"]) {
		t.Errorf("order: %v", idx)
	}
}
