//go:build unix

package state

import (
	"fmt"
	"os"
	"syscall"
)

// acquireLock acquires an exclusive lock on the given file.
func acquireLock(lockFile *os.File) error {
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	return nil
}

// releaseLock releases the lock on the given file.
func releaseLock(lockFile *os.File) error {
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("failed to unlock file: %w", err)
	}
	return nil
}
