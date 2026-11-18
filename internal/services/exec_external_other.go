//go:build !windows

package services

import "os/exec"

func hideParentWindow(*exec.Cmd) {}
