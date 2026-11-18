package model

// WSLDockerReport is what one probe of a WSL distribution found. State reuses the
// install states: stopped (the distribution is not running and was NOT started to
// look), running (its own Docker Engine answers), docker-desktop (the docker command
// is Docker Desktop's integration proxy), daemon-stopped, not-installed, unknown.
type WSLDockerReport struct {
	State     WSLInstallState `json:"state"`
	Reachable bool            `json:"reachable"` // a daemon answered (own engine or Docker Desktop)
	Version   string          `json:"version,omitempty"`
	Detail    string          `json:"detail,omitempty"`
	Supported bool            `json:"supported"` // Docker Engine can be installed here automatically
	Reason    string          `json:"reason,omitempty"`
	DocsURL   string          `json:"docsUrl,omitempty"`
}

// WSLDistroReport is one installed distribution with its Docker state.
type WSLDistroReport struct {
	Name     string          `json:"name"`
	Version  int             `json:"version"`
	State    string          `json:"state"` // as wsl.exe reports it, for example Running or Stopped
	Default  bool            `json:"default"`
	Internal bool            `json:"internal"` // Docker Desktop's own distribution; never a connection target
	Docker   WSLDockerReport `json:"docker"`
}

// WSLReport says whether WSL answered and which distributions it has.
type WSLReport struct {
	Available bool              `json:"available"` // at least one distribution is installed
	Detail    string            `json:"detail,omitempty"`
	Distros   []WSLDistroReport `json:"distros"`
}

// LocalEngineReport is one discovered local endpoint and whether it answered a test right now.
type LocalEngineReport struct {
	Profile       ConnectionProfile `json:"profile"`
	Reachable     bool              `json:"reachable"`
	ServerVersion string            `json:"serverVersion,omitempty"`
	Error         string            `json:"error,omitempty"`
}

// EnvironmentReport is the first-run detection result. Every part is a measurement;
// a probe that failed or timed out is reported as such, never as "nothing found".
type EnvironmentReport struct {
	OS       string              `json:"os"` // windows, linux, darwin
	WSL      WSLReport           `json:"wsl"`
	Local    []LocalEngineReport `json:"local"`
	Contexts []DockerContextInfo `json:"contexts"`
	Saved    []ConnectionProfile `json:"saved"` // user-saved connections only
}
