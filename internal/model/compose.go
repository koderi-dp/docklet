package model

// ComposeSourceLabels is the Source of projects discovered from container labels.
const ComposeSourceLabels = "Discovered from labels"

// Sources of managed projects (a saved record that supplies the Compose files).
const (
	ComposeSourceManagedLocal = "Managed from local files"
	ComposeSourceManagedWSL   = "WSL files"
)

// ComposeCapability says whether the Compose CLI can operate a project, where it
// would run and why not when it cannot. Labels alone never make a project operable.
type ComposeCapability struct {
	CanRun bool   `json:"canRun"`
	Reason string `json:"reason,omitempty"` // why CanRun is false
	// ExecutionLocation is where the CLI runs: "This machine" or "WSL: <distro>".
	ExecutionLocation string `json:"executionLocation"`
	// Warning is shown even when CanRun (e.g. relative bind mounts resolve on a remote host).
	Warning     string   `json:"warning,omitempty"`
	ConfigFiles []string `json:"configFiles"`
	WorkingDir  string   `json:"workingDir,omitempty"`
}

// ComposeContainerRef is a container that belongs to a service.
type ComposeContainerRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	OneOff bool   `json:"oneOff,omitempty"`
}

// ComposeServiceInfo is one service (exact com.docker.compose.service label) of a project.
type ComposeServiceInfo struct {
	Name       string                `json:"name"`
	Image      string                `json:"image"`
	Running    int                   `json:"running"`
	Total      int                   `json:"total"`
	Containers []ComposeContainerRef `json:"containers"`
}

// ComposeProject is a project discovered from container labels of one Engine.
type ComposeProject struct {
	ConnectionID string `json:"connectionId"`
	Name         string `json:"name"`
	Source       string `json:"source"`
	Running      int    `json:"running"`
	Stopped      int    `json:"stopped"` // exited | created | dead, excluding Completed
	// Completed counts one-shot containers that exited with code 0 (restart policy "no" or
	// Compose one-off); Failed counts exited ones with a non-zero code (also in Stopped).
	Completed    int                  `json:"completed"`
	Failed       int                  `json:"failed"`
	Total        int                  `json:"total"`
	Services     []ComposeServiceInfo `json:"services"`
	ContainerIDs []string             `json:"containerIds"`
	ConfigFiles  []string             `json:"configFiles"`
	EnvFiles     []string             `json:"envFiles"`
	WorkingDir   string               `json:"workingDir,omitempty"`
	Capability   ComposeCapability    `json:"capability"`
	// Managed is true when a saved record (user-selected Compose files) backs the project.
	Managed bool `json:"managed"`
	// NotCreated is true for a managed project with no containers on the Engine yet.
	NotCreated bool `json:"notCreated"`
}

// ComposeUpOptions are the explicit flags of `compose up`. Up always detaches (-d).
type ComposeUpOptions struct {
	Build         bool     `json:"build"`
	RemoveOrphans bool     `json:"removeOrphans"`
	Profiles      []string `json:"profiles"`
}

// ComposeDownOptions are the explicit flags of `compose down`. Volumes deletes data
// and must only be true when the user ticked the dedicated checkbox.
type ComposeDownOptions struct {
	Volumes       bool     `json:"volumes"`
	RemoveOrphans bool     `json:"removeOrphans"`
	Profiles      []string `json:"profiles"`
}

// ComposeLocation describes where the Compose CLI runs for a connection and how file
// paths must be written there. It drives the "Open Compose File" dialog.
type ComposeLocation struct {
	Available         bool   `json:"available"`
	Kind              string `json:"kind"` // local | wsl | none
	ExecutionLocation string `json:"executionLocation"`
	Reason            string `json:"reason,omitempty"`
	Warning           string `json:"warning,omitempty"`
	PathNote          string `json:"pathNote,omitempty"`
	// ManagedAvailable is false when the application database is not open.
	ManagedAvailable bool `json:"managedAvailable"`
}

// ComposeConfigResult is the Configuration tab payload.
type ComposeConfigResult struct {
	// Rendered is `docker compose config` output with secret-like values masked; empty when unavailable.
	Rendered  string   `json:"rendered,omitempty"`
	Available bool     `json:"available"`
	Files     []string `json:"files"`
	Note      string   `json:"note,omitempty"`
}

// ComposeNetwork / ComposeVolume are resources labelled with the project.
type ComposeNetwork struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}

type ComposeVolume struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Mountpoint string `json:"mountpoint"`
}

type ComposeResources struct {
	Networks []ComposeNetwork `json:"networks"`
	Volumes  []ComposeVolume  `json:"volumes"`
}

// ComposeLogLine is one merged project log record.
type ComposeLogLine struct {
	Service   string `json:"service"`
	Container string `json:"container"`
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"`
	Text      string `json:"text"`
}
