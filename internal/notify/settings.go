package notify

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"docklet/internal/model"
)

const settingsKey = "notifications.settings"

// KV is the persistence the hub needs for settings (satisfied by *appdb.DB).
type KV interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key, value string) error
}

// defaultSettings: alerts are opt-in; rules are pre-selected for when the user turns them on.
func defaultSettings() model.NotificationSettings {
	return model.NotificationSettings{Rules: model.AlertRules{Crash: true, OOM: true, Unhealthy: true, RestartLoop: true}}
}

func loadSettings(ctx context.Context, kv KV) model.NotificationSettings {
	s := defaultSettings()
	if kv == nil {
		return s
	}
	if raw, ok, err := kv.GetSetting(ctx, settingsKey); err == nil && ok {
		_ = json.Unmarshal([]byte(raw), &s) // a corrupt value keeps the defaults
	}
	s.OSAvailable = false
	return normalize(s)
}

func saveSettings(ctx context.Context, kv KV, s model.NotificationSettings) error {
	if kv == nil {
		return errors.New("settings storage is not available")
	}
	s.OSAvailable = false
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return kv.SetSetting(ctx, settingsKey, string(raw))
}

// normalize drops empty and duplicate mutes and guarantees a non-nil list.
func normalize(s model.NotificationSettings) model.NotificationSettings {
	seen := map[model.MutedContainer]bool{}
	muted := []model.MutedContainer{}
	for _, m := range s.MutedContainers {
		m.ContainerName = strings.TrimSpace(m.ContainerName)
		if m.ConnectionID == "" || m.ContainerName == "" || seen[m] {
			continue
		}
		seen[m] = true
		muted = append(muted, m)
	}
	s.MutedContainers = muted
	return s
}
