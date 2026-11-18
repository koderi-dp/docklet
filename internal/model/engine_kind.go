package model

// EngineKind says which container engine answers a connection. Podman serves the Docker API
// through its compatibility layer, so most of Docklet works the same; the kind lets the UI be
// honest about the few things that differ. Unknown engines are treated as Docker.
type EngineKind string

const (
	EngineDocker EngineKind = "docker"
	EnginePodman EngineKind = "podman"
)

// PodmanMachineInfo is one Podman machine (Windows and macOS) as `podman machine list` reports it.
// Docklet only reads this; it never starts, stops or changes a machine.
type PodmanMachineInfo struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	// Hint says what the user has to do when the machine is not running.
	Hint string `json:"hint,omitempty"`
	// Profile is the connection that reaches the machine (named pipe or socket).
	Profile ConnectionProfile `json:"profile"`
}
