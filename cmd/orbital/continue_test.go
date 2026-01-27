package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/flashingpumpkin/orbital/internal/session"
	"github.com/flashingpumpkin/orbital/internal/state"
)

func TestRunContinue_NoState(t *testing.T) {
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

	cmd := newContinueCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()

	if err == nil {
		t.Fatal("expected error when no state exists")
	}
	expected := "no session to continue in this directory"
	if err.Error() != expected {
		t.Errorf("expected '%s' error, got: %v", expected, err)
	}
}

func TestRunContinue_InstanceAlreadyRunning(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
		nonInteractive = false // Reset flag
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create state with current PID (simulates running instance)
	st := state.NewState("test-session", tempDir, []string{"/path/spec.md"}, "", nil)
	st.PID = os.Getpid() // Current process - will be considered "running"
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Use non-interactive mode to avoid TUI
	nonInteractive = true

	cmd := newContinueCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()

	if err == nil {
		t.Fatal("expected error when instance already running")
	}
	// With the new unified session abstraction, a running session is marked as invalid
	// and in non-interactive mode we get "no valid sessions to resume"
	errStr := err.Error()
	if !strings.Contains(errStr, "no valid sessions") {
		t.Errorf("expected error to contain 'no valid sessions', got: %s", errStr)
	}
}

func TestRunContinue_NoActiveFilesOrQueue(t *testing.T) {
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

	// Create state with no active files and a fake (dead) PID
	st := state.NewState("test-session", tempDir, []string{}, "", nil)
	st.PID = 999999 // Fake PID that's very unlikely to exist
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	cmd := newContinueCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()

	if err == nil {
		t.Fatal("expected error when no active files or queue")
	}
	expected := "no session to continue in this directory (no active or queued files)"
	if err.Error() != expected {
		t.Errorf("expected '%s' error, got: %v", expected, err)
	}
}

func TestContinueCmd_HelpWorks(t *testing.T) {
	cmd := newContinueCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	if err != nil {
		t.Fatalf("help should not error, got: %v", err)
	}

	output := out.String()
	if output == "" {
		t.Error("help output should not be empty")
	}
	// Check that help contains expected text
	if !bytes.Contains([]byte(output), []byte("Resume")) {
		t.Errorf("help should contain 'Resume', got: %s", output)
	}
}

func TestContinueCmd_AcceptsConfigFlags(t *testing.T) {
	// Test that continue command accepts the configuration flags via persistent flags
	// We need to test via rootCmd to verify flags are inherited properly
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	// Reset args and run help for continue subcommand
	rootCmd.SetArgs([]string{"continue", "--help"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("help should not error, got: %v", err)
	}

	output := out.String()

	// Check that important configuration flags are shown in Global Flags section
	requiredFlags := []string{"--model", "--budget", "--iterations", "--timeout"}
	for _, flag := range requiredFlags {
		if !bytes.Contains([]byte(output), []byte(flag)) {
			t.Errorf("help should contain '%s' flag in Global Flags, got: %s", flag, output)
		}
	}
}

func TestRunContinue_UsesActualWorkingDir(t *testing.T) {
	// Test that continue uses the actual working directory (from os.Getwd())
	// not the flag default "." for loading config files
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

	// Create a config file in the temp directory with a custom prompt
	configDir := tempDir + "/.orbital"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configContent := `prompt = "test prompt from config"`
	if err := os.WriteFile(configDir+"/config.toml", []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create a valid spec file
	specFile := tempDir + "/spec.md"
	if err := os.WriteFile(specFile, []byte("# Test spec"), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create state with a stale PID pointing to our spec file
	st := state.NewState("test-session", tempDir, []string{specFile}, "", nil)
	st.PID = 999999 // Dead PID so it's stale and can be resumed
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// The state's WorkingDir should already be the absolute path since we created it that way.
	loadedState, err := state.Load(tempDir)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loadedState.WorkingDir != tempDir {
		t.Errorf("state.WorkingDir = %q; want %q", loadedState.WorkingDir, tempDir)
	}
}

func TestSelectSession_SingleValidSession(t *testing.T) {
	// Create a single valid session
	sessions := []session.Session{
		{
			ID:    "sess-1",
			Type:  session.SessionTypeRegular,
			Name:  "Main session",
			Valid: true,
		},
	}

	nonInteractive = false
	collector := &mockCollector{validSessions: sessions}

	selected, cleanupPaths, err := selectSession(sessions, collector, "auto")
	if err != nil {
		t.Fatalf("selectSession() error = %v", err)
	}
	if selected.Name != "Main session" {
		t.Errorf("selected.Name = %q; want %q", selected.Name, "Main session")
	}
	if len(cleanupPaths) != 0 {
		t.Errorf("cleanupPaths = %v; want empty", cleanupPaths)
	}
}

func TestSelectSession_NonInteractiveMultiple(t *testing.T) {
	sessions := []session.Session{
		{
			ID:    "sess-1",
			Type:  session.SessionTypeRegular,
			Name:  "Session 1",
			Valid: true,
		},
		{
			ID:    "sess-2",
			Type:  session.SessionTypeRegular,
			Name:  "Session 2",
			Valid: true,
		},
	}

	nonInteractive = true
	defer func() { nonInteractive = false }()
	collector := &mockCollector{validSessions: sessions}

	_, _, err := selectSession(sessions, collector, "auto")
	if err == nil {
		t.Fatal("selectSession() should return error in non-interactive mode with multiple sessions")
	}
	if !strings.Contains(err.Error(), "multiple sessions found") {
		t.Errorf("error should contain 'multiple sessions found', got: %v", err)
	}
}

func TestSelectSession_NoValidSessions_NonInteractive(t *testing.T) {
	sessions := []session.Session{
		{
			ID:            "sess-1",
			Type:          session.SessionTypeRegular,
			Name:          "Bad session",
			Valid:         false,
			InvalidReason: "session not found",
		},
	}

	nonInteractive = true
	defer func() { nonInteractive = false }()
	collector := &mockCollector{validSessions: nil}

	_, _, err := selectSession(sessions, collector, "auto")
	if err == nil {
		t.Fatal("selectSession() should return error when no valid sessions")
	}
	if !strings.Contains(err.Error(), "no valid sessions") {
		t.Errorf("error should contain 'no valid sessions', got: %v", err)
	}
}

func TestFormatSessionList(t *testing.T) {
	sessions := []session.Session{
		{
			Type:  session.SessionTypeRegular,
			Name:  "Session 1",
			Valid: true,
		},
		{
			Type:          session.SessionTypeRegular,
			Name:          "Session 2",
			Valid:         false,
			InvalidReason: "Session is currently running",
		},
	}

	result := formatSessionList(sessions)

	// Check it contains expected info
	if !strings.Contains(result, "[1] Session 1") {
		t.Errorf("should contain numbered list with first session, got: %s", result)
	}
	if !strings.Contains(result, "[2] Session 2") {
		t.Errorf("should contain numbered list with second session, got: %s", result)
	}
	if !strings.Contains(result, "invalid") {
		t.Errorf("should contain invalid status, got: %s", result)
	}
}

// mockCollector implements the ValidSessions method for testing.
type mockCollector struct {
	validSessions []session.Session
}

func (m *mockCollector) ValidSessions(sessions []session.Session) []session.Session {
	return m.validSessions
}
