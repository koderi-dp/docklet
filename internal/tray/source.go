package tray

import (
	"context"
	"sync"

	"docklet/internal/model"
)

// EngineSource lists saved connections and their current status (never connects anything).
type EngineSource interface {
	List(ctx context.Context) ([]model.ConnectionProfile, error)
	Status(ctx context.Context, id string) (model.ConnectionStatus, error)
}

// ContainerSource lists containers of one connection.
type ContainerSource interface {
	List(ctx context.Context, connectionID string) ([]model.Container, error)
}

// AlertSource exposes the alert settings and history.
type AlertSource interface {
	Settings() model.NotificationSettings
	List(limit int) []model.Alert
}

// Source gathers the State the menu is built from. The frontend owns the active engine and reports it.
type Source struct {
	Engines    EngineSource
	Containers ContainerSource
	Alerts     AlertSource

	mu     sync.Mutex
	active string
}

// SetActive records the engine selected in the app.
func (s *Source) SetActive(id string) {
	s.mu.Lock()
	s.active = id
	s.mu.Unlock()
}

// Snapshot reads the current state. Failures degrade to a smaller menu, never to an error.
func (s *Source) Snapshot(ctx context.Context) State {
	s.mu.Lock()
	st := State{ActiveID: s.active}
	s.mu.Unlock()
	if s.Engines != nil {
		profiles, _ := s.Engines.List(ctx)
		for _, p := range profiles {
			status, _ := s.Engines.Status(ctx, p.ID)
			state := status.State
			if state == "" {
				state = model.StateDisconnected
			}
			st.Engines = append(st.Engines, Engine{ID: p.ID, Name: p.Name, State: state})
		}
	}
	if e, ok := st.active(); ok && e.State == model.StateConnected && s.Containers != nil {
		list, _ := s.Containers.List(ctx, e.ID)
		for _, c := range list {
			if c.State == "running" {
				st.Containers = append(st.Containers, Container{ID: c.ID, Name: c.Name})
			}
		}
	}
	if s.Alerts != nil && s.Alerts.Settings().Enabled {
		st.AlertsEnabled = true
		for _, a := range s.Alerts.List(0) {
			if !a.Read {
				st.Unread++
			}
		}
	}
	return st
}
