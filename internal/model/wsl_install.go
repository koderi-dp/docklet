package model

// WSLInstallState says what Docker looks like inside a distribution, which decides
// what the UI may offer.
type WSLInstallState string

const (
	WSLStateStopped       WSLInstallState = "stopped"        // distribution is not running; nothing was inspected
	WSLStateRunning       WSLInstallState = "running"        // Docker Engine is installed and answering
	WSLStateDaemonStopped WSLInstallState = "daemon-stopped" // Docker Engine is installed but not running
	WSLStateDockerDesktop WSLInstallState = "docker-desktop" // `docker` is Docker Desktop's WSL integration proxy
	WSLStateNotInstalled  WSLInstallState = "not-installed"  // no Docker Engine
	WSLStateUnknown       WSLInstallState = "unknown"        // inspection failed
)

// WSLInstallStep is one command of the install plan, shown to the user before anything runs.
type WSLInstallStep struct {
	Title   string `json:"title"`
	Command string `json:"command"`         // display string, properly quoted
	Input   string `json:"input,omitempty"` // text written to the command's stdin (file content)
	Why     string `json:"why"`
}

// WSLInstallPlan is the read-only result of inspecting a distribution: what is
// there, whether Docker Engine can be installed automatically, and exactly how.
type WSLInstallPlan struct {
	Distro           string           `json:"distro"`
	DistroPretty     string           `json:"distroPretty,omitempty"`
	State            WSLInstallState  `json:"state"`
	Supported        bool             `json:"supported"`
	Reason           string           `json:"reason,omitempty"` // why not supported / what is going on
	DocsURL          string           `json:"docsUrl,omitempty"`
	User             string           `json:"user,omitempty"` // default user added to the docker group (empty for root)
	Systemd          bool             `json:"systemd"`        // systemd is PID 1 now
	DockerDesktop    bool             `json:"dockerDesktop"`  // the docker command is Docker Desktop's integration proxy
	DaemonStart      string           `json:"daemonStart,omitempty"`
	Steps            []WSLInstallStep `json:"steps"`
	SystemdSteps     []WSLInstallStep `json:"systemdSteps"` // extra steps when the user opts into enabling systemd
	PackagesToRemove []string         `json:"packagesToRemove"`
	Notes            []string         `json:"notes"`
}

// WSLInstallOptions are the user's choices in the confirmation dialog.
type WSLInstallOptions struct {
	RemoveConflicting bool `json:"removeConflicting"`
	EnableSystemd     bool `json:"enableSystemd"`
}
