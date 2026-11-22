package model

// ComposeGraphPort is one published or exposed port of a service.
type ComposeGraphPort struct {
	HostIP    string `json:"hostIp,omitempty"`
	Published string `json:"published"` // "8080"; "" when only the container port is set
	Target    int    `json:"target"`
	Protocol  string `json:"protocol,omitempty"` // "tcp" | "udp"
}

// ComposeGraphDep is one depends_on edge.
type ComposeGraphDep struct {
	Service   string `json:"service"`
	Condition string `json:"condition,omitempty"` // service_started | service_healthy | service_completed_successfully
}

// ComposeGraphService is one service of the graph. It never carries environment
// values, commands or other secrets of the Compose config.
type ComposeGraphService struct {
	Name      string             `json:"name"`
	Image     string             `json:"image,omitempty"`
	Build     bool               `json:"build"` // has a build section
	Ports     []ComposeGraphPort `json:"ports"`
	Volumes   []string           `json:"volumes"` // named volumes and bind sources, as written
	DependsOn []ComposeGraphDep  `json:"dependsOn"`
	Profiles  []string           `json:"profiles"`
}

// ComposeGraph is the service structure of one Compose project.
type ComposeGraph struct {
	Services []ComposeGraphService `json:"services"` // sorted by name
	Networks []string              `json:"networks"` // resolved names, sorted
	Volumes  []string              `json:"volumes"`  // top-level volume keys, sorted
	// Source is "config" (from `docker compose config`) or "containers" (fallback from labels).
	Source string `json:"source"`
	// Note says why the fallback was used; empty for Source "config".
	Note string `json:"note,omitempty"`
}
