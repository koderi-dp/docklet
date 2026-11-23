package model

// ComposeNewSource says what a new Compose file starts from.
type ComposeNewSource struct {
	Kind string `json:"kind"` // minimal | template | containers
	// Template is the template id when Kind is "template".
	Template string `json:"template,omitempty"`
	// ContainerIDs are the containers to convert when Kind is "containers".
	ContainerIDs []string `json:"containerIds"`
	// IncludeSecrets writes secret-looking values instead of ${VAR} placeholders.
	IncludeSecrets bool `json:"includeSecrets"`
}

// ComposeNewPreview is the exact text a new Compose file would get, and what it leaves out.
type ComposeNewPreview struct {
	Text        string   `json:"text"`
	SecretKeys  []string `json:"secretKeys"`
	NotIncluded []string `json:"notIncluded"`
}

// ComposeCreateResult is the outcome of creating a new Compose file. Exactly one field is set.
type ComposeCreateResult struct {
	Project *ComposeProject `json:"project,omitempty"`
	// ExistingFile is the Compose file already in the folder; nothing was written.
	ExistingFile string `json:"existingFile,omitempty"`
}

// ComposeTemplate is a ready-made starting file for a new Compose project.
type ComposeTemplate struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
