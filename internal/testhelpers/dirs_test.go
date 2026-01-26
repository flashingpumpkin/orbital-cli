package testhelpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateDir_CreatesDirectoryStructure(t *testing.T) {
	tempDir, stateDir := StateDir(t)

	// Verify stateDir is under tempDir
	if !strings.HasPrefix(stateDir, tempDir) {
		t.Errorf("stateDir %q should be under tempDir %q", stateDir, tempDir)
	}

	// Verify stateDir has correct suffix
	wantSuffix := filepath.Join(".orbital", "state")
	if !strings.HasSuffix(stateDir, wantSuffix) {
		t.Errorf("stateDir %q should end with %q", stateDir, wantSuffix)
	}

	// Verify directory exists
	info, err := os.Stat(stateDir)
	if err != nil {
		t.Fatalf("stateDir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("stateDir should be a directory")
	}
}

func TestOrbitalDir_CreatesDirectoryStructure(t *testing.T) {
	tempDir, orbitalDir := OrbitalDir(t)

	// Verify orbitalDir is under tempDir
	if !strings.HasPrefix(orbitalDir, tempDir) {
		t.Errorf("orbitalDir %q should be under tempDir %q", orbitalDir, tempDir)
	}

	// Verify orbitalDir has correct suffix
	if !strings.HasSuffix(orbitalDir, ".orbital") {
		t.Errorf("orbitalDir %q should end with .orbital", orbitalDir)
	}

	// Verify directory exists
	info, err := os.Stat(orbitalDir)
	if err != nil {
		t.Fatalf("orbitalDir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("orbitalDir should be a directory")
	}
}

func TestWorkingDir_CreatesTempDirectory(t *testing.T) {
	dir := WorkingDir(t)

	// Verify directory exists
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("workingDir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("workingDir should be a directory")
	}
}
