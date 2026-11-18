package compose

import (
	"os/exec"
	"strconv"
	"syscall"
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// killTree makes context cancellation stop the whole process tree: the docker CLI
// spawns the compose plugin as a child that would otherwise keep running (and keep
// creating containers) after the CLI wrapper is killed.
func killTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		k := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		hideWindow(k)
		if err := k.Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
