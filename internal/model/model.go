// Package model holds the shared DTOs that cross the Go/TypeScript boundary.
// Wails generates the TS types from these structs; they are the single source
// of truth. Changes are coordinator-owned.
package model

// ConnectionType enumerates supported transports.
type ConnectionType string

const (
	ConnNamedPipe ConnectionType = "npipe"
	ConnUnix      ConnectionType = "unix"
	ConnTCP       ConnectionType = "tcp"
	ConnSSH       ConnectionType = "ssh"
	ConnWSL       ConnectionType = "wsl"
	ConnContext   ConnectionType = "context"
)

// ConnectionProfile is a saved Docker Engine connection. It never carries
// secrets: passwords/keys live in the OS credential store, referenced by ID.
type ConnectionProfile struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     ConnectionType `json:"type"`
	Endpoint string         `json:"endpoint"` // e.g. npipe:////./pipe/docker_engine, unix:///var/run/docker.sock, tcp://host:2376
	// WSL distribution name when Type == wsl.
	WSLDistro string `json:"wslDistro,omitempty"`
	// TCP/TLS
	TLSVerify  bool   `json:"tlsVerify,omitempty"`
	TLSCertDir string `json:"tlsCertDir,omitempty"`
	// AllowInsecure explicitly opts in to plain (non-TLS) tcp to a non-loopback host.
	AllowInsecure bool `json:"allowInsecure,omitempty"`
	// SSH (all optional; Endpoint stays authoritative for the host).
	SSHUser         string `json:"sshUser,omitempty"`
	SSHPort         int    `json:"sshPort,omitempty"`
	SSHIdentityFile string `json:"sshIdentityFile,omitempty"`
	SSHUseAgent     bool   `json:"sshUseAgent,omitempty"`
	SSHProxyJump    string `json:"sshProxyJump,omitempty"`
	// Discovered=true for auto-detected (not user-saved) profiles.
	Discovered bool `json:"discovered,omitempty"`
}

// ConnectionState is the lifecycle state of a connection.
type ConnectionState string

const (
	StateDisconnected ConnectionState = "disconnected"
	StateConnecting   ConnectionState = "connecting"
	StateConnected    ConnectionState = "connected"
	StateError        ConnectionState = "error"
)

// Capabilities describe what the connected Engine/transport can do. Unsupported
// features must be disabled in the UI with ComposeReason explaining why.
type Capabilities struct {
	CanListResources     bool   `json:"canListResources"`
	CanExecuteContainers bool   `json:"canExecuteContainers"`
	CanStreamLogs        bool   `json:"canStreamLogs"`
	CanStreamStats       bool   `json:"canStreamStats"`
	CanRunCompose        bool   `json:"canRunCompose"`
	CanBuildImages       bool   `json:"canBuildImages"`
	ComposeReason        string `json:"composeReason,omitempty"` // why CanRunCompose is false
}

// ConnectionStatus is the live status of one connection. Fields other than
// ConnectionID/State are only meaningful when State == connected.
type ConnectionStatus struct {
	ConnectionID  string          `json:"connectionId"`
	State         ConnectionState `json:"state"`
	Error         string          `json:"error,omitempty"`       // user-friendly
	Diagnostics   string          `json:"diagnostics,omitempty"` // raw detail, redacted
	ServerVersion string          `json:"serverVersion,omitempty"`
	// Kind is docker or podman, from the Engine's own /version answer; unknown stays docker.
	Kind         EngineKind   `json:"kind,omitempty"`
	APIVersion   string       `json:"apiVersion,omitempty"`
	OS           string       `json:"os,omitempty"`
	Arch         string       `json:"arch,omitempty"`
	NCPU         int          `json:"ncpu,omitempty"`
	MemTotal     int64        `json:"memTotal,omitempty"`
	Containers   int          `json:"containers"`
	Images       int          `json:"images"`
	Volumes      int          `json:"volumes"`
	Networks     int          `json:"networks"`
	Capabilities Capabilities `json:"capabilities"`
}

