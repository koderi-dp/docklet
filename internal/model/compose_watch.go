package model

// EventComposeWatch payload: ComposeWatchChanged. Fired (throttled) when the watch state of a
// project changed; the frontend refetches ComposeService.WatchState.
const EventComposeWatch = "compose:watch"

// ComposeWatchChanged identifies the project whose watch state changed.
type ComposeWatchChanged struct {
	ConnectionID string `json:"connectionId"`
	Project      string `json:"project"`
}

// ComposeWatchRule is one develop.watch entry of a service, from the resolved Compose config.
type ComposeWatchRule struct {
	Action string `json:"action"` // sync | rebuild | sync+restart | sync+exec | restart
	// Path is the watched path as resolved by Compose; DisplayPath is the same relative to the
	// project directory (./site) when it lies inside it.
	Path        string   `json:"path"`
	DisplayPath string   `json:"displayPath"`
	Target      string   `json:"target,omitempty"`
	Ignore      []string `json:"ignore"`
	Exec        string   `json:"exec,omitempty"` // command of sync+exec
}

// ComposeWatchService groups the watch rules of one service.
type ComposeWatchService struct {
	Name  string             `json:"name"`
	Rules []ComposeWatchRule `json:"rules"`
}

// ComposeWatchInfo says whether `docker compose watch` can be started for a project and what it
// would watch.
type ComposeWatchInfo struct {
	Available  bool                  `json:"available"`
	Reason     string                `json:"reason,omitempty"` // why not Available
	CLIVersion string                `json:"cliVersion,omitempty"`
	RuleCount  int                   `json:"ruleCount"`
	Services   []ComposeWatchService `json:"services"`
}

// ComposeWatchOptions are the explicit flags of `compose watch`. An empty Services list watches
// every service that has rules.
type ComposeWatchOptions struct {
	Services []string `json:"services"`
	NoUp     bool     `json:"noUp"`
}

// Kinds of ComposeWatchEvent.
const (
	WatchEventEnabled    = "enabled"
	WatchEventSync       = "sync"
	WatchEventRebuilding = "rebuilding"
	WatchEventRebuilt    = "rebuilt"
	WatchEventRestarted  = "restarted"
	WatchEventError      = "error"
)

// ComposeWatchEvent is one thing the Compose CLI reported while watching.
type ComposeWatchEvent struct {
	ID      int    `json:"id"`
	At      int64  `json:"at"`      // unix ms, when Docklet read the line
	Service string `json:"service"` // empty for project-wide lines
	Kind    string `json:"kind"`
	Text    string `json:"text"`
}

// ComposeWatchStat counts what the CLI reported per service.
type ComposeWatchStat struct {
	Service  string `json:"service"`
	Syncs    int    `json:"syncs"`
	Rebuilds int    `json:"rebuilds"`
	Restarts int    `json:"restarts"`
	LastAt   int64  `json:"lastAt"` // unix ms, 0 = no activity yet
}

// ComposeWatchState is the live (or just ended) watch of one project on one connection.
type ComposeWatchState struct {
	Active      bool     `json:"active"` // a watch run exists (running, or ended with an error)
	Running     bool     `json:"running"`
	Ready       bool     `json:"ready"` // the CLI printed "Watch enabled"
	OperationID string   `json:"operationId,omitempty"`
	StartedAt   int64    `json:"startedAt,omitempty"`
	EndedAt     int64    `json:"endedAt,omitempty"`
	Services    []string `json:"services"` // services limited with, empty = all
	NoUp        bool     `json:"noUp"`
	// Exit is why a watch that ended on its own stopped (last error lines of the CLI).
	Exit string `json:"exit,omitempty"`
	// Events are newest first, capped; Raw holds CLI lines that were not understood, oldest first.
	Events []ComposeWatchEvent `json:"events"`
	Raw    []string            `json:"raw"`
	Stats  []ComposeWatchStat  `json:"stats"`
}
