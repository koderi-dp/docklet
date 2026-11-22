package model

// ComposeFileRef names one file of a Compose project that the editor may show.
type ComposeFileRef struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // "config" or "env"
}

// ComposeFileList is the set of files of a project and whether they can be edited.
type ComposeFileList struct {
	Files   []ComposeFileRef `json:"files"`
	CanEdit bool             `json:"canEdit"`
	Reason  string           `json:"reason"`
}

// ComposeFileContent is one file read for the editor.
type ComposeFileContent struct {
	Path     string `json:"path"`
	Text     string `json:"text"`
	Version  string `json:"version"`
	ReadOnly bool   `json:"readOnly"`
}

// ComposeYAMLError is a YAML syntax error with its 1-based line.
type ComposeYAMLError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ComposeOutlineItem is one service and the 1-based line of its key.
type ComposeOutlineItem struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

// ComposeFileOutline lists the services of a file, or the error that stops it.
type ComposeFileOutline struct {
	Services []ComposeOutlineItem `json:"services"`
	Error    *ComposeYAMLError    `json:"error"`
}

// ComposeApplyItem is what `compose up` would do to one service. Action is one of
// "start", "recreate", "recreateImage", "new", "remove", "unchanged".
type ComposeApplyItem struct {
	Service string `json:"service"`
	Action  string `json:"action"`
	Detail  string `json:"detail"`
}

// ComposeApplyPlan is the list of changes Apply would make, or the reason it could not be worked out.
type ComposeApplyPlan struct {
	Items []ComposeApplyItem `json:"items"`
	Error string             `json:"error"`
}

// ComposeSaveResult is the outcome of saving a file.
type ComposeSaveResult struct {
	Saved    bool              `json:"saved"`
	Version  string            `json:"version"`
	Conflict bool              `json:"conflict"`
	Error    *ComposeYAMLError `json:"error"`
}
