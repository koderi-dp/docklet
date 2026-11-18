package update

import (
	"time"

	"docklet/internal/model"
)

// Interval returns how long to wait between background checks; ok=false means no background checks.
func Interval(mode string) (d time.Duration, ok bool) {
	switch mode {
	case model.UpdateCheckStartup:
		return 24 * time.Hour, true
	case model.UpdateCheckWeekly:
		return 7 * 24 * time.Hour, true
	}
	return 0, false
}

// Due reports whether a background check should run now. lastChecked is zero when never checked.
func Due(mode string, lastChecked, now time.Time) bool {
	d, ok := Interval(mode)
	if !ok {
		return false
	}
	return lastChecked.IsZero() || now.Sub(lastChecked) >= d
}
