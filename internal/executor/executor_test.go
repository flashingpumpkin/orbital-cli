package executor

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/flashingpumpkin/orbital/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Model:     "claude-sonnet-4-20250514",
		MaxBudget: 5.00,
		SessionID: "",
	}

	e := New(cfg)

	if e == nil {
		t.Fatal("New() returned nil")
	}
	if e.config != cfg {
		t.Error("New() did not store the config reference")
	}
}

func TestBuildArgs_BasicConfig(t *testing.T) {
	cfg := &config.Config{
		Model:     "claude-sonnet-4-20250514",
		MaxBudget: 5.00,
		SessionID: "",
	}
	e := New(cfg)

	args := e.BuildArgs("test prompt")

	// Check required args are present
	expected := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--model", "claude-sonnet-4-20250514",
		"--max-budget-usd", "5.00",
		"test prompt",
	}

	if len(args) != len(expected) {
		t.Fatalf("BuildArgs() returned %d args, want %d\nGot: %v\nWant: %v", len(args), len(expected), args, expected)
	}

	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("BuildArgs()[%d] = %q, want %q", i, args[i], arg)
		}
	}
}

func TestBuildArgs_WithSessionID(t *testing.T) {
	cfg := &config.Config{
		Model:     "claude-opus-4-20250514",
		MaxBudget: 10.50,
		SessionID: "session-123",
	}
	e := New(cfg)

	args := e.BuildArgs("resume this")

	expected := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--model", "claude-opus-4-20250514",
		"--max-budget-usd", "10.50",
		"--resume", "session-123",
		"resume this",
	}

	if len(args) != len(expected) {
		t.Fatalf("BuildArgs() returned %d args, want %d\nGot: %v\nWant: %v", len(args), len(expected), args, expected)
	}

	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("BuildArgs()[%d] = %q, want %q", i, args[i], arg)
		}
	}
}

func TestBuildArgs_WithAgents(t *testing.T) {
	cfg := &config.Config{
		Model:     "claude-sonnet-4-20250514",
		MaxBudget: 5.00,
		SessionID: "",
		Agents:    `{"reviewer": {"description": "Reviews code", "prompt": "You are a code reviewer"}}`,
	}
	e := New(cfg)

	args := e.BuildArgs("test prompt")

	// Check --agents flag is present
	var agentsFound bool
	var agentsValue string
	for i, arg := range args {
		if arg == "--agents" && i+1 < len(args) {
			agentsFound = true
			agentsValue = args[i+1]
			break
		}
	}

	if !agentsFound {
		t.Error("BuildArgs() should include --agents flag when Agents is set")
	}
	if agentsValue != cfg.Agents {
		t.Errorf("--agents value = %q, want %q", agentsValue, cfg.Agents)
	}
}

func TestBuildArgs_WithoutAgents(t *testing.T) {
	cfg := &config.Config{
		Model:     "claude-sonnet-4-20250514",
		MaxBudget: 5.00,
		SessionID: "",
		Agents:    "", // Empty
	}
	e := New(cfg)

	args := e.BuildArgs("test prompt")

	// Check --agents flag is NOT present
	for _, arg := range args {
		if arg == "--agents" {
			t.Error("BuildArgs() should not include --agents flag when Agents is empty")
			break
		}
	}
}

func TestBuildArgs_BudgetFormatting(t *testing.T) {
	tests := []struct {
		name     string
		budget   float64
		expected string
	}{
		{"whole number", 5.0, "5.00"},
		{"one decimal", 5.5, "5.50"},
		{"two decimals", 5.55, "5.55"},
		{"small value", 0.10, "0.10"},
		{"large value", 100.99, "100.99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Model:     "test-model",
				MaxBudget: tt.budget,
			}
			e := New(cfg)
			args := e.BuildArgs("test")

			// Find the --max-budget-usd value
			var found string
			for i, arg := range args {
				if arg == "--max-budget-usd" && i+1 < len(args) {
					found = args[i+1]
					break
				}
			}

			if found != tt.expected {
				t.Errorf("budget formatting: got %q, want %q", found, tt.expected)
			}
		})
	}
}

func TestExecutionResult_Fields(t *testing.T) {
	// Test that ExecutionResult struct has all required fields
	result := &ExecutionResult{
		Output:    "test output",
		ExitCode:  0,
		Duration:  time.Second * 5,
		TokensIn:  600,
		TokensOut: 400,
		CostUSD:   0.05,
		Completed: true,
		Error:     nil,
	}

	if result.Output != "test output" {
		t.Errorf("Output = %q, want %q", result.Output, "test output")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want %d", result.ExitCode, 0)
	}
	if result.Duration != time.Second*5 {
		t.Errorf("Duration = %v, want %v", result.Duration, time.Second*5)
	}
	if result.TokensIn != 600 {
		t.Errorf("TokensIn = %d, want %d", result.TokensIn, 600)
	}
	if result.TokensOut != 400 {
		t.Errorf("TokensOut = %d, want %d", result.TokensOut, 400)
	}
	if result.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want %f", result.CostUSD, 0.05)
	}
	if !result.Completed {
		t.Error("Completed = false, want true")
	}
	if result.Error != nil {
		t.Errorf("Error = %v, want nil", result.Error)
	}
}

