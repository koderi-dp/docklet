//go:build !windows

package registryauth

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
