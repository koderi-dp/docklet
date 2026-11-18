package notify

import (
	"testing"
	"time"

	"docklet/internal/model"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func allOn() model.NotificationSettings {
	return model.NotificationSettings{Enabled: true,
		Rules: model.AlertRules{Crash: true, OOM: true, Unhealthy: true, RestartLoop: true}}
}

func ev(action string) Event {
	return Event{ConnectionID: "c1", ContainerID: "id1", ContainerName: "web", Action: action}
}

func die(code string) Event { e := ev("die"); e.ExitCode = code; return e }

func rules() (*Rules, *clock) {
	c := &clock{t: time.Unix(1_000_000, 0)}
	return NewRules(c.now), c
}

func rulesOf(f []Finding) []string {
	var out []string
	for _, x := range f {
		out = append(out, x.Rule)
	}
	return out
}

func TestCrashAlertsOnNonZeroExitOnly(t *testing.T) {
	r, _ := rules()
	if got := r.Evaluate(allOn(), die("0")); len(got) != 0 {
		t.Fatalf("exit 0 alerted: %v", got)
	}
	got := r.Evaluate(allOn(), die("3"))
	if len(got) != 1 || got[0].Rule != model.RuleCrash || got[0].Detail != "exit code 3" {
		t.Fatalf("want one crash finding, got %+v", got)
	}
}

func TestDisabledSettingsProduceNothing(t *testing.T) {
	r, _ := rules()
	s := allOn()
	s.Enabled = false
	if got := r.Evaluate(s, die("1")); len(got) != 0 {
		t.Fatalf("alerted while globally disabled: %v", got)
	}
	s = allOn()
	s.Rules.Crash = false
	if got := r.Evaluate(s, die("1")); len(got) != 0 {
		t.Fatalf("alerted with the crash rule off: %v", got)
	}
}

func TestStopOrKillBeforeDieIsIntentional(t *testing.T) {
	r, c := rules()
	r.Evaluate(allOn(), ev("kill"))
	c.advance(2 * time.Second)
	if got := r.Evaluate(allOn(), die("137")); len(got) != 0 {
		t.Fatalf("docker stop produced %v", got)
	}
	c.advance(time.Minute)
	if got := r.Evaluate(allOn(), die("1")); len(got) != 1 {
		t.Fatal("a crash long after a stop must alert again")
	}
}

func TestOOMSuppressesTheFollowingDie(t *testing.T) {
	r, c := rules()
	got := r.Evaluate(allOn(), ev("oom"))
	if len(got) != 1 || got[0].Rule != model.RuleOOM {
		t.Fatalf("oom: %+v", got)
	}
	c.advance(time.Second)
	if got := r.Evaluate(allOn(), die("137")); len(got) != 0 {
		t.Fatalf("die after oom double-reported: %v", got)
	}
}

func TestUnhealthyAlertsOnTransitionOnly(t *testing.T) {
	r, c := rules()
	if got := r.Evaluate(allOn(), ev("health_status: unhealthy")); len(got) != 1 {
		t.Fatal("first unhealthy must alert")
	}
	c.advance(2 * time.Minute)
	if got := r.Evaluate(allOn(), ev("health_status: unhealthy")); len(got) != 0 {
		t.Fatal("repeated unhealthy without recovery must not alert")
	}
	r.Evaluate(allOn(), ev("health_status: healthy"))
	c.advance(2 * time.Minute)
	if got := r.Evaluate(allOn(), ev("health_status: unhealthy")); len(got) != 1 {
		t.Fatal("unhealthy after recovery must alert")
	}
}

func TestRestartLoopFiresOnceThenSuppressedForTenMinutes(t *testing.T) {
	r, c := rules()
	var fired int
	start := func() {
		for _, f := range r.Evaluate(allOn(), ev("start")) {
			if f.Rule == model.RuleRestartLoop {
				fired++
			}
		}
	}
	for range 5 {
		start()
		c.advance(10 * time.Second)
	}
	if fired != 1 {
		t.Fatalf("fired %d restart-loop alerts, want 1", fired)
	}
	c.advance(11 * time.Minute)
	for range 3 {
		start()
		c.advance(10 * time.Second)
	}
	if fired != 2 {
		t.Fatalf("after the suppression window fired=%d, want 2", fired)
	}
}

func TestSlowStartsAreNotARestartLoop(t *testing.T) {
	r, c := rules()
	for range 6 {
		if got := r.Evaluate(allOn(), ev("start")); len(got) != 0 {
			t.Fatalf("starts 90s apart alerted: %v", got)
		}
		c.advance(90 * time.Second)
	}
}

func TestUserRestartsAreNotCountedAsLoop(t *testing.T) {
	r, c := rules()
	for range 4 {
		r.Evaluate(allOn(), ev("kill"))
		c.advance(time.Second)
		if got := r.Evaluate(allOn(), ev("start")); len(got) != 0 {
			t.Fatalf("docker restart alerted: %v", got)
		}
		c.advance(5 * time.Second)
	}
}

func TestFlappingCrashIsCoalescedPerMinute(t *testing.T) {
	r, c := rules()
	n := 0
	for range 5 {
		n += len(r.Evaluate(allOn(), die("1")))
		c.advance(5 * time.Second)
	}
	if n != 1 {
		t.Fatalf("%d alerts within a minute, want 1", n)
	}
	c.advance(time.Minute)
	if len(r.Evaluate(allOn(), die("1"))) != 1 {
		t.Fatal("alert must fire again after the coalescing window")
	}
}

func TestMutedContainerIsSilentOnlyOnItsConnection(t *testing.T) {
	r, _ := rules()
	s := allOn()
	s.MutedContainers = []model.MutedContainer{{ConnectionID: "c1", ContainerName: "web"}}
	if got := r.Evaluate(s, die("1")); len(got) != 0 {
		t.Fatalf("muted container alerted: %v", got)
	}
	other := die("1")
	other.ConnectionID, other.ContainerID = "c2", "id2"
	if got := r.Evaluate(s, other); len(got) != 1 {
		t.Fatal("same name on another connection must still alert")
	}
}

func TestConnectionsDoNotShareState(t *testing.T) {
	r, _ := rules()
	r.Evaluate(allOn(), ev("kill")) // intentional stop on c1
	other := die("1")
	other.ConnectionID = "c2" // same container id on another engine
	if got := rulesOf(r.Evaluate(allOn(), other)); len(got) != 1 {
		t.Fatalf("a stop on c1 suppressed a crash on c2: %v", got)
	}
}
