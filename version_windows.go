package main

import (
	"os"
	"syscall"
)

// versionOutput returns stdout. Release builds are GUI-subsystem programs without a console, so when
// stdout is not redirected the parent's console is attached and written to directly.
func versionOutput() *os.File {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	getFileType := kernel.NewProc("GetFileType")
	if t, _, _ := getFileType.Call(uintptr(syscall.Handle(os.Stdout.Fd()))); t != 0 {
		return os.Stdout // redirected to a file or pipe, or already a console
	}
	const attachParentProcess = ^uintptr(0)
	if ok, _, _ := kernel.NewProc("AttachConsole").Call(attachParentProcess); ok == 0 {
		return os.Stdout
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		return f
	}
	return os.Stdout
}
