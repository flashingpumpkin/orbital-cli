//go:build windows

package state

import (
	"syscall"
)

// isProcessRunning checks if a process with the given PID is running on Windows.
// Returns true if the process exists, false otherwise.
func isProcessRunning(pid int) bool {
	// On Windows, we open the process handle to check if it exists
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		// Process doesn't exist or we don't have permission
		return false
	}
	syscall.CloseHandle(handle)
	return true
}
