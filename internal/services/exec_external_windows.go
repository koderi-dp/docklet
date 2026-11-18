//go:build windows

package services

import (
	"os/exec"
	"syscall"
)

// hideParentWindow stops the launcher (cmd.exe) from flashing its own console window.
func hideParentWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
