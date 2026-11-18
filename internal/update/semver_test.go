package update

import (
	"testing"
	"time"

	"docklet/internal/model"
)

func TestCompare_OrdersSemanticVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.9.0", "0.10.0", -1},
		{"1.0.0", "1.0.0", 0},
		{"v1.2.3", "1.2.3", 0},
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-1", "1.0.0-alpha", -1},
		{"2.0.0", "1.99.99", 1},
		{"1.0.0+build5", "1.0.0", 0},
	}
	for _, c := range cases {
		got, ok := Compare(c.a, c.b)
		if !ok || got != c.want {
			t.Errorf("Compare(%q,%q) = %d,%v want %d", c.a, c.b, got, ok, c.want)
		}
	}
}

func TestIsNewer_DevAndInvalidBuildsNeverReportAnUpdate(t *testing.T) {
	for _, current := range []string{"dev", "", "1.2", "banana"} {
		if IsNewer("9.9.9", current) {
			t.Errorf("current %q must never report an update", current)
		}
	}
	if !IsNewer("v0.2.0", "0.1.0") || IsNewer("0.1.0", "0.1.0") || IsNewer("1.0.0-rc.1", "1.0.0") {
		t.Error("ordinary comparisons are wrong")
	}
}

func TestDue_FollowsTheChosenSchedule(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		mode string
		last time.Time
		want bool
	}{
		{model.UpdateCheckNever, time.Time{}, false},
		{"", time.Time{}, false},
		{model.UpdateCheckStartup, time.Time{}, true},
		{model.UpdateCheckStartup, now.Add(-23 * time.Hour), false},
		{model.UpdateCheckStartup, now.Add(-25 * time.Hour), true},
		{model.UpdateCheckWeekly, now.Add(-3 * 24 * time.Hour), false},
		{model.UpdateCheckWeekly, now.Add(-7 * 24 * time.Hour), true},
	}
	for _, c := range cases {
		if got := Due(c.mode, c.last, now); got != c.want {
			t.Errorf("Due(%q, %v) = %v want %v", c.mode, c.last, got, c.want)
		}
	}
}
