// Package notify turns Docker Engine container events into health alerts
// (crash, OOM, unhealthy, restart loop). The rule engine here is pure; delivery
// lives in hub.go.
package notify

import (
	"fmt"
	"sync"
	"time"

	"docklet/internal/model"
)

const (
	intentionalWindow = 10 * time.Second // a die this soon after stop/kill is intentional
	oomWindow         = 10 * time.Second // a die right after oom is the same incident
	coalesceWindow    = 60 * time.Second // at most one alert per container and rule
	loopStarts        = 3
	loopWindow        = 2 * time.Minute
	loopSuppress      = 10 * time.Minute
)

// Event is the subset of an Engine container event the rules need.
type Event struct {
	ConnectionID  string
	ContainerID   string
	ContainerName string
	Project       string
	Action        string // e.g. "die", "oom", "health_status: unhealthy"
	ExitCode      string // only set for die
}

// Finding is a rule match, before delivery details are attached.
type Finding struct {
	Rule     string
	Severity string
	Title    string
	Detail   string
}

type key struct{ conn, container string }

type ruleKey struct {
	key
	rule string
}

// Rules evaluates events with per-container state. Safe for concurrent use.
type Rules struct {
	now func() time.Time

	mu        sync.Mutex
	stopped   map[key]time.Time   // last stop/kill, i.e. user intent
	oom       map[key]time.Time   // last oom
	starts    map[key][]time.Time // recent counted starts
	health    map[key]string      // last health_status
	fired     map[ruleKey]time.Time
	loopUntil map[key]time.Time
}

// NewRules returns a rule engine; now may be nil (time.Now).
func NewRules(now func() time.Time) *Rules {
	if now == nil {
		now = time.Now
	}
	return &Rules{now: now, stopped: map[key]time.Time{}, oom: map[key]time.Time{}, starts: map[key][]time.Time{},
		health: map[key]string{}, fired: map[ruleKey]time.Time{}, loopUntil: map[key]time.Time{}}
}

// Evaluate returns the findings for one event under the given settings.
func (r *Rules) Evaluate(s model.NotificationSettings, ev Event) []Finding {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key{ev.ConnectionID, ev.ContainerID}
	now := r.now()
	var f *Finding
	switch ev.Action {
	case "kill", "stop":
		r.stopped[k] = now
	case "destroy":
		r.forget(k)
	case "oom":
		r.oom[k] = now
		f = &Finding{model.RuleOOM, model.SeverityError, "Out of memory", "OOM killed"}
	case "die":
		f = r.onDie(k, ev, now)
	case "start":
		f = r.onStart(k, now)
	case "health_status: unhealthy":
		prev := r.health[k]
		r.health[k] = "unhealthy"
		if prev != "unhealthy" {
			f = &Finding{model.RuleUnhealthy, model.SeverityWarning, "Unhealthy", "unhealthy"}
		}
	case "health_status: healthy", "health_status: starting":
		r.health[k] = ev.Action[len("health_status: "):]
	}
	if f == nil || !s.Enabled || !ruleOn(s.Rules, f.Rule) || muted(s, ev) || !r.allow(k, f.Rule, now) {
		return nil
	}
	return []Finding{*f}
}

func (r *Rules) onDie(k key, ev Event, now time.Time) *Finding {
	if ev.ExitCode == "" || ev.ExitCode == "0" {
		return nil
	}
	if t, ok := r.stopped[k]; ok && now.Sub(t) <= intentionalWindow {
		return nil
	}
	if t, ok := r.oom[k]; ok && now.Sub(t) <= oomWindow {
		return nil // already reported as OOM
	}
	return &Finding{model.RuleCrash, model.SeverityError, "Container crashed", "exit code " + ev.ExitCode}
}

func (r *Rules) onStart(k key, now time.Time) *Finding {
	if t, ok := r.stopped[k]; ok && now.Sub(t) <= intentionalWindow {
		return nil // a user-driven restart is not a loop
	}
	kept := r.starts[k][:0]
	for _, t := range r.starts[k] {
		if now.Sub(t) <= loopWindow {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	r.starts[k] = kept
	if len(kept) < loopStarts {
		return nil
	}
	if until, ok := r.loopUntil[k]; ok && now.Before(until) {
		return nil
	}
	r.loopUntil[k] = now.Add(loopSuppress)
	return &Finding{model.RuleRestartLoop, model.SeverityError, "Restart loop",
		fmt.Sprintf("restarted %d times in %d minutes", len(kept), int(loopWindow.Minutes()))}
}

// allow applies per-container per-rule coalescing and records the firing.
func (r *Rules) allow(k key, rule string, now time.Time) bool {
	rk := ruleKey{k, rule}
	if t, ok := r.fired[rk]; ok && now.Sub(t) < coalesceWindow && rule != model.RuleRestartLoop {
		return false
	}
	r.fired[rk] = now
	return true
}

func (r *Rules) forget(k key) {
	delete(r.stopped, k)
	delete(r.oom, k)
	delete(r.starts, k)
	delete(r.health, k)
	delete(r.loopUntil, k)
	for rk := range r.fired {
		if rk.key == k {
			delete(r.fired, rk)
		}
	}
}

func ruleOn(r model.AlertRules, rule string) bool {
	switch rule {
	case model.RuleCrash:
		return r.Crash
	case model.RuleOOM:
		return r.OOM
	case model.RuleUnhealthy:
		return r.Unhealthy
	case model.RuleRestartLoop:
		return r.RestartLoop
	}
	return false
}

func muted(s model.NotificationSettings, ev Event) bool {
	for _, m := range s.MutedContainers {
		if m.ConnectionID == ev.ConnectionID && m.ContainerName == ev.ContainerName {
			return true
		}
	}
	return false
}
