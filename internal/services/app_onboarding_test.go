package services

import (
	"context"
	"testing"
	"time"

	"docklet/internal/model"
)

func TestSaveAppSettings_KeepsOnboardingStateFromAStaleCopy(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	s, _ := newApp(newReleaseServer(t, "v0.2.0"), &now)
	if _, err := s.SetOnboarding(context.Background(), model.OnboardingCompleted); err != nil {
		t.Fatal(err)
	}
	got, err := s.SaveAppSettings(context.Background(), model.AppSettings{ShowTrayIcon: true, Onboarding: model.OnboardingPending})
	if err != nil {
		t.Fatal(err)
	}
	if got.Onboarding != model.OnboardingCompleted || got.OnboardingAt != now.UnixMilli() || !got.ShowTrayIcon {
		t.Fatalf("got %+v", got)
	}
}

func TestSetOnboarding_StampsFinishedStatesAndClearsOnReopen(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	s, _ := newApp(newReleaseServer(t, "v0.2.0"), &now)
	done, _ := s.SetOnboarding(context.Background(), model.OnboardingSkipped)
	if done.Onboarding != model.OnboardingSkipped || done.OnboardingAt == 0 {
		t.Fatalf("skipped: %+v", done)
	}
	again, _ := s.SetOnboarding(context.Background(), model.OnboardingPending)
	if again.Onboarding != model.OnboardingPending || again.OnboardingAt != 0 {
		t.Fatalf("reopened: %+v", again)
	}
	if _, err := s.SetOnboarding(context.Background(), "later"); err == nil {
		t.Fatal("an unknown state must be rejected")
	}
}
