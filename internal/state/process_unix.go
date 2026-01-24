//go:build unix

package state

import (
	"syscall"
)

// isProcessRunning checks if a process with the given PID is running.
// Returns true if the process exists, false otherwise.
func isProcessRunning(pid int) bool {
	// Send signal 0 to check if process exists
	err := syscall.Kill(pid, 0)
	// If no error, process exists
	return err == nil
}
