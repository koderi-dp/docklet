package model

// Events for live container log streams.
const (
	// EventContainerLogsData payload: ContainerLogsData.
	EventContainerLogsData = "containerlogs:data"
	// EventContainerLogsEnd payload: ContainerLogsEnd.
	EventContainerLogsEnd = "containerlogs:end"
)

// ContainerLogsData is one batch of log lines of a live stream.
type ContainerLogsData struct {
	StreamID string    `json:"streamId"`
	Lines    []LogLine `json:"lines"`
}

// ContainerLogsEnd is emitted when a live stream ends on its own (container
// stopped, Engine connection lost, ...). It is not emitted after StopLogStream.
type ContainerLogsEnd struct {
	StreamID string `json:"streamId"`
	Error    string `json:"error,omitempty"`
}

// ContainerStatDetail is one detailed stats sample. Nil pointers mean the value
// is not available on this Engine/OS and must render as "Not available", never 0.
type ContainerStatDetail struct {
	ID              string   `json:"id"`
	Timestamp       string   `json:"timestamp,omitempty"`
	CPUPercent      *float64 `json:"cpuPercent,omitempty"`
	MemoryBytes     *uint64  `json:"memoryBytes,omitempty"`
	MemoryLimit     *uint64  `json:"memoryLimit,omitempty"`
	NetRxBytes      *uint64  `json:"netRxBytes,omitempty"`
	NetTxBytes      *uint64  `json:"netTxBytes,omitempty"`
	BlockReadBytes  *uint64  `json:"blockReadBytes,omitempty"`
	BlockWriteBytes *uint64  `json:"blockWriteBytes,omitempty"`
	Pids            *uint64  `json:"pids,omitempty"`
}

// ContainerProcesses is the Engine "top" output.
type ContainerProcesses struct {
	Titles    []string   `json:"titles"`
	Processes [][]string `json:"processes"`
}
