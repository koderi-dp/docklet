package model

// ExecOptions configures an interactive terminal session in a running container.
type ExecOptions struct {
	// Shell is empty (auto-detect: /bin/bash then /bin/sh) or one of
	// auto, bash, sh, ash, zsh, or an absolute shell path.
	Shell   string `json:"shell"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
	User    string `json:"user"`
	WorkDir string `json:"workDir"`
}

// ExecSession identifies a started terminal session.
type ExecSession struct {
	ID    string `json:"id"`
	Shell string `json:"shell"` // shell actually started
}

// ExecDataEvent is the payload of "exec:data".
type ExecDataEvent struct {
	SessionID  string `json:"sessionId"`
	DataBase64 string `json:"dataBase64"`
}

// ExecExitEvent is the payload of "exec:exit". ExitCode is nil when unknown.
type ExecExitEvent struct {
	SessionID string `json:"sessionId"`
	ExitCode  *int   `json:"exitCode,omitempty"`
	Error     string `json:"error,omitempty"`
}

// CreateEnvVar is one environment variable of a new container.
type CreateEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PortMapping publishes a container port. HostPort 0 lets the Engine choose.
type PortMapping struct {
	HostIP        string `json:"hostIp"`
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"` // tcp (default), udp, sctp
}

// MountSpec is a bind mount or a named volume.
type MountSpec struct {
	Type     string `json:"type"` // "bind" or "volume"
	Source   string `json:"source"`
	Target   string `json:"target"` // absolute path inside the container
	ReadOnly bool   `json:"readOnly"`
}

// ContainerCreateSpec is the New Container form.
type ContainerCreateSpec struct {
	Name          string         `json:"name"`
	Image         string         `json:"image"`
	Command       string         `json:"command"` // parsed into argv, never run through a shell
	Env           []CreateEnvVar `json:"env"`
	Ports         []PortMapping  `json:"ports"`
	Mounts        []MountSpec    `json:"mounts"`
	RestartPolicy string         `json:"restartPolicy"` // no, always, unless-stopped, on-failure
	MaxRetries    int            `json:"maxRetries"`    // on-failure only
	MemoryMiB     int64          `json:"memoryMiB"`     // 0 = unlimited
	CPUs          float64        `json:"cpus"`          // 0 = unlimited
	AutoStart     bool           `json:"autoStart"`
}
