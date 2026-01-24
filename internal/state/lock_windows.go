//go:build windows

package state

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	LOCKFILE_EXCLUSIVE_LOCK = 0x00000002
)

// acquireLock acquires an exclusive lock on the given file using Windows LockFileEx.
func acquireLock(lockFile *os.File) error {
	var overlapped syscall.Overlapped
	
	// LockFileEx flags: LOCKFILE_EXCLUSIVE_LOCK
	flags := uint32(LOCKFILE_EXCLUSIVE_LOCK)
	
	// Lock the entire file (0xFFFFFFFF for high and low bytes)
	ret, _, err := procLockFileEx.Call(
		uintptr(lockFile.Fd()),
		uintptr(flags),
		uintptr(0), // reserved
		uintptr(0xFFFFFFFF), // nNumberOfBytesToLockLow
		uintptr(0xFFFFFFFF), // nNumberOfBytesToLockHigh
		uintptr(unsafe.Pointer(&overlapped)),
	)
	
	if ret == 0 {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	return nil
}

// releaseLock releases the lock on the given file using Windows UnlockFileEx.
func releaseLock(lockFile *os.File) error {
	var overlapped syscall.Overlapped
	
	// Unlock the entire file (0xFFFFFFFF for high and low bytes)
	ret, _, err := procUnlockFileEx.Call(
		uintptr(lockFile.Fd()),
		uintptr(0), // reserved
		uintptr(0xFFFFFFFF), // nNumberOfBytesToUnlockLow
		uintptr(0xFFFFFFFF), // nNumberOfBytesToUnlockHigh
		uintptr(unsafe.Pointer(&overlapped)),
	)
	
	if ret == 0 {
		return fmt.Errorf("failed to unlock file: %w", err)
	}
	return nil
}
