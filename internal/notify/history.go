package notify

import (
	"context"
	"errors"

	"docklet/internal/model"
)

// Settings returns the current settings with OSAvailable filled in.
func (h *Hub) Settings() model.NotificationSettings {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.settings
	s.MutedContainers = append([]model.MutedContainer{}, s.MutedContainers...)
	s.OSAvailable = h.d.OS != nil
	return s
}

// Save validates, persists and applies new settings.
func (h *Hub) Save(ctx context.Context, s model.NotificationSettings) (model.NotificationSettings, error) {
	s = normalize(s)
	if s.OSNotifications && h.d.OS == nil {
		return model.NotificationSettings{}, errors.New("OS notifications are not available on this system")
	}
	if err := saveSettings(ctx, h.d.KV, s); err != nil {
		return model.NotificationSettings{}, err
	}
	h.mu.Lock()
	h.settings = s
	h.mu.Unlock()
	return h.Settings(), nil
}

// List returns the newest alerts first.
func (h *Hub) List(limit int) []model.Alert {
	h.mu.Lock()
	defer h.mu.Unlock()
	if limit <= 0 || limit > len(h.alerts) {
		limit = len(h.alerts)
	}
	return append([]model.Alert{}, h.alerts[:limit]...)
}

// MarkRead marks the given alerts read.
func (h *Hub) MarkRead(ctx context.Context, ids []string) error {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	h.mu.Lock()
	for i := range h.alerts {
		if want[h.alerts[i].ID] {
			h.alerts[i].Read = true
		}
	}
	h.mu.Unlock()
	if h.d.Store == nil {
		return nil
	}
	return h.d.Store.MarkAlertsRead(ctx, ids)
}

// MarkAllRead marks every alert read.
func (h *Hub) MarkAllRead(ctx context.Context) error {
	h.mu.Lock()
	for i := range h.alerts {
		h.alerts[i].Read = true
	}
	h.mu.Unlock()
	if h.d.Store == nil {
		return nil
	}
	return h.d.Store.MarkAllAlertsRead(ctx)
}

// Clear deletes the alert history.
func (h *Hub) Clear(ctx context.Context) error {
	h.mu.Lock()
	h.alerts = nil
	h.mu.Unlock()
	if h.d.Store == nil {
		return nil
	}
	return h.d.Store.ClearAlerts(ctx)
}
