package model

// ComposeClonePreview tells the Clone dialog what copying a project would involve.
type ComposeClonePreview struct {
	SourceDir     string `json:"sourceDir"`
	SuggestedName string `json:"suggestedName"`
	SuggestedDir  string `json:"suggestedDir"`
	// SizeBytes and Files describe the folder that is copied; SizeKnown is false when it
	// could not be measured in time.
	SizeBytes int64 `json:"sizeBytes"`
	Files     int   `json:"files"`
	SizeKnown bool  `json:"sizeKnown"`
	// Running is true when the original has running containers (files may change while copied).
	Running bool `json:"running"`
	// Outside lists Compose and env files outside the folder: the copy keeps using them.
	Outside []string `json:"outside"`
	// Collisions are sentences about what the copy shares with the original or cannot hold at
	// the same time (host ports, container names, fixed volumes).
	Collisions []string `json:"collisions"`
	// Blocked is the reason the project cannot be cloned; empty when it can.
	Blocked string `json:"blocked"`
}
