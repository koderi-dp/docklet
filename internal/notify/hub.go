package notify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/moby/moby/api/types/events"

	"docklet/internal/engine"
	"docklet/internal/model"
)

const (
	ringSize  = 200
	queueSize = 512
)

// OSNotifier shows a native desktop notification. A nil notifier means in-app only.
type OSNotifier interface {
	Notify(title, body string) error
}

// AlertStore persists the alert history (satisfied by *appdb.DB).
type AlertStore interface {
	AddAlert(ctx context.Context, a model.Alert) error
	ListAlerts(ctx context.Context, limit int) ([]model.Alert, error)
	MarkAlertsRead(ctx context.Context, ids []string) error
	MarkAllAlertsRead(ctx context.Context) error
	ClearAlerts(ctx context.Context) error
}

// Deps are the hub's collaborators. Everything except Engines may be nil.
type Deps struct {
	Engines  engine.Provider      // used to read the last health-check output
	Profiles engine.ProfileSource // connection names
	Emitter  engine.Emitter
	KV       KV
	Store    AlertStore
	OS       OSNotifier
	Now      func() time.Time
}

// Hub receives container events, applies the rules and delivers alerts.
type Hub struct {
	d     Deps
	rules *Rules
	in    chan Event

	mu       sync.Mutex
	settings model.NotificationSettings
	alerts   []model.Alert // newest first
}

// NewHub loads persisted settings and history and starts the worker.
func NewHub(d Deps) *Hub {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := &Hub{d: d, rules: NewRules(d.Now), in: make(chan Event, queueSize), settings: loadSettings(ctx, d.KV)}
	if d.Store != nil {
		if list, err := d.Store.ListAlerts(ctx, ringSize); err == nil {
			h.alerts = list
		}
	}
	go h.run()
	return h
}

// ObserveEvent implements engine.EventObserver. It never blocks the event loop.
func (h *Hub) ObserveEvent(connectionID string, m events.Message) {
	if m.Type != events.ContainerEventType {
		return
	}
	a := m.Actor.Attributes
	ev := Event{ConnectionID: connectionID, ContainerID: m.Actor.ID, ContainerName: a["name"],
		Project: a["com.docker.compose.project"], Action: string(m.Action), ExitCode: a["exitCode"]}
	select {
	case h.in <- ev:
	default: // queue full: dropping an alert beats stalling the Engine stream
	}
}

func (h *Hub) run() {
	for ev := range h.in {
		for _, f := range h.rules.Evaluate(h.Settings(), ev) {
			if f.Rule == model.RuleUnhealthy {
				f.Detail = h.healthDetail(ev)
			}
			h.record(ev, f)
		}
	}
}

func (h *Hub) healthDetail(ev Event) string {
	if h.d.Engines == nil {
		return "unhealthy"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if out := lastHealthOutput(ctx, h.d.Engines, ev.ConnectionID, ev.ContainerID); out != "" {
		return "unhealthy: " + out
	}
	return "unhealthy"
}

func (h *Hub) record(ev Event, f Finding) {
	a := model.Alert{ID: newID(), ConnectionID: ev.ConnectionID, ConnectionName: h.connectionName(ev.ConnectionID),
		ContainerID: ev.ContainerID, ContainerName: ev.ContainerName, ComposeProject: ev.Project,
		Rule: f.Rule, Severity: f.Severity, Title: f.Title, Detail: f.Detail, At: h.now().UnixMilli()}
	h.add(a)
	if h.Settings().OSNotifications {
		h.osNotify(a)
	}
}

func (h *Hub) add(a model.Alert) {
	h.mu.Lock()
	h.alerts = append([]model.Alert{a}, h.alerts...)
	if len(h.alerts) > ringSize {
		h.alerts = h.alerts[:ringSize]
	}
	h.mu.Unlock()
	if h.d.Store != nil {
		if err := h.d.Store.AddAlert(context.Background(), a); err != nil {
			log.Printf("alerts: persist: %v", err)
		}
	}
	if h.d.Emitter != nil {
		h.d.Emitter.Emit(model.EventAlertsNew, model.AlertNew{Alert: a})
	}
}

func (h *Hub) osNotify(a model.Alert) {
	if h.d.OS == nil {
		return
	}
	title := a.Title + ": " + a.ContainerName
	body := a.Detail
	if a.ConnectionName != "" {
		body += " (" + a.ConnectionName + ")"
	}
	go func() {
		defer func() { _ = recover() }()
		if err := h.d.OS.Notify(title, body); err != nil {
			log.Printf("alerts: OS notification: %v", err)
		}
	}()
}

// Test records a sample alert and, when an OS notifier is wired, sends a sample OS notification.
func (h *Hub) Test(connectionID string) error {
	a := model.Alert{ID: newID(), ConnectionID: connectionID, ConnectionName: h.connectionName(connectionID),
		Rule: "test", Severity: model.SeverityWarning, Title: "Test notification", Detail: "Container alerts are working.", At: h.now().UnixMilli()}
	h.add(a)
	if h.d.OS == nil {
		return nil
	}
	if err := h.d.OS.Notify(a.Title, a.Detail); err != nil {
		return errors.New("the operating system rejected the notification: " + err.Error())
	}
	return nil
}

func (h *Hub) now() time.Time {
	if h.d.Now != nil {
		return h.d.Now()
	}
	return time.Now()
}

func (h *Hub) connectionName(id string) string {
	if h.d.Profiles == nil {
		return ""
	}
	if p, err := h.d.Profiles.Profile(id); err == nil {
		return p.Name
	}
	return ""
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
