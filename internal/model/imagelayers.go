package model

// ImageLayerStep is one build step of an image, in build order (oldest first).
// Sizes are what the Engine reports in the image history; 0 means the step
// added no data or the Engine did not report one.
type ImageLayerStep struct {
	Index int    `json:"index"` // 1-based, build order
	ID    string `json:"id"`    // may be "<missing>"
	// Kind is RUN, COPY, ADD, ENV, CMD or OTHER.
	Kind string `json:"kind"`
	// Instruction is the cleaned instruction including its keyword; Command is
	// the same without the keyword for the five classified kinds.
	Instruction string `json:"instruction"`
	Command     string `json:"command"`
	Size        int64  `json:"size"`
	// Share is Size divided by the summed step sizes (0..1).
	Share   float64 `json:"share"`
	Created int64   `json:"created"` // unix seconds, 0 = unknown
}

// ImageLayerHint is a heuristic pointer based only on the instruction text.
type ImageLayerHint struct {
	Step   int    `json:"step"` // ImageLayerStep.Index it points at
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// ImageSharedBase names another local image that starts with the same RootFS layers.
type ImageSharedBase struct {
	Count   int    `json:"count"`
	Ref     string `json:"ref"`
	ImageID string `json:"imageId"`
}

// ImageLayersReport feeds the layer explorer.
type ImageLayersReport struct {
	ImageID      string `json:"imageId"`
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Created      string `json:"created,omitempty"`
	// TotalSize is the image size from inspect; HistorySize is the sum of the
	// step sizes. They can differ (e.g. containerd image store).
	TotalSize   int64 `json:"totalSize"`
	HistorySize int64 `json:"historySize"`
	// RootFSLayers is the number of filesystem layers; ContentSteps the number
	// of history steps that report a size above zero.
	RootFSLayers int `json:"rootfsLayers"`
	ContentSteps int `json:"contentSteps"`
	// LargestStep is the Index of the biggest step, 0 when no step reports a size.
	LargestStep int              `json:"largestStep"`
	Steps       []ImageLayerStep `json:"steps"`
	Hints       []ImageLayerHint `json:"hints"`
	// SharedBase is nil when none was found or the check was skipped.
	SharedBase *ImageSharedBase `json:"sharedBase"`
}
