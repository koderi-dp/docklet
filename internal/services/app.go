package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/update"
)

const (
	appSettingsKey    = "app.settings"
	updateStatusKey   = "app.updateStatus"
	updateDismissKey  = "app.updateDismissed"
	backgroundCheckIn = 20 * time.Second
)

// KV is the settings storage the app service needs (satisfied by *appdb.DB).
type KV interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key, value string) error
}

// TrayControl applies tray settings at runtime and learns which engine is active.
type TrayControl interface {
	Apply(model.AppSettings)
	SetActive(connectionID string)
}

// AppService exposes app-level settings (tray, update checks) and the version/update status.
// Update checks only report; nothing is ever downloaded or installed.
type AppService struct {
	KV      KV
	Tray    TrayControl // nil when there is no tray support
	Events  engine.Emitter
	Checker *update.Checker
	Version string           // the running version ("dev" for development builds)
	Now     func() time.Time // defaults to time.Now
}

func (s *AppService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *AppService) kv() (KV, error) {
	if s.KV == nil {
		return nil, errors.New("settings storage is not available")
	}
	return s.KV, nil
}

// LoadAppSettings reads the stored settings; a missing or corrupt value yields the defaults (all off).
func LoadAppSettings(ctx context.Context, kv KV) model.AppSettings {
	s := model.AppSettings{CheckForUpdates: model.UpdateCheckNever}
	if kv != nil {
		if raw, ok, err := kv.GetSetting(ctx, appSettingsKey); err == nil && ok {
			_ = json.Unmarshal([]byte(raw), &s)
		}
	}
	if _, ok := update.Interval(s.CheckForUpdates); !ok {
		s.CheckForUpdates = model.UpdateCheckNever
	}
	return s
}

// GetAppSettings returns the tray and update preferences.
func (s *AppService) GetAppSettings(ctx context.Context) (model.AppSettings, error) {
	return LoadAppSettings(ctx, s.KV), nil
}

// SaveAppSettings stores the settings and applies the tray changes immediately.
func (s *AppService) SaveAppSettings(ctx context.Context, settings model.AppSettings) (model.AppSettings, error) {
	kv, err := s.kv()
	if err != nil {
		return model.AppSettings{}, err
	}
	if _, ok := update.Interval(settings.CheckForUpdates); !ok {
		settings.CheckForUpdates = model.UpdateCheckNever
	}
	// The setup state belongs to SetOnboarding: a stale copy from the UI must not overwrite it.
	stored := LoadAppSettings(ctx, kv)
	settings.Onboarding, settings.OnboardingAt = stored.Onboarding, stored.OnboardingAt
	raw, err := json.Marshal(settings)
	if err != nil {
		return model.AppSettings{}, err
	}
	if err := kv.SetSetting(ctx, appSettingsKey, string(raw)); err != nil {
		return model.AppSettings{}, err
	}
	if s.Tray != nil {
		s.Tray.Apply(settings)
	}
	return settings, nil
}

// SetActiveConnection tells the tray which engine is selected in the app.
func (s *AppService) SetActiveConnection(ctx context.Context, connectionID string) {
	if s.Tray != nil {
		s.Tray.SetActive(connectionID)
	}
}

// GetVersion returns the running version.
func (s *AppService) GetVersion(ctx context.Context) string { return s.Version }
