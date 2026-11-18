package runspec

import "strings"

// PodmanInjectedEnv reports whether an environment entry ("KEY=value") is one Podman adds to every
// container itself: container=podman, HOSTNAME set to the short container ID, and HOME for root. Podman
// writes them into the container's configured environment (Docker does not), so copying them into a
// new container or a docker run command would pin values that belong to the old container.
func PodmanInjectedEnv(entry, containerID string) bool {
	k, v, _ := strings.Cut(entry, "=")
	switch k {
	case "container":
		return v == "podman"
	case "HOSTNAME":
		return v == shortID(containerID)
	case "HOME":
		return v == "" || v == "/root"
	}
	return false
}