// Health is the Docker health status. HealthNone means NO healthcheck is
// configured — it must never be shown as "healthy".
type Health string

const (
	HealthNone      Health = "none"
	HealthStarting  Health = "starting"
	HealthHealthy   Health = "healthy"
	HealthUnhealthy Health = "unhealthy"
)

// PortBinding is one exposed port. HostPort==0 means not published.
type PortBinding struct {
	HostIP        string `json:"hostIp,omitempty"`
	HostPort      int    `json:"hostPort,omitempty"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// Container is the list-row read model. Compose fields are copied verbatim from
// Docker labels (com.docker.compose.*); empty means the label is absent.
type Container struct {
	ConnectionID string `json:"connectionId"`
	ID           string `json:"id"`
	Name         string `json:"name"` // without leading slash
	Image        string `json:"image"`
	// State is the raw Engine state: created|running|paused|restarting|removing|exited|dead.
	State  string `json:"state"`
	Status string `json:"status"` // Engine human text, e.g. "Up 2 hours (healthy)"
	Health Health `json:"health"`
	// Unix seconds.
	Created int64         `json:"created"`
	Ports   []PortBinding `json:"ports"`
	// ExitCode is parsed from the "Exited (N) ..." status text; nil unless the container exited.
	ExitCode *int `json:"exitCode,omitempty"`
	// RestartPolicy is "no", "always", ... Only resolved for containers that exited with
	// code 0 (needed to tell a finished one-shot from a stopped service); empty = unknown.
	RestartPolicy string `json:"restartPolicy,omitempty"`

	ComposeProject     string `json:"composeProject,omitempty"`     // com.docker.compose.project
	ComposeService     string `json:"composeService,omitempty"`     // com.docker.compose.service
	ComposeWorkingDir  string `json:"composeWorkingDir,omitempty"`  // com.docker.compose.project.working_dir
	ComposeConfigFiles string `json:"composeConfigFiles,omitempty"` // com.docker.compose.project.config_files
}

// ContainerStat is a point-in-time sample. Nil pointers mean "not collected /
// unavailable" and must render as "—", never as zero.
type ContainerStat struct {
	ID          string   `json:"id"`
	CPUPercent  *float64 `json:"cpuPercent,omitempty"`
	MemoryBytes *uint64  `json:"memoryBytes,omitempty"`
	MemoryLimit *uint64  `json:"memoryLimit,omitempty"`
}

// ActionResult reports the outcome for one container in a batch operation.
// Skipped=true means ineligible for the action (e.g. start on a running one).
type ActionResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ContainerDetail feeds the inspector Overview tab. Environment values for
// secret-like keys are masked server-side (Masked=true); reveal is a separate call.
type ContainerDetail struct {
	Container
	Command       string            `json:"command"`
	Entrypoint    string            `json:"entrypoint"`
	RestartPolicy string            `json:"restartPolicy"`
	StartedAt     string            `json:"startedAt,omitempty"`
	FinishedAt    string            `json:"finishedAt,omitempty"`
	Mounts        []Mount           `json:"mounts"`
	Networks      []string          `json:"networks"`
	Labels        map[string]string `json:"labels"`
	Env           []EnvVar          `json:"env"`
}

type Mount struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"readOnly"`
}

type EnvVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"` // empty when Masked
	Masked bool   `json:"masked"`
}

// LogLine is one parsed log record (timestamps requested from the Engine).
type LogLine struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"` // stdout|stderr
	Text      string `json:"text"`
}

// Event names emitted from Go to the frontend.
const (
	// EventContainersChanged payload: ContainersChanged. Triggers targeted refetch.
	EventContainersChanged = "containers:changed"
	// EventConnectionStatus payload: ConnectionStatus.
	EventConnectionStatus = "connection:status"
)

type ContainersChanged struct {
	ConnectionID string `json:"connectionId"`
}

// WSLDistro describes an installed WSL distribution (from wsl.exe -l -v).
type WSLDistro struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Version int    `json:"version"`
	Default bool   `json:"default"`
}
