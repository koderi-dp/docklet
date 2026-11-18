package services

import (
	"context"
	"encoding/json"
	"fmt"

	"docklet/internal/model"
)

// SetOnboarding records the first-run setup state (pending, completed or skipped) in the
// settings store, so it survives a webview data reset. Completing or skipping stamps the time;
// reopening the wizard (pending) clears it. Other settings are left as they are.
func (s *AppService) SetOnboarding(ctx context.Context, state string) (model.AppSettings, error) {
	kv, err := s.kv()
	if err != nil {
		return model.AppSettings{}, err
	}
	switch state {
	case model.OnboardingPending, model.OnboardingCompleted, model.OnboardingSkipped:
	default:
		return model.AppSettings{}, fmt.Errorf("unknown onboarding state %q", state)
	}
	cur := LoadAppSettings(ctx, kv)
	cur.Onboarding, cur.OnboardingAt = state, 0
	if state != model.OnboardingPending {
		cur.OnboardingAt = s.now().UnixMilli()
	}
	raw, err := json.Marshal(cur)
	if err != nil {
		return model.AppSettings{}, err
	}
	if err := kv.SetSetting(ctx, appSettingsKey, string(raw)); err != nil {
		return model.AppSettings{}, err
	}
	return cur, nil
}
