package services

import (
	"context"
	"encoding/json"
	"time"

	"docklet/internal/model"
	"docklet/internal/update"
)

// CheckForUpdate asks GitHub for the latest release now (the "Check now" button).
// Problems are reported inside the status, never as an error.
func (s *AppService) CheckForUpdate(ctx context.Context) (model.UpdateStatus, error) {
	st := s.Checker.Check(ctx, s.Version)
	if st.Error == "" { // a failed check keeps the last good result
		s.store(ctx, st)
	}
	return s.withDismissed(ctx, st), nil
}

// GetUpdateStatus returns the last stored result, re-evaluated against the running version.
func (s *AppService) GetUpdateStatus(ctx context.Context) (model.UpdateStatus, error) {
	st := s.cached(ctx)
	st.CurrentVersion = s.Version
	st.UpdateAvailable = update.IsNewer(st.LatestVersion, s.Version)
	return s.withDismissed(ctx, st), nil
}

// DismissUpdate hides the "update available" notice for that version only.
func (s *AppService) DismissUpdate(ctx context.Context, version string) error {
	kv, err := s.kv()
	if err != nil {
		return err
	}
	return kv.SetSetting(ctx, updateDismissKey, version)
}

func (s *AppService) withDismissed(ctx context.Context, st model.UpdateStatus) model.UpdateStatus {
	if s.KV != nil && st.LatestVersion != "" {
		if v, ok, _ := s.KV.GetSetting(ctx, updateDismissKey); ok && v == st.LatestVersion {
			st.Dismissed = true
		}
	}
	return st
}

func (s *AppService) cached(ctx context.Context) (st model.UpdateStatus) {
	if s.KV != nil {
		if raw, ok, err := s.KV.GetSetting(ctx, updateStatusKey); err == nil && ok {
			_ = json.Unmarshal([]byte(raw), &st)
		}
	}
	return st
}

func (s *AppService) store(ctx context.Context, st model.UpdateStatus) {
	if s.KV == nil {
		return
	}
	if raw, err := json.Marshal(st); err == nil {
		_ = s.KV.SetSetting(ctx, updateStatusKey, string(raw))
	}
}

// RunScheduledCheck performs the background check when the setting and the last check time say it is due.
// It tells the frontend (once per check) when a not yet dismissed update exists. Returns whether it checked.
func (s *AppService) RunScheduledCheck(ctx context.Context) bool {
	cfg := LoadAppSettings(ctx, s.KV)
	last := time.Time{}
	if at := s.cached(ctx).CheckedAt; at > 0 {
		last = time.UnixMilli(at)
	}
	if !update.Due(cfg.CheckForUpdates, last, s.now()) {
		return false
	}
	st, _ := s.CheckForUpdate(ctx)
	if st.UpdateAvailable && !st.Dismissed && s.Events != nil {
		s.Events.Emit(model.EventAppUpdate, st)
	}
	return true
}

// StartBackgroundChecks runs one scheduled check after the startup delay (20s). It returns at once.
func StartBackgroundChecks(ctx context.Context, s *AppService) {
	go func() {
		select {
		case <-time.After(backgroundCheckIn):
			s.RunScheduledCheck(ctx)
		case <-ctx.Done():
		}
	}()
}
