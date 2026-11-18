package services

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"docklet/internal/compose"
	"docklet/internal/model"
)

const (
	watchMaxEvents = 500
	watchMaxRaw    = 300
	watchTailLines = 8
	watchEmitDelay = 120 * time.Millisecond
)

// watchRun is the live state of one project's `docker compose watch`: the events parsed from the
// CLI output, per-service counters and the lines that were not understood. It only records what
// the CLI printed; nothing is inferred.
type watchRun struct {
	mu       sync.Mutex
	conn     string
	project  string
	notify   func() // called (throttled) after the state changed
	timer    *time.Timer
	done     chan struct{}
	st       model.ComposeWatchState
	seq      int
	events   []model.ComposeWatchEvent // oldest first
	stats    map[string]*model.ComposeWatchStat
	order    []string
	started  map[string]time.Time // service -> when its rebuild line was read
	tail     []string             // last non-empty lines, for the exit reason
	errLines []string
}

func newWatchRun(conn, project string, opts model.ComposeWatchOptions, now time.Time, notify func()) *watchRun {
	return &watchRun{
		conn: conn, project: project, notify: notify, done: make(chan struct{}),
		stats: map[string]*model.ComposeWatchStat{}, started: map[string]time.Time{},
		st: model.ComposeWatchState{
			Active: true, Running: true, StartedAt: now.UnixMilli(),
			Services: append([]string{}, opts.Services...), NoUp: opts.NoUp,
		},
	}
}

func (r *watchRun) stat(service string) *model.ComposeWatchStat {
	s := r.stats[service]
	if s == nil {
		s = &model.ComposeWatchStat{Service: service}
		r.stats[service] = s
		r.order = append(r.order, service)
	}
	return s
}

func (r *watchRun) add(now time.Time, service, kind, text string) {
	r.seq++
	r.events = append(r.events, model.ComposeWatchEvent{ID: r.seq, At: now.UnixMilli(), Service: service, Kind: kind, Text: text})
	if len(r.events) > watchMaxEvents {
		r.events = r.events[len(r.events)-watchMaxEvents:]
	}
}

// feed interprets one CLI line. It returns the text worth logging in the Activity center (empty
// for build/status noise, which is kept only in the Raw section of the project's Watch tab).
func (r *watchRun) feed(line string, now time.Time) (logText string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	p := compose.ParseWatchLine(line)
	r.mu.Lock()
	r.tail = append(r.tail, line)
	if len(r.tail) > watchTailLines {
		r.tail = r.tail[1:]
	}
	switch p.Kind {
	case compose.WatchEnabled:
		r.st.Ready = true
		r.add(now, "", model.WatchEventEnabled, "Watch enabled")
		logText = "Watch enabled"
	case compose.WatchSync:
		text := fmt.Sprintf("Syncing %d change%s", p.Count, pluralS(p.Count))
		for _, sv := range p.Services {
			s := r.stat(sv)
			s.Syncs, s.LastAt = s.Syncs+1, now.UnixMilli()
			r.add(now, sv, model.WatchEventSync, text)
		}
		logText = text + svcList(p.Services)
	case compose.WatchRebuilding:
		for _, sv := range p.Services {
			r.started[sv] = now
			r.stat(sv).LastAt = now.UnixMilli()
			r.add(now, sv, model.WatchEventRebuilding, "Rebuilding")
		}
		logText = "Rebuilding" + svcList(p.Services)
	case compose.WatchRebuilt:
		for _, sv := range p.Services {
			s := r.stat(sv)
			s.Rebuilds, s.LastAt = s.Rebuilds+1, now.UnixMilli()
			text := "Rebuilt"
			if t0, ok := r.started[sv]; ok {
				text += " in " + elapsed(now.Sub(t0))
				delete(r.started, sv)
			}
			r.add(now, sv, model.WatchEventRebuilt, text)
		}
		logText = "Rebuilt" + svcList(p.Services)
	case compose.WatchRestarted:
		for _, sv := range p.Services {
			s := r.stat(sv)
			s.Restarts, s.LastAt = s.Restarts+1, now.UnixMilli()
			r.add(now, sv, model.WatchEventRestarted, "Restarted")
		}
		logText = "Restarted" + svcList(p.Services)
	case compose.WatchError:
		r.add(now, "", model.WatchEventError, line)
		r.errLines = append(r.errLines, line)
		if len(r.errLines) > 3 {
			r.errLines = r.errLines[1:]
		}
		logText = line
	default:
		r.st.Raw = append(r.st.Raw, line)
		if len(r.st.Raw) > watchMaxRaw {
			r.st.Raw = r.st.Raw[len(r.st.Raw)-watchMaxRaw:]
		}
	}
	r.mu.Unlock()
	r.touch()
	return logText
}

// exitReason is what the CLI said last: its error lines; without any, the process result and its
// last output line (so "Watch enabled" is never presented as the reason).
func (r *watchRun) exitReason(err error) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.errLines) > 0 {
		return strings.Join(r.errLines, "\n")
	}
	msg := "docker compose watch ended without an error message"
	if err != nil {
		msg = "docker compose watch ended unexpectedly: " + err.Error()
	}
	if n := len(r.tail); n > 0 {
		msg += "\nLast output: " + r.tail[n-1]
	}
	return msg
}

// finish records the end of the process. exit is empty for a stop requested by the user.
func (r *watchRun) finish(now time.Time, exit string) {
	r.mu.Lock()
	r.st.Running, r.st.EndedAt, r.st.Exit = false, now.UnixMilli(), exit
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	r.mu.Unlock()
	if r.notify != nil {
		r.notify()
	}
	close(r.done)
}

func (r *watchRun) setOperation(id string) {
	r.mu.Lock()
	r.st.OperationID = id
	r.mu.Unlock()
}

func (r *watchRun) running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.st.Running
}

// touch schedules one throttled notification (a burst of lines becomes a single event).
func (r *watchRun) touch() {
	if r.notify == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timer != nil || !r.st.Running {
		return
	}
	r.timer = time.AfterFunc(watchEmitDelay, func() {
		r.mu.Lock()
		r.timer = nil
		r.mu.Unlock()
		r.notify()
	})
}

// snapshot copies the state for the frontend (events newest first).
func (r *watchRun) snapshot() model.ComposeWatchState {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.st
	st.Raw = append([]string{}, r.st.Raw...)
	st.Services = append([]string{}, r.st.Services...)
	st.Events = make([]model.ComposeWatchEvent, len(r.events))
	for i, e := range r.events {
		st.Events[len(r.events)-1-i] = e
	}
	st.Stats = make([]model.ComposeWatchStat, 0, len(r.order))
	for _, n := range r.order {
		st.Stats = append(st.Stats, *r.stats[n])
	}
	return st
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func svcList(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return ": " + strings.Join(s, ", ")
}

// elapsed formats a measured duration: "850 ms", "8 s", "1 min 5 s".
func elapsed(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%d ms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Round(time.Second)/time.Second))
	}
	d = d.Round(time.Second)
	return fmt.Sprintf("%d min %d s", int(d/time.Minute), int(d%time.Minute/time.Second))
}
