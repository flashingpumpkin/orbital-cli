// Package testhelpers provides common utilities for tests across packages.
package testhelpers

import (
	"os"
	"path/filepath"
	"testing"
)

// StateDir creates a temporary directory with the .orbital/state structure.
// Returns the temp dir root and the state dir path.
// The temp dir is automatically cleaned up when the test completes.
func StateDir(t *testing.T) (tempDir, stateDir string) {
	t.Helper()
	tempDir = t.TempDir()
	stateDir = filepath.Join(tempDir, ".orbital", "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}
	return tempDir, stateDir
}

// OrbitalDir creates a temporary directory with the .orbital structure.
// Returns the temp dir root and the orbital dir path.
// The temp dir is automatically cleaned up when the test completes.
func OrbitalDir(t *testing.T) (tempDir, orbitalDir string) {
	t.Helper()
	tempDir = t.TempDir()
	orbitalDir = filepath.Join(tempDir, ".orbital")
	if err := os.MkdirAll(orbitalDir, 0755); err != nil {
		t.Fatalf("failed to create orbital dir: %v", err)
	}
	return tempDir, orbitalDir
}

// WorkingDir creates a temporary directory suitable for use as a working directory.
// Returns the temp dir path.
// The temp dir is automatically cleaned up when the test completes.
func WorkingDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
