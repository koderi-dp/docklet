package model

// Update check schedule values for AppSettings.CheckForUpdates.
const (
	UpdateCheckNever   = "never"
	UpdateCheckStartup = "startup"
	UpdateCheckWeekly  = "weekly"
)

// Events for the app shell. EventAppUpdate payload: UpdateStatus (an update was found).
// EventTrayOpen payload: TrayOpen. EventTraySelectConnection payload: TraySelectConnection.
const (
	EventAppUpdate            = "app:update"
	EventTrayOpen             = "tray:open"
	EventTraySelectConnection = "tray:select-connection"
)

// Targets of EventTrayOpen.
const (
	TrayTargetAlerts    = "alerts"
	TrayTargetContainer = "container"
)

// AppSettings are the tray and update preferences. Everything is off by default.
type AppSettings struct {
	ShowTrayIcon         bool   `json:"showTrayIcon"`
	CloseToTray          bool   `json:"closeToTray"`
	StartMinimizedToTray bool   `json:"startMinimizedToTray"`
	CheckForUpdates      string `json:"checkForUpdates"` // never | startup | weekly
	// Onboarding is the first-run setup state: "" (never decided), pending, completed or skipped.
	// It is owned by SetOnboarding; SaveAppSettings keeps the stored value.
	Onboarding   string `json:"onboarding"`
	OnboardingAt int64  `json:"onboardingAt"` // unix milliseconds of completion or skip; 0 otherwise
}

// First-run setup states for AppSettings.Onboarding.
const (
	OnboardingPending   = "pending"
	OnboardingCompleted = "completed"
	OnboardingSkipped   = "skipped"
)

// UpdateStatus is the outcome of a check against the latest GitHub release.
type UpdateStatus struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	ReleaseNotes    string `json:"releaseNotes"`
	PublishedAt     string `json:"publishedAt"`
	CheckedAt       int64  `json:"checkedAt"` // unix milliseconds; 0 = never checked
	Dismissed       bool   `json:"dismissed"` // the user dismissed this version's notice
	Message         string `json:"message"`   // friendly, non-alarming info (e.g. no releases yet)
	Error           string `json:"error"`     // the check could not complete (offline, rate limited)
}

// TrayOpen asks the frontend to open something: Target is "alerts" or "container".
type TrayOpen struct {
	Target       string `json:"target"`
	ConnectionID string `json:"connectionId"`
	ContainerID  string `json:"containerId"`
}

// TraySelectConnection asks the frontend to select and connect an engine.
type TraySelectConnection struct {
	ConnectionID string `json:"connectionId"`
}
