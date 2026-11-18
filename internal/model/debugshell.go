package model

// DebugShellOptions configures a temporary toolbox container that shares a
// running container's namespaces.
type DebugShellOptions struct {
	// Image is the toolbox image; empty means busybox:latest.
	Image string `json:"image"`
	// SysPtrace adds the single SYS_PTRACE capability (process inspection, strace).
	SysPtrace bool `json:"sysPtrace"`
	Cols      int  `json:"cols"`
	Rows      int  `json:"rows"`
}

// DebugShellResult is the outcome of opening a debug shell. When ImageMissing is
// set nothing was created and SessionID is empty: the toolbox image is not on
// the Engine and the caller may offer to pull it.
type DebugShellResult struct {
	SessionID    string   `json:"sessionId"`
	Shell        string   `json:"shell"`
	Image        string   `json:"image"`
	SidecarName  string   `json:"sidecarName"`
	ImageMissing bool     `json:"imageMissing"`
	SharesIPC    bool     `json:"sharesIpc"`
	Notices      []string `json:"notices"`
}
