package model

// WSL helper actions. The IDs are the contract between the UI and WSLService.
const (
	WSLActionReclaimMemory     = "reclaim-memory"
	WSLActionRestartDocker     = "restart-docker"
	WSLActionRestartDistro     = "restart-distro"
	WSLActionShutdown          = "shutdown"
	WSLDockerRunning           = "running"
	WSLDockerStopped           = "stopped"
	WSLDockerNotInstalled      = "not-installed"
	WSLDockerDesktopIntegrated = "docker-desktop"
)

// WSLAction describes one maintenance action as it is available right now.
type WSLAction struct {
	ID       string `json:"id"`
	Command  string `json:"command"`  // what runs, for the confirmation dialog
	Disabled string `json:"disabled"` // empty when the action can run; otherwise why it cannot
}

// WSLInfo is the live state of the WSL distribution behind a connection. Every
// figure that could not be read is nil (or empty) and shown as "—", never as 0.
type WSLInfo struct {
	ConnectionID string `json:"connectionId"`
	Distro       string `json:"distro"`
	Running      bool   `json:"running"`
	Version      int    `json:"version"` // 1 or 2; 0 when unknown
	IsDefault    bool   `json:"isDefault"`
	OSName       string `json:"osName"`
	// SystemdEnabled is the [boot] systemd setting of /etc/wsl.conf; SystemdRunning is what PID 1 is now.
	SystemdEnabled *bool  `json:"systemdEnabled"`
	SystemdRunning *bool  `json:"systemdRunning"`
	Docker         string `json:"docker"` // WSLDocker* values; empty when unknown

	MemTotal *int64 `json:"memTotal"` // inside the distribution
	MemUsed  *int64 `json:"memUsed"`  // total - available, as `free` reports it
	MemCache *int64 `json:"memCache"` // buffers + page cache + reclaimable slab
	// VMMemory is the working set of the vmmem/vmmemWSL process: the whole WSL VM, all distributions.
	VMMemory *int64 `json:"vmMemory"`

	DiskSize *int64 `json:"diskSize"` // root filesystem
	DiskUsed *int64 `json:"diskUsed"`
	VHDXPath string `json:"vhdxPath"`
	VHDXSize *int64 `json:"vhdxSize"` // size of ext4.vhdx on Windows

	OtherRunning []string    `json:"otherRunning"` // other running distributions: names
	Actions      []WSLAction `json:"actions"`
	Notes        []string    `json:"notes"` // what could not be read, and why
}
