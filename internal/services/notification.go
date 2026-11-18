package services

import (
	"context"
	"errors"
	"strings"

	"docklet/internal/model"
	"docklet/internal/notify"
)

// NotificationService exposes container health alerts and their settings.
// Hub is created in main.go (notify.NewHub) and also installed as the engine event observer.
type NotificationService struct {
	Hub *notify.Hub
}

func (s *NotificationService) hub() (*notify.Hub, error) {
	if s.Hub == nil {
		return nil, errors.New("container alerts are not available")
	}
	return s.Hub, nil
}

// GetSettings returns the alert settings (everything is off until the user opts in).
func (s *NotificationService) GetSettings(ctx context.Context) (model.NotificationSettings, error) {
	h, err := s.hub()
	if err != nil {
		return model.NotificationSettings{}, err
	}
	return h.Settings(), nil
}

// SaveSettings validates and stores the settings.
func (s *NotificationService) SaveSettings(ctx context.Context, settings model.NotificationSettings) (model.NotificationSettings, error) {
	h, err := s.hub()
	if err != nil {
		return model.NotificationSettings{}, err
	}
	return h.Save(ctx, settings)
}

// List returns the newest alerts first (limit <= 0 means all kept alerts).
func (s *NotificationService) List(ctx context.Context, limit int) ([]model.Alert, error) {
	h, err := s.hub()
	if err != nil {
		return nil, err
	}
	return h.List(limit), nil
}

// MarkRead marks the given alerts as read.
func (s *NotificationService) MarkRead(ctx context.Context, ids []string) error {
	h, err := s.hub()
	if err != nil {
		return err
	}
	return h.MarkRead(ctx, ids)
}

// MarkAllRead marks every alert as read.
func (s *NotificationService) MarkAllRead(ctx context.Context) error {
	h, err := s.hub()
	if err != nil {
		return err
	}
	return h.MarkAllRead(ctx)
}

// Clear deletes all alerts.
func (s *NotificationService) Clear(ctx context.Context) error {
	h, err := s.hub()
	if err != nil {
		return err
	}
	return h.Clear(ctx)
}

// MuteContainer silences alerts for a container name on one connection.
func (s *NotificationService) MuteContainer(ctx context.Context, connectionID, containerName string) (model.NotificationSettings, error) {
	return s.updateMute(ctx, connectionID, containerName, true)
}

// UnmuteContainer re-enables alerts for a container name on one connection.
func (s *NotificationService) UnmuteContainer(ctx context.Context, connectionID, containerName string) (model.NotificationSettings, error) {
	return s.updateMute(ctx, connectionID, containerName, false)
}

func (s *NotificationService) updateMute(ctx context.Context, connectionID, name string, mute bool) (model.NotificationSettings, error) {
	h, err := s.hub()
	if err != nil {
		return model.NotificationSettings{}, err
	}
	name = strings.TrimPrefix(strings.TrimSpace(name), "/")
	if connectionID == "" || name == "" {
		return model.NotificationSettings{}, errors.New("a connection and a container name are required")
	}
	cur := h.Settings()
	list := cur.MutedContainers[:0]
	for _, m := range cur.MutedContainers {
		if m.ConnectionID != connectionID || m.ContainerName != name {
			list = append(list, m)
		}
	}
	if mute {
		list = append(list, model.MutedContainer{ConnectionID: connectionID, ContainerName: name})
	}
	cur.MutedContainers = list
	return h.Save(ctx, cur)
}

// TestNotification records a sample alert and, when available, shows a sample OS notification.
func (s *NotificationService) TestNotification(ctx context.Context, connectionID string) error {
	h, err := s.hub()
	if err != nil {
		return err
	}
	return h.Test(connectionID)
}
