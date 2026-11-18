package model

// RecreateOptions are the explicit choices of ContainerService.Recreate.
type RecreateOptions struct {
	PullFirst bool `json:"pullFirst"`
	// KeepOriginal creates a copy and leaves the original container untouched (needs a new name).
	KeepOriginal bool `json:"keepOriginal"`
}

// AnonymousVolume is a volume of the old container that has no name. It is not
// carried over to the replacement, so its data stays behind in the old volume.
type AnonymousVolume struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

// ContainerRecreateInfo is an editable create spec derived from an existing
// container plus everything the UI must tell the user before replacing it.
type ContainerRecreateInfo struct {
	ID      string              `json:"id"`
	Name    string              `json:"name"`
	Running bool                `json:"running"`
	Spec    ContainerCreateSpec `json:"spec"`
	// ComposeManaged containers belong to a Compose project: replacing them here
	// would desync the project, so the UI offers the Compose paths instead.
	ComposeManaged   bool              `json:"composeManaged"`
	ComposeProject   string            `json:"composeProject,omitempty"`
	ComposeService   string            `json:"composeService,omitempty"`
	AnonymousVolumes []AnonymousVolume `json:"anonymousVolumes"`
	Warnings         []string          `json:"warnings"`
}
