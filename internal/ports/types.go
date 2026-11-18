// Package ports answers "what already holds this host port?" from two sources: the
// published ports of the containers on an Engine, and (when the Engine runs on this
// computer) the listening sockets of the machine. It has no UI or Wails dependencies.
package ports

// Status of one requested port. "unknown" is never reported as free: it means a
// source that could hold the port could not be read.
type Status string

const (
	StatusFree    Status = "free"
	StatusInUse   Status = "in-use"
	StatusUnknown Status = "unknown"
)

// HolderKind says what holds a port.
type HolderKind string

const (
	HolderContainer HolderKind = "container"
	HolderProcess   HolderKind = "process"
	HolderSystem    HolderKind = "system" // the operating system itself (for example HTTP.sys)
)

// Request is one host port to check. HostIP empty means every address.
type Request struct {
	HostIP   string `json:"hostIp"`
	HostPort int    `json:"hostPort"`
	Protocol string `json:"protocol"` // tcp (default), udp or sctp
}

// Holder is whatever holds a port.
type Holder struct {
	Kind           HolderKind `json:"kind"`
	Location       string     `json:"location"` // "This machine", "Windows", "WSL (Ubuntu)", "Docker Engine"
	BindIP         string     `json:"bindIp"`
	ContainerID    string     `json:"containerId"`
	ContainerName  string     `json:"containerName"`
	ComposeProject string     `json:"composeProject"`
	Running        bool       `json:"running"` // containers only: a stopped container holds nothing
	PID            int        `json:"pid"`
	ProcessName    string     `json:"processName"`
	ExecutablePath string     `json:"executablePath"`
	User           string     `json:"user"`
}

// PortCheck is the answer for one Request.
type PortCheck struct {
	HostIP   string   `json:"hostIp"`
	HostPort int      `json:"hostPort"`
	Protocol string   `json:"protocol"`
	Status   Status   `json:"status"`
	Holder   *Holder  `json:"holder"`
	Also     []Holder `json:"also"`   // further holders besides the main one
	Note     string   `json:"note"`   // for example a stopped container with the same binding
	Detail   string   `json:"detail"` // one sentence: who holds it, or why it could not be checked
}

// Entry is one row of the full listing ("Ports" view).
type Entry struct {
	HostIP   string `json:"hostIp"`
	HostPort int    `json:"hostPort"`
	Protocol string `json:"protocol"`
	Holder   Holder `json:"holder"`
}

// Listing is every published port and, when readable, every host listener.
type Listing struct {
	Entries []Entry  `json:"entries"`
	Notes   []string `json:"notes"` // what could not be seen, and why
}

// Binding is one published port of a container, as the Engine reports it.
type Binding struct {
	ContainerID string
	Name        string
	Project     string
	Running     bool
	HostIP      string
	HostPort    int
	Protocol    string
	// Engine names the engine that publishes the port ("Docker Engine" when empty).
	Engine string
}

// Listener is one listening socket on a machine.
type Listener struct {
	IP       string
	Port     int
	Protocol string
	PID      int
	Name     string
	Path     string
	User     string
	Location string
	System   bool // owned by the operating system, not a program
	Hidden   bool // the details of the owning process could not be read
}

func normProto(p string) string {
	switch p {
	case "", "TCP", "Tcp":
		return "tcp"
	case "UDP", "Udp":
		return "udp"
	}
	return p
}
