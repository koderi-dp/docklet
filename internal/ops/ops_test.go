package ops

import (
	"context"
	"errors"
	"testing"
	"time"

	"docklet/internal/model"
)

func waitStatus(t *testing.T, m *Manager, id string, want model.OperationStatus) model.Operation {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, op := range m.List() {
			if op.ID == id && op.Status == want {
				return op
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("operation %s never reached %s", id, want)
	return model.Operation{}
}

func TestStart_SucceedsAndRecordsLog(t *testing.T) {
	m := New(nil)
	op := m.Start(Spec{Kind: "t"}, func(ctx context.Context, h *Handle) error { h.Log("hello"); return nil })
	if op.Status != model.OpRunning {
		t.Fatalf("initial status = %s", op.Status)
	}
	waitStatus(t, m, op.ID, model.OpSucceeded)
	if logs, _ := m.Logs(op.ID); len(logs) != 1 || logs[0] != "hello" {
		t.Fatalf("logs = %v", logs)
	}
}

func TestStart_FailsWithErrorMessage(t *testing.T) {
	m := New(nil)
	op := m.Start(Spec{Kind: "t"}, func(ctx context.Context, h *Handle) error { return errors.New("boom") })
	if got := waitStatus(t, m, op.ID, model.OpFailed); got.Error != "boom" {
		t.Fatalf("error = %q", got.Error)
	}
}

func TestStart_TurnsPanicIntoFailedOperation(t *testing.T) {
	m := New(nil)
	op := m.Start(Spec{Kind: "t"}, func(ctx context.Context, h *Handle) error { panic("x") })
	if got := waitStatus(t, m, op.ID, model.OpFailed); got.Error == "" {
		t.Fatal("a panic must be reported as the operation error")
	}
}

func TestOnFinish_CalledOnceWithTerminalStatusForEveryOutcome(t *testing.T) {
	m := New(nil)
	finished := make(chan model.Operation, 4)
	m.OnFinish = func(op model.Operation) { finished <- op }
	m.Start(Spec{Kind: "ok"}, func(context.Context, *Handle) error { return nil })
	m.Start(Spec{Kind: "bad"}, func(context.Context, *Handle) error { return errors.New("boom") })
	cancelled := m.Start(Spec{Kind: "cancelled", Cancellable: true}, func(ctx context.Context, h *Handle) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err := m.Cancel(cancelled.ID); err != nil {
		t.Fatal(err)
	}
	got := map[string]model.Operation{}
	for range 3 {
		select {
		case op := <-finished:
			got[op.Kind] = op
		case <-time.After(2 * time.Second):
			t.Fatalf("OnFinish called %d times, want 3", len(got))
		}
	}
	if got["ok"].Status != model.OpSucceeded || got["bad"].Status != model.OpFailed || got["bad"].Error != "boom" || got["cancelled"].Status != model.OpCancelled {
		t.Fatalf("terminal states = %+v", got)
	}
	for _, op := range got {
		if op.FinishedAt == 0 {
			t.Errorf("%s: FinishedAt not set", op.Kind)
		}
	}
	select {
	case extra := <-finished:
		t.Fatalf("OnFinish called twice for %s", extra.Kind)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCancel_StopsRunningBodyAndMarksCancelled(t *testing.T) {
	m := New(nil)
	stopped := make(chan struct{})
	op := m.Start(Spec{Kind: "t", Cancellable: true}, func(ctx context.Context, h *Handle) error {
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	})
	if err := m.Cancel(op.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("body did not observe cancellation")
	}
	waitStatus(t, m, op.ID, model.OpCancelled)
}

func TestCancel_UnknownIDIsAnError(t *testing.T) {
	if err := New(nil).Cancel("nope"); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestClearFinished_KeepsRunningOperations(t *testing.T) {
	m := New(nil)
	release := make(chan struct{})
	done := m.Start(Spec{Kind: "done"}, func(context.Context, *Handle) error { return nil })
	running := m.Start(Spec{Kind: "running"}, func(context.Context, *Handle) error { <-release; return nil })
	waitStatus(t, m, done.ID, model.OpSucceeded)
	m.ClearFinished()
	list := m.List()
	if len(list) != 1 || list[0].ID != running.ID {
		t.Fatalf("after ClearFinished: %+v", list)
	}
	close(release)
}
