//go:build !windows

package wsldocker

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
