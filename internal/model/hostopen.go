package model

// LocalPath says where a path reported by an Engine (a bind mount source, a Compose working
// directory) lives on THIS computer. LocalPath is only meaningful when Exists is true.
type LocalPath struct {
	LocalPath string `json:"localPath"`
	Exists    bool   `json:"exists"`
	IsDir     bool   `json:"isDir"`
	// Reason explains why the path is not available here (remote engine, inside a VM, missing).
	Reason string `json:"reason,omitempty"`
}

// EditorSetting selects the editor for "Open in editor". Mode is "auto", "vscode", "cursor" or
// "custom"; a custom Template is a command line that must contain {path}, e.g. `zed {path}`.
type EditorSetting struct {
	Mode     string `json:"mode"`
	Template string `json:"template"`
}

// EditorLaunch is the outcome of OpenInEditor. NeedsSetting is true when no editor could be
// found (nothing was started); Message then says what to do.
type EditorLaunch struct {
	Started      bool   `json:"started"`
	Editor       string `json:"editor,omitempty"`
	NeedsSetting bool   `json:"needsSetting,omitempty"`
	Message      string `json:"message,omitempty"`
}
