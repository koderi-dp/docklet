package model

// Builder names used by BuildSpec.Builder, BuildPlan.Builder and BuildRecord.Builder.
const (
	BuilderAuto     = "auto"
	BuilderBuildKit = "buildkit"
	BuilderClassic  = "classic"
)

// Build step states (BuildStep.State).
const (
	StepWaiting   = "waiting"
	StepRunning   = "running"
	StepDone      = "done"
	StepCached    = "cached"
	StepError     = "error"
	StepCancelled = "cancelled"
)

// Build results (BuildRecord.Result).
const (
	BuildSuccess   = "success"
	BuildFailed    = "failed"
	BuildCancelled = "cancelled"
)

// BuildCapability says whether the docker CLI can run BuildKit builds for a
// connection, and where that CLI runs.
type BuildCapability struct {
	Available bool `json:"available"`
	// Method is "buildx" (docker buildx build) or "docker-build" (DOCKER_BUILDKIT=1 docker build); empty when unavailable.
	Method  string `json:"method"`
	Version string `json:"version"` // buildx version, or docker CLI version for "docker-build"
	// RawJSON is true when the CLI can report structured progress (--progress=rawjson).
	RawJSON bool `json:"rawJson"`
	// Summary is the one-line explanation shown in the dialog.
	Summary  string `json:"summary"`
	Reason   string `json:"reason"`   // why BuildKit is unavailable
	Location string `json:"location"` // where the CLI runs: "This machine" | "WSL: Ubuntu"
	// ContextHint explains which kind of context path the CLI can read (empty when "any path on this computer").
	ContextHint string `json:"contextHint"`
}

// BuildPlan is what a build with a given spec would do, computed before starting.
type BuildPlan struct {
	Capability BuildCapability `json:"capability"`
	// Builder is the builder that would run: "buildkit" or "classic".
	Builder string `json:"builder"`
	// Summary is the one quiet line shown in the dialog.
	Summary string `json:"summary"`
	// ContextProblem is set when the chosen folder or Dockerfile cannot be used by the chosen builder.
	ContextProblem string `json:"contextProblem"`
	// OfferClassic is true when the Classic builder could build despite ContextProblem or missing BuildKit.
	OfferClassic bool `json:"offerClassic"`
}

// BuildStep is one step (BuildKit vertex) of a build.
type BuildStep struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	State      string   `json:"state"`
	StartedAt  int64    `json:"startedAt"`  // unix ms, local clock; 0 when not started
	DurationMs int64    `json:"durationMs"` // 0 until finished
	Internal   bool     `json:"internal"`   // setup/export steps that are not Dockerfile instructions
	Error      string   `json:"error"`
	Log        []string `json:"log"`
}

// BuildProgress is the live (or last known) step list of one build operation.
type BuildProgress struct {
	OperationID string      `json:"operationId"`
	Steps       []BuildStep `json:"steps"`
	// Total/Done/Cached/Failed count Dockerfile steps only (Internal steps excluded).
	Total   int  `json:"total"`
	Done    int  `json:"done"`
	Cached  int  `json:"cached"`
	Failed  int  `json:"failed"`
	Plain   bool `json:"plain"` // parsed from plain-text progress (best effort)
	Running bool `json:"running"`
}

// BuildRecord is one build started from Docklet, stored locally. It never holds
// build-argument values, only their names.
type BuildRecord struct {
	ID           string      `json:"id"`
	ConnectionID string      `json:"connectionId"`
	StartedAt    int64       `json:"startedAt"`
	FinishedAt   int64       `json:"finishedAt"`
	DurationMs   int64       `json:"durationMs"`
	Result       string      `json:"result"`
	Builder      string      `json:"builder"`
	Method       string      `json:"method"` // "buildx 0.38.1", "docker build 20.10.24" or "Engine API"
	ContextDir   string      `json:"contextDir"`
	Dockerfile   string      `json:"dockerfile"`
	Tags         []string    `json:"tags"`
	Target       string      `json:"target"`
	Platform     string      `json:"platform"`
	NoCache      bool        `json:"noCache"`
	Pull         bool        `json:"pull"`
	ArgNames     []string    `json:"argNames"`
	ImageID      string      `json:"imageId"`
	StepsTotal   int         `json:"stepsTotal"`
	StepsCached  int         `json:"stepsCached"`
	Steps        []BuildStep `json:"steps"`
	LogTail      []string    `json:"logTail"` // last lines of a failed build
	Error        string      `json:"error"`
	Command      string      `json:"command"` // equivalent docker command, argument values replaced by $NAME
}
