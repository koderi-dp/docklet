package model

// ComposeProjectDisk is what one Compose project occupies on the Engine, taken from the
// Engine's own disk accounting (`docker system df -v`). Parts stay separate on purpose:
// image layers are usually shared with other projects and are never "owned" by one.
// A nil size means the Engine reported none (never zero).
type ComposeProjectDisk struct {
	Project string `json:"project"`
	// ContainersBytes sums the writable layers (SizeRw) of the project's containers that report one.
	ContainersBytes *int64 `json:"containersBytes"`
	ContainerCount  int    `json:"containerCount"`
	ContainersKnown int    `json:"containersKnown"` // containers that reported a size
	// VolumesBytes sums UsageData.Size of volumes labelled with the project.
	VolumesBytes *int64 `json:"volumesBytes"`
	VolumeCount  int    `json:"volumeCount"`
	VolumesKnown int    `json:"volumesKnown"`
	// ImagesBytes sums the size of the UNIQUE images the project's containers use (may be shared).
	ImagesBytes *int64 `json:"imagesBytes"`
	ImageCount  int    `json:"imageCount"`
	ImagesKnown int    `json:"imagesKnown"`
}

// ComposeDiskUsage is the per-project disk accounting of one connection.
type ComposeDiskUsage struct {
	ConnectionID string               `json:"connectionId"`
	Projects     []ComposeProjectDisk `json:"projects"`
}
