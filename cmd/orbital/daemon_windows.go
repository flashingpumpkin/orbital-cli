//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr sets process attributes for Windows systems.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
