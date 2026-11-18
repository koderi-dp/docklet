//go:build !windows

package compose

import (
	"os/exec"
	"syscall"
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree makes context cancellation stop the whole process group (docker CLI and the
// compose plugin it spawns), not just the CLI wrapper.
func killTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
