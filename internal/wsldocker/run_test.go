package wsldocker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func simpleSteps() []Step {
	mk := func(title string, argv ...string) Step {
		return Step{Title: title, Cmd: root(argv...), Timeout: time.Second}
	}
	return []Step{mk("one", "echo", "1"), mk("two", "echo", "2"), mk("three", "echo", "3")}
}

func wslConfStep() (Step, string) {
	step := Step{Title: "wsl.conf", Timeout: time.Second, Edit: &FileEdit{Path: "/etc/wsl.conf", Transform: EnableSystemd}}
	return step, Display(root("sh", "-c", readScript, "sh", "/etc/wsl.conf"))
}

func TestRun_StopsAtFirstFailureAndNamesTheStep(t *testing.T) {
	ex := &fakeExec{
		output: map[string][]string{"echo 2": {"E: unable to locate package"}},
		fail:   map[string]error{"echo 2": errors.New("exit status 100")},
	}
	var log []string
	err := Run(context.Background(), ex, "D", simpleSteps(), Reporter{Log: func(l string) { log = append(log, l) }})
	var se *StepError
	if !errors.As(err, &se) || se.Index != 2 || se.Title != "two" {
		t.Fatalf("err = %v", err)
	}
	if len(ex.calls) != 2 {
		t.Fatalf("step three must not run: %v", ex.displays())
	}
	msg := err.Error()
	for _, want := range []string{"Step 2 of 3", "unable to locate package", "Already done, and left in place: one"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q: %s", want, msg)
		}
	}
	mustContain(t, log, "$ echo 2")
	mustContain(t, log, "[2/3] two")
}

func TestRun_CancellationStopsBeforeTheNextStep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ex := &fakeExec{onCall: func(Cmd) { cancel() }} // the user cancels while step one runs
	err := Run(ctx, ex, "D", simpleSteps(), Reporter{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if len(ex.calls) != 1 {
		t.Fatalf("only the running step may start: %v", ex.displays())
	}
}

func TestRun_FileEditPreservesExistingContentAndSkipsWhenAlreadySet(t *testing.T) {
	step, read := wslConfStep()
	ex := &fakeExec{output: map[string][]string{read: {"[user]", "default=alice"}}}
	if err := Run(context.Background(), ex, "D", []Step{step}, Reporter{}); err != nil {
		t.Fatal(err)
	}
	w := ex.calls[len(ex.calls)-1]
	if Display(w) != "tee /etc/wsl.conf" || w.Stdin != "[user]\ndefault=alice\n\n[boot]\nsystemd=true\n" {
		t.Fatalf("write: %v %q", w.Argv, w.Stdin)
	}
	ex = &fakeExec{output: map[string][]string{read: {"[boot]", "systemd=true"}}}
	if err := Run(context.Background(), ex, "D", []Step{step}, Reporter{}); err != nil || len(ex.calls) != 1 {
		t.Fatalf("already enabled must not write: %v %v", err, ex.displays())
	}
}

func TestRun_FileEditDoesNotOverwriteWhenReadingFails(t *testing.T) {
	step, read := wslConfStep()
	ex := &fakeExec{fail: map[string]error{read: errors.New("exit status 1")}}
	if err := Run(context.Background(), ex, "D", []Step{step}, Reporter{}); err == nil {
		t.Fatal("expected failure")
	}
	if len(ex.calls) != 1 {
		t.Fatal("an unreadable file must not be rewritten")
	}
}

func TestWslArgs_RootAndHostCommandsKeepArgvSeparate(t *testing.T) {
	got := wslArgs("My Distro", Cmd{Argv: []string{"usermod", "-aG", "docker", "a b"}, Root: true})
	if want := "-d|My Distro|-u|root|--exec|usermod|-aG|docker|a b"; strings.Join(got, "|") != want {
		t.Fatalf("got %v", got)
	}
	if got := wslArgs("D", Cmd{Argv: []string{"--terminate", "D"}, Host: true}); strings.Join(got, "|") != "--terminate|D" {
		t.Fatalf("host: %v", got)
	}
	if got := wslArgs("D", Cmd{Argv: []string{"id"}}); strings.Contains(strings.Join(got, "|"), "root") {
		t.Fatalf("non-root command must not use -u root: %v", got)
	}
}

func TestDisplay_QuotesArgumentsThatNeedIt(t *testing.T) {
	if got := Display(Cmd{Argv: []string{"sh", "-c", "echo 'hi'", ""}}); got != `sh -c 'echo '\''hi'\''' ''` {
		t.Fatalf("got %s", got)
	}
}
