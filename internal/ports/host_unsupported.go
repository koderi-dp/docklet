//go:build !windows && !linux && !darwin

package ports

// NewLocalHost: listeners cannot be read on this platform.
func NewLocalHost() HostSource { return unavailable{where: "This machine"} }
