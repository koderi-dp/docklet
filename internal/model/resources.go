package model

// DTOs for the Volumes, Networks and System workstream. Pointers are used for
// figures the Engine may omit: nil must render as "—"/"Not available", never 0.

const (
	// EventVolumesChanged payload: VolumesChanged.
	EventVolumesChanged = "volumes:changed"
	// EventNetworksChanged payload: NetworksChanged.
	EventNetworksChanged = "networks:changed"
)

type VolumesChanged struct {
	ConnectionID string `json:"connectionId"`
}

type NetworksChanged struct {
	ConnectionID string `json:"connectionId"`
}

// ResourceUser is a container that references a volume or network.
type ResourceUser struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Destination string `json:"destination,omitempty"` // volume mount path
	ReadOnly    bool   `json:"readOnly,omitempty"`
	IPv4        string `json:"ipv4,omitempty"` // network endpoint address
	IPv6        string `json:"ipv6,omitempty"`
}

// Volume is a list/inspect row. InUseBy is computed from the container mounts of
// the same connection (stopped containers included, as Docker refuses removal).
type Volume struct {
	ConnectionID string            `json:"connectionId"`
	Name         string            `json:"name"`
	Driver       string            `json:"driver"`
	Mountpoint   string            `json:"mountpoint"`
	Scope        string            `json:"scope"`
	Labels       map[string]string `json:"labels"`
	Options      map[string]string `json:"options"`
	Created      string            `json:"created,omitempty"` // RFC3339 as reported; empty when not provided
	InUseBy      []ResourceUser    `json:"inUseBy"`
}

// IPAMConfig is one IPAM pool exactly as reported.
type IPAMConfig struct {
	Subnet  string `json:"subnet,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	IPRange string `json:"ipRange,omitempty"`
}

// Network is a list row. Attached is computed from the containers' network
// settings of the same connection.
type Network struct {
	ConnectionID string            `json:"connectionId"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Driver       string            `json:"driver"`
	Scope        string            `json:"scope"`
	Internal     bool              `json:"internal"`
	Attachable   bool              `json:"attachable"`
	EnableIPv6   bool              `json:"enableIPv6"`
	Predefined   bool              `json:"predefined"` // bridge/host/none etc.: cannot be removed
	Created      string            `json:"created,omitempty"`
	Subnets      []string          `json:"subnets"`
	Gateways     []string          `json:"gateways"`
	Labels       map[string]string `json:"labels"`
	Attached     []ResourceUser    `json:"attached"`
}

// NetworkDetail feeds the network inspector (Engine inspect, authoritative for endpoints).
type NetworkDetail struct {
	Network
	IPAMDriver string            `json:"ipamDriver,omitempty"`
	IPAM       []IPAMConfig      `json:"ipam"`
	Options    map[string]string `json:"options"`
}

// NetworkCreateRequest is validated server-side.
type NetworkCreateRequest struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"` // default bridge
	Subnet     string            `json:"subnet,omitempty"`
	Gateway    string            `json:"gateway,omitempty"`
	Internal   bool              `json:"internal"`
	Attachable bool              `json:"attachable"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// EngineInfo is the System > Overview read model.
type EngineInfo struct {
	ConnectionID      string     `json:"connectionId"`
	Name              string     `json:"name"`
	ServerVersion     string     `json:"serverVersion"`
	Kind              EngineKind `json:"kind,omitempty"`
	APIVersion        string     `json:"apiVersion,omitempty"`
	MinAPIVersion     string     `json:"minApiVersion,omitempty"`
	OperatingSystem   string     `json:"operatingSystem"`
	OSType            string     `json:"osType"`
	KernelVersion     string     `json:"kernelVersion"`
	Architecture      string     `json:"architecture"`
	NCPU              int        `json:"ncpu"`
	MemTotal          int64      `json:"memTotal"`
	StorageDriver     string     `json:"storageDriver"`
	LoggingDriver     string     `json:"loggingDriver"`
	CgroupDriver      string     `json:"cgroupDriver"`
	CgroupVersion     string     `json:"cgroupVersion,omitempty"`
	DockerRootDir     string     `json:"dockerRootDir"`
	DefaultRuntime    string     `json:"defaultRuntime"`
	Runtimes          []string   `json:"runtimes"`
	SecurityOptions   []string   `json:"securityOptions"`
	Containers        int        `json:"containers"`
	ContainersRunning int        `json:"containersRunning"`
	ContainersPaused  int        `json:"containersPaused"`
	ContainersStopped int        `json:"containersStopped"`
	Images            int        `json:"images"`
	ServerTime        string     `json:"serverTime,omitempty"`
	Warnings          []string   `json:"warnings"`
}

// DiskCategory is one Docker disk-usage category exactly as the Engine reports it.
// Size/Reclaimable are nil when the Engine did not provide a figure (negative = unavailable).
type DiskCategory struct {
	Key         string `json:"key"` // images|containers|volumes|buildCache
	Label       string `json:"label"`
	TotalCount  int64  `json:"totalCount"`
	ActiveCount int64  `json:"activeCount"`
	Size        *int64 `json:"size,omitempty"`
	Reclaimable *int64 `json:"reclaimable,omitempty"`
}

// DiskUsage is Docker's own accounting (`docker system df`), NOT host disk utilisation.
type DiskUsage struct {
	ConnectionID string         `json:"connectionId"`
	Categories   []DiskCategory `json:"categories"`
}

// EngineEvent is one Docker event mapped for display.
type EngineEvent struct {
	Time       int64             `json:"time"` // unix ms
	Type       string            `json:"type"`
	Action     string            `json:"action"`
	ActorID    string            `json:"actorId"`
	ActorName  string            `json:"actorName"`
	Attributes map[string]string `json:"attributes"`
}
