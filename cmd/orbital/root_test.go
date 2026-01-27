package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flashingpumpkin/orbital/internal/completion"
	"github.com/flashingpumpkin/orbital/internal/state"
	"github.com/flashingpumpkin/orbital/internal/workflow"
)

func TestGenerateSessionID_ReturnsNonEmptyString(t *testing.T) {
	id, err := generateSessionID()
	if err != nil {
		t.Fatalf("generateSessionID() error = %v", err)
	}
	if id == "" {
		t.Error("generateSessionID() returned empty string")
	}
}

func TestGenerateSessionID_ReturnsUniqueIDs(t *testing.T) {
	id1, err := generateSessionID()
	if err != nil {
		t.Fatalf("generateSessionID() error = %v", err)
	}
	id2, err := generateSessionID()
	if err != nil {
		t.Fatalf("generateSessionID() error = %v", err)
	}
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

func TestEnsureNotesFile_CreatesNewFile(t *testing.T) {
	tempDir := t.TempDir()
	notesPath := filepath.Join(tempDir, "notes.md")
	specPath := "/path/to/spec.md"

	err := ensureNotesFile(notesPath, specPath)
	if err != nil {
		t.Fatalf("ensureNotesFile() error = %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(notesPath)
	if err != nil {
		t.Fatalf("failed to read notes file: %v", err)
	}

	// Check header content
	if !contains(string(content), "# Notes") {
		t.Error("notes file missing header")
	}
	if !contains(string(content), "Spec: spec.md") {
		t.Error("notes file missing spec reference")
	}
	if !contains(string(content), "Date:") {
		t.Error("notes file missing date")
	}
}

func TestEnsureNotesFile_PreservesExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	notesPath := filepath.Join(tempDir, "notes.md")
	specPath := "/path/to/spec.md"

	// Create existing file with custom content
	existingContent := "# My Custom Notes\n\nImportant stuff here."
	if err := os.WriteFile(notesPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to write existing notes file: %v", err)
	}

	err := ensureNotesFile(notesPath, specPath)
	if err != nil {
		t.Fatalf("ensureNotesFile() error = %v", err)
	}

	// Verify existing content was preserved
	content, err := os.ReadFile(notesPath)
	if err != nil {
		t.Fatalf("failed to read notes file: %v", err)
	}

	if string(content) != existingContent {
		t.Errorf("existing content was not preserved, got %q, want %q", string(content), existingContent)
	}
}

func TestEnsureNotesFile_HandlesNestedPath(t *testing.T) {
	tempDir := t.TempDir()
	// Parent directory already exists but notes file doesn't
	notesDir := filepath.Join(tempDir, "docs", "notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		t.Fatalf("failed to create notes directory: %v", err)
	}
	notesPath := filepath.Join(notesDir, "session-notes.md")
	specPath := "my-spec.md"

	err := ensureNotesFile(notesPath, specPath)
	if err != nil {
		t.Fatalf("ensureNotesFile() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		t.Error("notes file was not created in nested directory")
	}
}

// contains is a helper to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPromiseDetectionInWorkflowSteps(t *testing.T) {
	promise := "<promise>COMPLETE</promise>"
	detector := completion.New(promise)

	tests := []struct {
		name           string
		steps          []*workflow.StepResult
		expectDetected bool
	}{
		{
			name: "detects promise in first step",
			steps: []*workflow.StepResult{
				{StepName: "implement", Output: "Done! <promise>COMPLETE</promise>"},
				{StepName: "review", Output: "<gate>PASS</gate>"},
			},
			expectDetected: true,
		},
		{
			name: "detects promise in second step",
			steps: []*workflow.StepResult{
				{StepName: "implement", Output: "Working on it..."},
				{StepName: "review", Output: "All good! <promise>COMPLETE</promise>"},
			},
			expectDetected: true,
		},
		{
			name: "no promise in any step",
			steps: []*workflow.StepResult{
				{StepName: "implement", Output: "Made progress"},
				{StepName: "review", Output: "<gate>FAIL</gate> Issues found"},
			},
			expectDetected: false,
		},
		{
			name:           "empty steps list",
			steps:          []*workflow.StepResult{},
			expectDetected: false,
		},
		{
			name: "promise mixed with other content",
			steps: []*workflow.StepResult{
				{StepName: "implement", Output: "Starting work...\nDoing things...\n<promise>COMPLETE</promise>\nCleaning up..."},
			},
			expectDetected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detected := false
			for _, stepResult := range tt.steps {
				if detector.Check(stepResult.Output) {
					detected = true
					break
				}
			}

			if detected != tt.expectDetected {
				t.Errorf("promise detected = %v; want %v", detected, tt.expectDetected)
			}
		})
	}
}
