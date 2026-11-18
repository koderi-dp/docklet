package model

// EventAlertsNew payload: AlertNew. Emitted for every recorded container alert.
const EventAlertsNew = "alerts:new"

// Alert rule identifiers.
const (
	RuleCrash       = "crash"
	RuleOOM         = "oom"
	RuleUnhealthy   = "unhealthy"
	RuleRestartLoop = "restartLoop"
)

// Alert severities.
const (
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// Alert is one container health notification.
type Alert struct {
	ID             string `json:"id"`
	ConnectionID   string `json:"connectionId"`
	ConnectionName string `json:"connectionName"`
	ContainerID    string `json:"containerId"`
	ContainerName  string `json:"containerName"`
	ComposeProject string `json:"composeProject"`
	Rule           string `json:"rule"`
	Severity       string `json:"severity"`
	Title          string `json:"title"`
	Detail         string `json:"detail"`
	At             int64  `json:"at"` // unix milliseconds
	Read           bool   `json:"read"`
}

// AlertNew is the payload of EventAlertsNew.
type AlertNew struct {
	Alert Alert `json:"alert"`
}

// AlertRules are the per-rule switches.
type AlertRules struct {
	Crash       bool `json:"crash"`
	OOM         bool `json:"oom"`
	Unhealthy   bool `json:"unhealthy"`
	RestartLoop bool `json:"restartLoop"`
}

// MutedContainer silences alerts for one container name on one connection.
type MutedContainer struct {
	ConnectionID  string `json:"connectionId"`
	ContainerName string `json:"containerName"`
}

// NotificationSettings configure container health alerts. Everything is off by default.
type NotificationSettings struct {
	Enabled         bool             `json:"enabled"`
	Rules           AlertRules       `json:"rules"`
	OSNotifications bool             `json:"osNotifications"`
	OSAvailable     bool             `json:"osAvailable"` // read-only: an OS notifier is wired in
	MutedContainers []MutedContainer `json:"mutedContainers"`
}
