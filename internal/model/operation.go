package model

// OperationStatus is the lifecycle of a background operation.
type OperationStatus string

const (
	OpRunning   OperationStatus = "running"
	OpSucceeded OperationStatus = "succeeded"
	OpFailed    OperationStatus = "failed"
	OpCancelled OperationStatus = "cancelled"
)

// Operation is a long-running background task (pull, build, compose up, prune...).
// Progress is nil unless the Engine/CLI reports a real figure; never fabricate one.
type Operation struct {
	ID           string          `json:"id"`
	Kind         string          `json:"kind"`  // e.g. "image.pull", "compose.up"
	Title        string          `json:"title"` // human label, e.g. "Pull nginx:latest"
	ConnectionID string          `json:"connectionId"`
	Resource     string          `json:"resource"` // image ref, project name...
	Status       OperationStatus `json:"status"`
	Progress     *float64        `json:"progress,omitempty"` // 0..1
	Detail       string          `json:"detail,omitempty"`   // latest status line
	Error        string          `json:"error,omitempty"`
	StartedAt    int64           `json:"startedAt"`            // unix ms
	FinishedAt   int64           `json:"finishedAt,omitempty"` // unix ms
	Cancellable  bool            `json:"cancellable"`
}

// OperationLog is an incremental batch of log lines for one operation.
type OperationLog struct {
	ID    string   `json:"id"`
	Lines []string `json:"lines"`
}

const (
	// EventOperationUpdated payload: Operation (throttled per operation).
	EventOperationUpdated = "operation:updated"
	// EventOperationLog payload: OperationLog.
	EventOperationLog = "operation:log"
)
