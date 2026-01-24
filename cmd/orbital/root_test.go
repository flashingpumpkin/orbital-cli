package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flashingpumpkin/orbital/internal/state"
)

func TestGenerateSessionID_ReturnsNonEmptyString(t *testing.T) {
	id := generateSessionID()
	if id == "" {
		t.Error("generateSessionID() returned empty string")
	}
}

func TestGenerateSessionID_ReturnsUniqueIDs(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()
	if id1 == id2 {
		t.Errorf("generateSessionID() returned duplicate IDs: %s", id1)
	}
}

func TestInitState_CreatesStateFile(t *testing.T) {
	tempDir := t.TempDir()

	st, err := initState("test-session", tempDir, []string{"/path/spec.md"}, "", nil)
	if err != nil {
		t.Fatalf("initState() error = %v", err)
	}

	// Verify state was saved
	if !state.Exists(tempDir) {
		t.Error("state file was not created")
	}

	// Verify state fields
	if st.SessionID != "test-session" {
		t.Errorf("SessionID = %q; want %q", st.SessionID, "test-session")
	}
	if st.WorkingDir != tempDir {
		t.Errorf("WorkingDir = %q; want %q", st.WorkingDir, tempDir)
	}
	if len(st.ActiveFiles) != 1 || st.ActiveFiles[0] != "/path/spec.md" {
		t.Errorf("ActiveFiles = %v; want [/path/spec.md]", st.ActiveFiles)
	}
	if st.PID != os.Getpid() {
		t.Errorf("PID = %d; want %d", st.PID, os.Getpid())
	}
}

func TestInitState_CreatesStateDirectory(t *testing.T) {
	tempDir := t.TempDir()

	_, err := initState("test-session", tempDir, []string{"/path/spec.md"}, "", nil)
	if err != nil {
		t.Fatalf("initState() error = %v", err)
	}

	stateDir := state.StateDir(tempDir)
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		t.Error("state directory was not created")
	}
}

func TestCleanupState_RemovesStateDirectory(t *testing.T) {
	tempDir := t.TempDir()

	st, err := initState("test-session", tempDir, []string{"/path/spec.md"}, "", nil)
	if err != nil {
		t.Fatalf("initState() error = %v", err)
	}

	err = cleanupState(st)
	if err != nil {
		t.Fatalf("cleanupState() error = %v", err)
	}

	if state.Exists(tempDir) {
		t.Error("state file still exists after cleanup")
	}
}

func TestUpdateState_UpdatesIterationAndCost(t *testing.T) {
	tempDir := t.TempDir()

	st, err := initState("test-session", tempDir, []string{"/path/spec.md"}, "", nil)
	if err != nil {
		t.Fatalf("initState() error = %v", err)
	}

	err = updateState(st, 5, 1.23)
	if err != nil {
		t.Fatalf("updateState() error = %v", err)
	}

	// Reload and verify
	loaded, err := state.Load(tempDir)
	if err != nil {
		t.Fatalf("state.Load() error = %v", err)
	}

	if loaded.Iteration != 5 {
		t.Errorf("Iteration = %d; want 5", loaded.Iteration)
	}
	if loaded.TotalCost != 1.23 {
		t.Errorf("TotalCost = %f; want 1.23", loaded.TotalCost)
	}
}

func TestGetAbsolutePaths_ConvertsRelativePaths(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create a file
	specFile := filepath.Join(tempDir, "spec.md")
	if err := os.WriteFile(specFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	paths, err := getAbsolutePaths([]string{"spec.md"})
	if err != nil {
		t.Fatalf("getAbsolutePaths() error = %v", err)
	}

	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d; want 1", len(paths))
	}

	if !filepath.IsAbs(paths[0]) {
		t.Errorf("path %q is not absolute", paths[0])
	}
}
