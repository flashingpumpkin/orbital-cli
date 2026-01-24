package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/flashingpumpkin/orbital/internal/state"
)

// createTestStatusCmd creates a fresh status command for testing.
func createTestStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "status",
		RunE: runStatus,
	}
	cmd.Flags().BoolVar(&statusJSON, "json", false, "Output in JSON format")
	return cmd
}

func TestStatusCmd_ShowsNoSessionWhenNoState(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Save and restore global workingDir
	originalWorkingDir := workingDir
	defer func() {
		workingDir = originalWorkingDir
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	workingDir = tempDir

	cmd := createTestStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	expected := "No orbital session in this directory"
	if !strings.Contains(output, expected) {
		t.Errorf("output = %q; want to contain %q", output, expected)
	}
}

func TestStatusCmd_ShowsStoppedStateMessage(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	originalWorkingDir := workingDir
	defer func() {
		workingDir = originalWorkingDir
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	workingDir = tempDir

	// Create a stale state with a dead PID
	st := state.NewState("session-123", tempDir, []string{"/path/spec.md"}, "", nil)
	st.PID = 99999999 // Non-existent PID
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	cmd := createTestStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	expected := "Status:     STOPPED"
	if !strings.Contains(output, expected) {
		t.Errorf("output = %q; want to contain %q", output, expected)
	}
}

func TestStatusCmd_ShowsRunningInstanceStatus(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	originalWorkingDir := workingDir
	defer func() {
		workingDir = originalWorkingDir
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	workingDir = tempDir

	// Create a running instance state (with current PID so it's not stale)
	st := state.NewState("session-abc123", tempDir, []string{"/path/spec1.md", "/path/spec2.md"}, "", nil)
	st.Iteration = 5
	st.TotalCost = 1.23
	st.StartedAt = time.Date(2026, 1, 18, 10, 0, 0, 0, time.UTC)
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	cmd := createTestStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()

	// Check for key elements
	checks := []string{
		"Orbital Status",
		"session-abc123",
		"5", // Iteration
		"$1.23", // Cost
		"/path/spec1.md",
		"/path/spec2.md",
		"Active Files:",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q\nfull output: %s", check, output)
		}
	}
}

func TestStatusCmd_ShowsQueuedFiles(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	originalWorkingDir := workingDir
	defer func() {
		workingDir = originalWorkingDir
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	workingDir = tempDir

	// Create a running instance state
	st := state.NewState("session-123", tempDir, []string{"/path/spec.md"}, "", nil)
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Add files to queue
	stateDir := state.StateDir(tempDir)
	queue, err := state.LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("failed to load queue: %v", err)
	}
	if err := queue.Add("/queued/spec1.md"); err != nil {
		t.Fatalf("failed to add file to queue: %v", err)
	}
	if err := queue.Add("/queued/spec2.md"); err != nil {
		t.Fatalf("failed to add file to queue: %v", err)
	}

	cmd := createTestStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()

	checks := []string{
		"Queued Files:",
		"/queued/spec1.md",
		"/queued/spec2.md",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q\nfull output: %s", check, output)
		}
	}
}

func TestStatusCmd_ShowsNoQueuedFilesMessage(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	originalWorkingDir := workingDir
	defer func() {
		workingDir = originalWorkingDir
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	workingDir = tempDir

	// Create a running instance state with no queue
	st := state.NewState("session-123", tempDir, []string{"/path/spec.md"}, "", nil)
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	cmd := createTestStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	expected := "Queued Files: (none)"
	if !strings.Contains(output, expected) {
		t.Errorf("output = %q; want to contain %q", output, expected)
	}
}
