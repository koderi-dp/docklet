package model

// DockerContextInfo describes one Docker CLI context discovered on disk
// (read-only; the user's docker config is never modified).
type DockerContextInfo struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"` // docker endpoint host, e.g. npipe:////./pipe/x, ssh://u@h
	Current     bool   `json:"current"`  // the CLI's current context
	Description string `json:"description,omitempty"`
	// Type is the connection type an import would produce; empty when the
	// endpoint cannot be imported (Reason says why).
	Type   ConnectionType `json:"type,omitempty"`
	Reason string         `json:"reason,omitempty"`
}

// WSLDockerInfo is the Docker availability inside one WSL distribution.
// A stopped distribution is a state (Running=false), not a failure: it is never
// started implicitly by detection.
type WSLDockerInfo struct {
	Distro          string `json:"distro"`
	Running         bool   `json:"running"`
	CLIFound        bool   `json:"cliFound"`
	DaemonReachable bool   `json:"daemonReachable"`
	ServerVersion   string `json:"serverVersion,omitempty"`
	Detail          string `json:"detail,omitempty"` // human explanation, redacted
}
