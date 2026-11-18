package model

// Events emitted by the image service.
const (
	// EventImagesChanged payload: ImagesChanged. Triggers a targeted refetch.
	EventImagesChanged = "images:changed"
)

type ImagesChanged struct {
	ConnectionID string `json:"connectionId"`
}

// ImageRow is one row of the image list: one row per repo:tag, plus one row for
// each image without any tag (Dangling). Rows of the same image share ID,
// Size, InUse and ContainerCount.
type ImageRow struct {
	ConnectionID string `json:"connectionId"`
	// Key is unique within a list: image ID plus the tag reference.
	Key string `json:"key"`
	ID  string `json:"id"` // full image ID, e.g. sha256:...
	// Ref is what to pass back to the service for this row: "repo:tag", or the image ID when dangling.
	Ref        string `json:"ref"`
	Repository string `json:"repository"` // empty when dangling
	Tag        string `json:"tag"`        // empty when dangling
	Dangling   bool   `json:"dangling"`
	Created    int64  `json:"created"` // unix seconds
	Size       int64  `json:"size"`    // bytes
	// InUse is true when at least one container (running or stopped) of the same
	// connection uses the image. ContainerCount is how many.
	InUse          bool `json:"inUse"`
	ContainerCount int  `json:"containerCount"`
}

// ImageContainerRef is a container that uses an image.
type ImageContainerRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// ImageDetail feeds the image inspector. Environment values are never returned,
// only their count, because image env can contain secrets.
type ImageDetail struct {
	ID           string              `json:"id"`
	Tags         []string            `json:"tags"`
	Digests      []string            `json:"digests"`
	Created      string              `json:"created,omitempty"` // RFC3339 as reported by the Engine
	Architecture string              `json:"architecture"`
	OS           string              `json:"os"`
	Variant      string              `json:"variant,omitempty"`
	Size         int64               `json:"size"`
	EnvCount     int                 `json:"envCount"`
	Entrypoint   []string            `json:"entrypoint"`
	Cmd          []string            `json:"cmd"`
	ExposedPorts []string            `json:"exposedPorts"`
	WorkingDir   string              `json:"workingDir,omitempty"`
	User         string              `json:"user,omitempty"`
	Labels       map[string]string   `json:"labels"`
	LayerCount   int                 `json:"layerCount"`
	Containers   []ImageContainerRef `json:"containers"`
}

// ImageLayer is one history entry (newest first, as reported by the Engine).
type ImageLayer struct {
	ID        string   `json:"id"` // may be "<missing>"
	Created   int64    `json:"created"`
	CreatedBy string   `json:"createdBy"`
	Size      int64    `json:"size"`
	Tags      []string `json:"tags"`
}

// RegistryAuthStatus tells the UI whether Docker CLI credentials exist for a
// registry. It never carries any secret, not even the user name.
type RegistryAuthStatus struct {
	Registry       string `json:"registry"` // normalized host, Docker Hub is "docker.io"
	HasCredentials bool   `json:"hasCredentials"`
	// Source is "config", "helper:<name>" or "none".
	Source string `json:"source"`
}

// BuildSpec describes an image build. The context directory is read on the
// machine running Docklet and uploaded to the selected Engine.
type BuildSpec struct {
	ContextDir string `json:"contextDir"`
	// Dockerfile is relative to ContextDir (default "Dockerfile").
	Dockerfile string            `json:"dockerfile"`
	Tags       []string          `json:"tags"`
	BuildArgs  map[string]string `json:"buildArgs"`
	Target     string            `json:"target"`
	Platform   string            `json:"platform"` // e.g. linux/amd64, empty = Engine default
	NoCache    bool              `json:"noCache"`
	Pull       bool              `json:"pull"`
	// Builder is "auto" (default), "buildkit" (docker CLI) or "classic" (Engine API).
	Builder string `json:"builder"`
}
