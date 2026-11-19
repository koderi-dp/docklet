package notify

import (
	"testing"
	"time"

	"docklet/internal/model"
)

// "destroy" is the only event that calls forget: a container that is gone must leave no state behind,
// so a recreated container reusing its id is judged from scratch.
func TestForget_DestroyClearsEveryPerContainerRecord(t *testing.T) {
	t.Run("coalescing", func(t *testing.T) {
		r, _ := rules()
		if len(r.Evaluate(allOn(), die("1"))) != 1 {
			t.Fatal("first crash must alert")
		}
		if len(r.Evaluate(allOn(), die("1"))) != 0 {
			t.Fatal("second crash within the window is coalesced")
		}
		r.Evaluate(allOn(), ev("destroy"))
		if len(r.Evaluate(allOn(), die("1"))) != 1 {
			t.Fatal("a destroyed container must not keep its coalescing window")
		}
	})
	t.Run("intentional stop", func(t *testing.T) {
		r, _ := rules()
		r.Evaluate(allOn(), ev("stop"))
		r.Evaluate(allOn(), ev("destroy"))
		if len(r.Evaluate(allOn(), die("1"))) != 1 {
			t.Fatal("the old stop must not excuse a crash of the new container")
		}
	})
	t.Run("oom", func(t *testing.T) {
		r, _ := rules()
		r.Evaluate(allOn(), ev("oom"))
		r.Evaluate(allOn(), ev("destroy"))
		if len(r.Evaluate(allOn(), die("137"))) != 1 {
			t.Fatal("the old oom must not swallow a later crash")
		}
	})
	t.Run("unhealthy transition", func(t *testing.T) {
		r, _ := rules()
		r.Evaluate(allOn(), ev("health_status: unhealthy"))
		r.Evaluate(allOn(), ev("destroy"))
		if len(r.Evaluate(allOn(), ev("health_status: unhealthy"))) != 1 {
			t.Fatal("unhealthy after a recreate is a new transition")
		}
	})
	t.Run("start history and loop suppression", func(t *testing.T) {
		r, c := rules()
		for range loopStarts {
			r.Evaluate(allOn(), ev("start"))
			c.advance(time.Second)
		}
		r.Evaluate(allOn(), ev("destroy"))
		for i := 0; i < loopStarts-1; i++ {
			if f := r.Evaluate(allOn(), ev("start")); len(f) != 0 {
				t.Fatal("old starts were counted towards the new container")
			}
			c.advance(time.Second)
		}
		if f := r.Evaluate(allOn(), ev("start")); len(f) != 1 || f[0].Rule != model.RuleRestartLoop {
			t.Fatalf("a fresh loop must alert again right after the old loop was forgotten: %+v", f)
		}
	})
}

func TestForget_LeavesOtherContainersAlone(t *testing.T) {
	r, _ := rules()
	other := die("1")
	other.ContainerID = "id2"
	r.Evaluate(allOn(), other)
	r.Evaluate(allOn(), ev("destroy")) // destroys id1 only
	if len(r.Evaluate(allOn(), other)) != 0 {
		t.Fatal("destroying one container reset another container's coalescing")
	}
	otherConn := ev("destroy")
	otherConn.ConnectionID = "c2"
	r.Evaluate(allOn(), otherConn)
	if len(r.Evaluate(allOn(), other)) != 0 {
		t.Fatal("destroy on another connection with the same container id reset this one")
	}
}
