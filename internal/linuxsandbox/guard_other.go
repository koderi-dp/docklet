//go:build !linux

package linuxsandbox

// Guard does nothing outside Linux.
func Guard() {}