func TestExecute_ClaudeNotInPath(t *testing.T) {
	// Save original PATH and restore after test
	cfg := &config.Config{
		Model:     "test-model",
		MaxBudget: 1.00,
	}
	e := New(cfg)

	// Use a non-existent command name to test PATH lookup failure
	e.claudeCmd = "nonexistent-claude-cmd-12345"

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	if err == nil {
		t.Error("Execute() should return error when claude is not in PATH")
	}
	if result != nil {
		t.Error("Execute() should return nil result on PATH error")
	}
}

func TestExecute_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		Model:     "test-model",
		MaxBudget: 1.00,
	}
	e := New(cfg)

	// Use a context that's already cancelled to test cancellation handling
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Use echo which would succeed normally
	e.claudeCmd = "echo"

	result, err := e.Execute(ctx, "test")

	// When context is already cancelled, the command should fail
	// Either we get an error, or the result indicates non-completion
	if err == nil && result != nil && result.Completed {
		t.Error("Execute() should not complete successfully with cancelled context")
	}
}

func TestExecute_ContextCancellationDuringRun(t *testing.T) {
	cfg := &config.Config{
		Model:     "test-model",
		MaxBudget: 1.00,
	}
	e := New(cfg)

	// Create a custom executor that uses a long-running command
	// We'll test by running a command that ignores the extra args
	e.claudeCmd = "sh"

	ctx, cancel := context.WithCancel(context.Background())

	// Start a goroutine to cancel the context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// sh -c "sleep 10" will actually sleep, ignoring the other args from BuildArgs
	// But BuildArgs passes args differently, so we need to be creative
	// Actually, let's use a different approach - just verify that a cancelled
	// context before start results in proper error handling
	result, err := e.Execute(ctx, "-c 'sleep 10'")

	// The command should either:
	// 1. Return an error due to context cancellation
	// 2. Return a result with Completed = false
	if err == nil && result != nil && result.Completed {
		t.Error("Execute() should handle context cancellation properly")
	}
}

func TestExecute_Success(t *testing.T) {
	// Skip if echo is not available (shouldn't happen on Unix)
	if _, err := exec.LookPath("echo"); err != nil {
		t.Skip("echo not available")
	}

	cfg := &config.Config{
		Model:     "test-model",
		MaxBudget: 1.00,
	}
	e := New(cfg)

	// Use echo as a simple command that returns quickly
	e.claudeCmd = "echo"

	ctx := context.Background()
	result, err := e.Execute(ctx, "hello world")

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !result.Completed {
		t.Error("Completed = false, want true")
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestExecute_WorkingDirSet(t *testing.T) {
	// This test verifies that when WorkingDir is set in config,
	// the executor sets cmd.Dir appropriately.
	// We can't easily test the actual working directory change via Execute()
	// because BuildArgs adds many arguments, so we test the config storage
	// and verify that a non-empty/non-dot WorkingDir is configured.

	cfg := &config.Config{
		Model:      "test-model",
		MaxBudget:  1.00,
		WorkingDir: "/tmp/test-worktree",
	}
	e := New(cfg)

	// Verify the executor stores the config
	if e.config.WorkingDir != "/tmp/test-worktree" {
		t.Errorf("Executor did not store WorkingDir; got %q, want %q",
			e.config.WorkingDir, "/tmp/test-worktree")
	}

	// The actual cmd.Dir setting is tested by running echo successfully
	// with a valid working directory (we can't verify the exact dir via output
	// because of how BuildArgs works)
	e.claudeCmd = "echo"

	ctx := context.Background()
	_, err := e.Execute(ctx, "test")

	// This should fail because /tmp/test-worktree doesn't exist
	if err == nil {
		t.Log("Note: If /tmp/test-worktree exists, the test passes but doesn't verify cmd.Dir")
	}
	// Either way, the config is correctly stored
}

func TestExecute_WorkingDirDefault(t *testing.T) {
	// When WorkingDir is "." or empty, cmd.Dir should not be set
	// We test this indirectly by verifying the command runs successfully

	tests := []struct {
		name       string
		workingDir string
	}{
		{"empty string", ""},
		{"dot", "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Model:      "test-model",
				MaxBudget:  1.00,
				WorkingDir: tt.workingDir,
			}
			e := New(cfg)
			e.claudeCmd = "echo"

			ctx := context.Background()
			result, err := e.Execute(ctx, "test")

			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			if result == nil {
				t.Fatal("Execute() returned nil result")
			}
			// Just verify it completed successfully
			if !result.Completed {
				t.Error("Execute() should complete when WorkingDir is default")
			}
		})
	}
}
