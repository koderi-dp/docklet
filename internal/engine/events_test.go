package engine

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncer_CoalescesBurstIntoOneCall(t *testing.T) {
	var n atomic.Int32
	d := newDebouncer(20*time.Millisecond, func() { n.Add(1) })
	defer d.Stop()
	for range 5 {
		d.Trigger()
	}
	waitFor(t, func() bool { return n.Load() >= 1 }, "debounced call")
	time.Sleep(60 * time.Millisecond) // a second call would have fired by now
	if got := n.Load(); got != 1 {
		t.Fatalf("burst of 5 triggers produced %d calls, want 1", got)
	}
}

func TestDebouncer_StopPreventsPendingCall(t *testing.T) {
	var n atomic.Int32
	d := newDebouncer(20*time.Millisecond, func() { n.Add(1) })
	d.Trigger()
	d.Stop()
	time.Sleep(60 * time.Millisecond)
	if n.Load() != 0 {
		t.Fatal("stopped debouncer fired")
	}
}

func TestDebouncer_FiresDuringContinuousEventStream(t *testing.T) {
	old := maxBurstWait
	maxBurstWait = 150 * time.Millisecond
	defer func() { maxBurstWait = old }()
	var n atomic.Int32
	d := newDebouncer(50*time.Millisecond, func() { n.Add(1) })
	defer d.Stop()
	deadline := time.Now().Add(maxBurstWait + 700*time.Millisecond)
	for time.Now().Before(deadline) && n.Load() == 0 {
		d.Trigger() // faster than the quiet period: pure debouncing would never fire
		time.Sleep(10 * time.Millisecond)
	}
	if n.Load() == 0 {
		t.Fatal("notification starved by a continuous event stream")
	}
}
