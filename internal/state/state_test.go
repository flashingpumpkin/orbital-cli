package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/flashingpumpkin/orbit-cli/internal/workflow"
)

func TestStateDir_ReturnsCorrectPath(t *testing.T) {
	dir := StateDir("/some/project")

	want := "/some/project/.orbit/state"
	if dir != want {
		t.Errorf("StateDir() = %q; want %q", dir, want)
	}
}

func TestStateDir_HandlesTrailingSlash(t *testing.T) {
	dir := StateDir("/some/project/")

	want := "/some/project/.orbit/state"
	if dir != want {
		t.Errorf("StateDir() = %q; want %q", dir, want)
	}
}

func TestNewState_CreatesStateWithCorrectFields(t *testing.T) {
	files := []string{"/path/to/spec1.md", "/path/to/spec2.md"}
	state := NewState("session-123", "/working/dir", files, "", nil)

	if state.SessionID != "session-123" {
		t.Errorf("SessionID = %q; want %q", state.SessionID, "session-123")
	}
	if state.WorkingDir != "/working/dir" {
		t.Errorf("WorkingDir = %q; want %q", state.WorkingDir, "/working/dir")
	}
	if len(state.ActiveFiles) != 2 {
		t.Errorf("len(ActiveFiles) = %d; want 2", len(state.ActiveFiles))
	}
	if state.ActiveFiles[0] != "/path/to/spec1.md" {
		t.Errorf("ActiveFiles[0] = %q; want %q", state.ActiveFiles[0], "/path/to/spec1.md")
	}
	if state.PID != os.Getpid() {
		t.Errorf("PID = %d; want %d", state.PID, os.Getpid())
	}
	if state.Iteration != 0 {
		t.Errorf("Iteration = %d; want 0", state.Iteration)
	}
	if state.TotalCost != 0.0 {
		t.Errorf("TotalCost = %f; want 0.0", state.TotalCost)
	}
	if state.StartedAt.IsZero() {
		t.Error("StartedAt is zero; want current time")
	}
}

func TestState_SaveAndLoad_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	workingDir := tempDir

	// Create original state
	original := NewState("session-abc", workingDir, []string{"/path/spec.md"}, "", nil)
	original.Iteration = 5
	original.TotalCost = 1.23

	// Save state
	err := original.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	stateFile := filepath.Join(StateDir(workingDir), "state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Fatalf("state.json not created at %s", stateFile)
	}

	// Load state
	loaded, err := Load(workingDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Compare fields
	if loaded.SessionID != original.SessionID {
		t.Errorf("SessionID = %q; want %q", loaded.SessionID, original.SessionID)
	}
	if loaded.PID != original.PID {
		t.Errorf("PID = %d; want %d", loaded.PID, original.PID)
	}
	if loaded.WorkingDir != original.WorkingDir {
		t.Errorf("WorkingDir = %q; want %q", loaded.WorkingDir, original.WorkingDir)
	}
	if len(loaded.ActiveFiles) != len(original.ActiveFiles) {
		t.Errorf("len(ActiveFiles) = %d; want %d", len(loaded.ActiveFiles), len(original.ActiveFiles))
	}
	if loaded.Iteration != original.Iteration {
		t.Errorf("Iteration = %d; want %d", loaded.Iteration, original.Iteration)
	}
	if loaded.TotalCost != original.TotalCost {
		t.Errorf("TotalCost = %f; want %f", loaded.TotalCost, original.TotalCost)
	}
}

func TestLoad_ReturnsErrorWhenNoStateFile(t *testing.T) {
	tempDir := t.TempDir()

	_, err := Load(tempDir)

	if err == nil {
		t.Error("Load() returned nil error; want error for missing state file")
	}
}

func TestState_IsStale_ReturnsFalseForCurrentProcess(t *testing.T) {
	tempDir := t.TempDir()
	state := NewState("session-123", tempDir, []string{}, "", nil)

	if state.IsStale() {
		t.Error("IsStale() = true; want false for current process PID")
	}
}

func TestState_IsStale_ReturnsTrueForDeadProcess(t *testing.T) {
	tempDir := t.TempDir()
	state := NewState("session-123", tempDir, []string{}, "", nil)
	// Use a PID that definitely doesn't exist
	state.PID = 99999999

	if !state.IsStale() {
		t.Error("IsStale() = false; want true for dead PID")
	}
}

