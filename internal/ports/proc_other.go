//go:build !windows

package ports

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
