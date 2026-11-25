package model

// ComposeWSLPathIssue is a bind mount of a project on a WSL connection whose source is a
// Windows path, which the Linux engine inside the distribution rejects.
type ComposeWSLPathIssue struct {
	Service string `json:"service"`
	Source  string `json:"source"` // as Compose resolved it, e.g. C:/Source/app
	Target  string `json:"target"`
	// LinuxPath is the same folder as the distribution sees it; empty when it cannot reach it.
	LinuxPath string `json:"linuxPath"`
	// Problem says why no fix is offered for this mount; empty when one is.
	Problem string `json:"problem"`
	// The fix: line Line of File becomes NewLine, and Variable gets LinuxPath in the env file.
	// NewLine equals OldLine when the line already uses the variable.
	File     string `json:"file"`
	Line     int    `json:"line"`
	OldLine  string `json:"oldLine"`
	NewLine  string `json:"newLine"`
	Variable string `json:"variable"`
}

// ComposeWSLPathReport lists the Windows-path mounts of a project and where the fix would
// put the Linux paths. Issues is empty when there is nothing to do.
type ComposeWSLPathReport struct {
	Issues []ComposeWSLPathIssue `json:"issues"`
	// EnvFile is the env file of this connection that receives the Linux paths.
	EnvFile string `json:"envFile"`
	// Fixable is true when at least one issue has a fix.
	Fixable bool `json:"fixable"`
}