func TestState_Cleanup_RemovesStateDirectory(t *testing.T) {
	tempDir := t.TempDir()

	state := NewState("session-123", tempDir, []string{"/path/spec.md"}, "", nil)
	err := state.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify state directory exists
	stateDir := StateDir(tempDir)
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		t.Fatal("state directory not created")
	}

	// Cleanup
	err = state.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// Verify state directory is removed
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Error("state directory still exists after Cleanup()")
	}
}

func TestState_UpdateIteration_UpdatesFields(t *testing.T) {
	tempDir := t.TempDir()
	state := NewState("session-123", tempDir, []string{}, "", nil)

	state.UpdateIteration(5, 2.50)

	if state.Iteration != 5 {
		t.Errorf("Iteration = %d; want 5", state.Iteration)
	}
	if state.TotalCost != 2.50 {
		t.Errorf("TotalCost = %f; want 2.50", state.TotalCost)
	}
}

func TestState_StartedAt_IsPreservedOnLoad(t *testing.T) {
	tempDir := t.TempDir()

	original := NewState("session-123", tempDir, []string{}, "", nil)
	// Set a specific time in the past
	original.StartedAt = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	err := original.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !loaded.StartedAt.Equal(original.StartedAt) {
		t.Errorf("StartedAt = %v; want %v", loaded.StartedAt, original.StartedAt)
	}
}

func TestExists_ReturnsTrueWhenStateFileExists(t *testing.T) {
	tempDir := t.TempDir()

	state := NewState("session-123", tempDir, []string{}, "", nil)
	err := state.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if !Exists(tempDir) {
		t.Error("Exists() = false; want true when state file exists")
	}
}

func TestExists_ReturnsFalseWhenNoStateFile(t *testing.T) {
	tempDir := t.TempDir()

	if Exists(tempDir) {
		t.Error("Exists() = true; want false when no state file")
	}
}

func TestNewState_SetsNotesAndContextFiles(t *testing.T) {
	files := []string{"/path/to/spec.md"}
	notesFile := "docs/notes/2026-01-24-notes-feature.md"
	contextFiles := []string{"/path/to/context1.md", "/path/to/context2.md"}

	state := NewState("session-123", "/working/dir", files, notesFile, contextFiles)

	if state.NotesFile != notesFile {
		t.Errorf("NotesFile = %q; want %q", state.NotesFile, notesFile)
	}
	if len(state.ContextFiles) != 2 {
		t.Errorf("len(ContextFiles) = %d; want 2", len(state.ContextFiles))
	}
	if state.ContextFiles[0] != "/path/to/context1.md" {
		t.Errorf("ContextFiles[0] = %q; want %q", state.ContextFiles[0], "/path/to/context1.md")
	}
	if state.ContextFiles[1] != "/path/to/context2.md" {
		t.Errorf("ContextFiles[1] = %q; want %q", state.ContextFiles[1], "/path/to/context2.md")
	}
}

func TestState_SaveAndLoad_PreservesNotesAndContext(t *testing.T) {
	tempDir := t.TempDir()
	notesFile := "docs/notes/2026-01-24-notes-feature.md"
	contextFiles := []string{"/path/to/context1.md", "/path/to/context2.md"}

	// Create and save state with notes and context
	original := NewState("session-abc", tempDir, []string{"/path/spec.md"}, notesFile, contextFiles)
	err := original.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load state
	loaded, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify NotesFile is preserved
	if loaded.NotesFile != notesFile {
		t.Errorf("NotesFile = %q; want %q", loaded.NotesFile, notesFile)
	}

	// Verify ContextFiles are preserved
	if len(loaded.ContextFiles) != len(contextFiles) {
		t.Errorf("len(ContextFiles) = %d; want %d", len(loaded.ContextFiles), len(contextFiles))
	}
	for i, f := range contextFiles {
		if loaded.ContextFiles[i] != f {
			t.Errorf("ContextFiles[%d] = %q; want %q", i, loaded.ContextFiles[i], f)
		}
	}
}

