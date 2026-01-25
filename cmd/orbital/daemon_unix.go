//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr sets process attributes for Unix systems.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}
}