func TestState_SaveAndLoad_HandlesEmptyNotesAndContext(t *testing.T) {
	tempDir := t.TempDir()

	// Create state with empty notes and context (backward compatibility)
	original := NewState("session-abc", tempDir, []string{"/path/spec.md"}, "", nil)
	err := original.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load state
	loaded, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify empty values
	if loaded.NotesFile != "" {
		t.Errorf("NotesFile = %q; want empty string", loaded.NotesFile)
	}
	if len(loaded.ContextFiles) != 0 {
		t.Errorf("len(ContextFiles) = %d; want 0", len(loaded.ContextFiles))
	}
}

func TestState_SetWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	state := NewState("session-123", tempDir, []string{}, "", nil)

	w := &workflow.Workflow{
		Preset: "tdd",
		Steps: []workflow.Step{
			{Name: "red", Prompt: "Write test"},
			{Name: "green", Prompt: "Make pass"},
			{Name: "refactor", Prompt: "Clean up"},
			{Name: "review", Prompt: "Review", Gate: true, OnFail: "refactor"},
		},
	}

	state.SetWorkflow(w)

	if state.Workflow == nil {
		t.Fatal("Workflow is nil after SetWorkflow()")
	}
	if state.Workflow.PresetName != "tdd" {
		t.Errorf("PresetName = %q; want \"tdd\"", state.Workflow.PresetName)
	}
	if len(state.Workflow.Steps) != 4 {
		t.Errorf("len(Steps) = %d; want 4", len(state.Workflow.Steps))
	}
	if state.Workflow.CurrentStepIndex != 0 {
		t.Errorf("CurrentStepIndex = %d; want 0", state.Workflow.CurrentStepIndex)
	}
}

func TestState_UpdateWorkflowStep(t *testing.T) {
	tempDir := t.TempDir()
	state := NewState("session-123", tempDir, []string{}, "", nil)

	w := &workflow.Workflow{
		Steps: []workflow.Step{
			{Name: "step1", Prompt: "First"},
			{Name: "step2", Prompt: "Second"},
		},
	}
	state.SetWorkflow(w)

	state.UpdateWorkflowStep(1)

	if state.Workflow.CurrentStepIndex != 1 {
		t.Errorf("CurrentStepIndex = %d; want 1", state.Workflow.CurrentStepIndex)
	}
}

func TestState_GateRetryTracking(t *testing.T) {
	tempDir := t.TempDir()
	state := NewState("session-123", tempDir, []string{}, "", nil)

	w := &workflow.Workflow{
		Steps: []workflow.Step{
			{Name: "review", Prompt: "Review", Gate: true},
		},
	}
	state.SetWorkflow(w)

	// Initially zero
	if state.GetGateRetryCount("review") != 0 {
		t.Errorf("initial retry count = %d; want 0", state.GetGateRetryCount("review"))
	}

	// Increment
	state.IncrementGateRetry("review")
	if state.GetGateRetryCount("review") != 1 {
		t.Errorf("after first increment = %d; want 1", state.GetGateRetryCount("review"))
	}

	// Increment again
	state.IncrementGateRetry("review")
	if state.GetGateRetryCount("review") != 2 {
		t.Errorf("after second increment = %d; want 2", state.GetGateRetryCount("review"))
	}
}

func TestState_SaveAndLoad_PreservesWorkflow(t *testing.T) {
	tempDir := t.TempDir()

	original := NewState("session-abc", tempDir, []string{"/path/spec.md"}, "", nil)
	w := &workflow.Workflow{
		Preset: "reviewed",
		Steps: []workflow.Step{
			{Name: "implement", Prompt: "Do it"},
			{Name: "review", Prompt: "Review", Gate: true, OnFail: "implement"},
		},
	}
	original.SetWorkflow(w)
	original.UpdateWorkflowStep(1)
	original.IncrementGateRetry("review")

	err := original.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Workflow == nil {
		t.Fatal("Workflow is nil after Load()")
	}
	if loaded.Workflow.PresetName != "reviewed" {
		t.Errorf("PresetName = %q; want \"reviewed\"", loaded.Workflow.PresetName)
	}
	if len(loaded.Workflow.Steps) != 2 {
		t.Errorf("len(Steps) = %d; want 2", len(loaded.Workflow.Steps))
	}
	if loaded.Workflow.CurrentStepIndex != 1 {
		t.Errorf("CurrentStepIndex = %d; want 1", loaded.Workflow.CurrentStepIndex)
	}
	if loaded.GetGateRetryCount("review") != 1 {
		t.Errorf("GateRetries[review] = %d; want 1", loaded.GetGateRetryCount("review"))
	}
}
