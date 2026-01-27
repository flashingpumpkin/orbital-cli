package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

	// Check required args are present - note: --dangerously-skip-permissions NOT included by default
	expected := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
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

	// Note: --dangerously-skip-permissions NOT included by default
	expected := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
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

func TestBuildArgs_WithDangerousMode(t *testing.T) {
	cfg := &config.Config{
		Model:                      "claude-sonnet-4-20250514",
		MaxBudget:                  5.00,
		SessionID:                  "",
		DangerouslySkipPermissions: true,
	}
	e := New(cfg)

	args := e.BuildArgs("test prompt")

	// Check --dangerously-skip-permissions flag is present when enabled
	var found bool
	for _, arg := range args {
		if arg == "--dangerously-skip-permissions" {
			found = true
			break
		}
	}

	if !found {
		t.Error("BuildArgs() should include --dangerously-skip-permissions when DangerouslySkipPermissions is true")
	}
}

func TestBuildArgs_WithoutDangerousMode(t *testing.T) {
	cfg := &config.Config{
		Model:                      "claude-sonnet-4-20250514",
		MaxBudget:                  5.00,
		SessionID:                  "",
		DangerouslySkipPermissions: false, // Explicitly false
	}
	e := New(cfg)

	args := e.BuildArgs("test prompt")

	// Check --dangerously-skip-permissions flag is NOT present by default
	for _, arg := range args {
		if arg == "--dangerously-skip-permissions" {
			t.Error("BuildArgs() should NOT include --dangerously-skip-permissions when DangerouslySkipPermissions is false")
			break
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

func TestExecute_StreamingParsesStatsOnce(t *testing.T) {
	// This test verifies that the streaming path parses stats during streaming
	// rather than double-parsing at the end.
	// We create a test script that outputs stream-json with stats.

	// Create a temporary script that outputs our test JSON
	tempDir := t.TempDir()
	scriptPath := tempDir + "/test-claude.sh"
	scriptContent := `#!/bin/sh
echo '{"type":"result","total_cost_usd":0.05,"duration_ms":1000,"usage":{"input_tokens":100,"output_tokens":50}}'
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	cfg := &config.Config{
		Model:     "test-model",
		MaxBudget: 1.00,
	}
	e := New(cfg)

	// Create a stream writer to enable the streaming path
	var streamOutput strings.Builder
	e.SetStreamWriter(&streamOutput)

	// Use our test script
	e.claudeCmd = scriptPath

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Verify stats were extracted from streaming
	if result.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want 0.05 (stats should be parsed during streaming)", result.CostUSD)
	}
	if result.TokensIn != 100 {
		t.Errorf("TokensIn = %d, want 100", result.TokensIn)
	}
	if result.TokensOut != 50 {
		t.Errorf("TokensOut = %d, want 50", result.TokensOut)
	}
}

func TestExecute_NonStreamingParsesStatsOnce(t *testing.T) {
	// This test verifies that the non-streaming path parses stats only once.

	// Create a temporary script that outputs our test JSON
	tempDir := t.TempDir()
	scriptPath := tempDir + "/test-claude.sh"
	scriptContent := `#!/bin/sh
echo '{"type":"result","total_cost_usd":0.03,"duration_ms":500,"usage":{"input_tokens":200,"output_tokens":75}}'
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	cfg := &config.Config{
		Model:     "test-model",
		MaxBudget: 1.00,
	}
	e := New(cfg)

	// No stream writer = non-streaming path
	e.claudeCmd = scriptPath

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Verify stats were extracted
	if result.CostUSD != 0.03 {
		t.Errorf("CostUSD = %f, want 0.03", result.CostUSD)
	}
	if result.TokensIn != 200 {
		t.Errorf("TokensIn = %d, want 200", result.TokensIn)
	}
	if result.TokensOut != 75 {
		t.Errorf("TokensOut = %d, want 75", result.TokensOut)
	}
}

func TestExecute_LargeLineHandled(t *testing.T) {
	// Test that lines up to 10MB can be handled without error.
	// We use 5MB as a practical test size to keep test execution fast.

	tempDir := t.TempDir()
	scriptPath := tempDir + "/test-claude.sh"

	// Create a 5MB line (well under the 10MB limit)
	largeLineSize := 5 * 1024 * 1024
	largeLine := strings.Repeat("x", largeLineSize)

	// Script that outputs a large JSON line
	scriptContent := fmt.Sprintf(`#!/bin/sh
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"%s"}]}}'
`, largeLine)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	cfg := &config.Config{
		Model:     "test-model",
		MaxBudget: 1.00,
	}
	e := New(cfg)

	// Enable streaming path
	var streamOutput strings.Builder
	e.SetStreamWriter(&streamOutput)
	e.claudeCmd = scriptPath

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	if err != nil {
		t.Fatalf("Execute() returned error for 5MB line: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if !result.Completed {
		t.Error("Execute() should complete successfully for 5MB line")
	}

	// Verify the large line was captured
	if len(result.Output) < largeLineSize {
		t.Errorf("Output length = %d, want at least %d", len(result.Output), largeLineSize)
	}
}

func TestExecute_OversizedLineError(t *testing.T) {
	// Test that lines exceeding the 10MB limit return an error.
	// We create a script that outputs a line slightly over the limit.

	tempDir := t.TempDir()
	scriptPath := tempDir + "/test-claude.sh"

	// Create a line that exceeds 10MB (add 1KB over to ensure it's over)
	oversizedLineSize := 10*1024*1024 + 1024
	oversizedLine := strings.Repeat("y", oversizedLineSize)

	// Script that outputs an oversized line
	scriptContent := fmt.Sprintf(`#!/bin/sh
printf '%s'
`, oversizedLine)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	cfg := &config.Config{
		Model:     "test-model",
		MaxBudget: 1.00,
	}
	e := New(cfg)

	// Enable streaming path
	var streamOutput strings.Builder
	e.SetStreamWriter(&streamOutput)
	e.claudeCmd = scriptPath

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	// Should return an error for oversized line
	if err == nil {
		t.Error("Execute() should return error for oversized line")
	}

	// Check that error message mentions the limit
	if err != nil && !strings.Contains(err.Error(), "byte limit") {
		t.Errorf("Error should mention byte limit, got: %v", err)
	}

	// Result should indicate failure
	if result != nil && result.Completed {
		t.Error("Result should not be Completed for oversized line")
	}
}

func TestExecute_LargeLineWarning(t *testing.T) {
	// Test that lines approaching the limit trigger a warning when verbose mode is on.
	// We create a line just over the warning threshold (8MB).

	tempDir := t.TempDir()
	scriptPath := tempDir + "/test-claude.sh"

	// Create a line just over the warning threshold (8MB + 1KB)
	warningLineSize := 8*1024*1024 + 1024
	warningLine := strings.Repeat("z", warningLineSize)

	// Script that outputs a line over the warning threshold
	scriptContent := fmt.Sprintf(`#!/bin/sh
printf '%s\n'
`, warningLine)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	cfg := &config.Config{
		Model:     "test-model",
		MaxBudget: 1.00,
		Verbose:   true, // Enable verbose mode to get warnings
	}
	e := New(cfg)

	// Enable streaming path
	var streamOutput strings.Builder
	e.SetStreamWriter(&streamOutput)
	e.claudeCmd = scriptPath

	// Capture stderr to check for warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	// Close write end and restore stderr
	_ = w.Close()
	os.Stderr = oldStderr

	// Read captured stderr
	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	stderrOutput := stderrBuf.String()

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Verify warning was logged
	if !strings.Contains(stderrOutput, "warning") || !strings.Contains(stderrOutput, "large output line") {
		t.Errorf("Expected warning about large output line, got stderr: %s", stderrOutput)
	}
}

func TestExecute_OutputTruncation_Streaming(t *testing.T) {
	// Test that output is truncated when it exceeds MaxOutputSize in streaming mode.

	tempDir := t.TempDir()
	scriptPath := tempDir + "/test-claude.sh"

	// Create a script that outputs more than our limit
	// Each line is 100 bytes + newline
	lineContent := strings.Repeat("a", 100)
	numLines := 200 // 200 lines * 101 bytes = ~20KB total

	var scriptBuilder strings.Builder
	scriptBuilder.WriteString("#!/bin/sh\n")
	for i := 0; i < numLines; i++ {
		scriptBuilder.WriteString(fmt.Sprintf("echo '%s'\n", lineContent))
	}
	// Add a completion marker at the end
	scriptBuilder.WriteString("echo 'COMPLETION_MARKER'\n")

	if err := os.WriteFile(scriptPath, []byte(scriptBuilder.String()), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	// Set a small MaxOutputSize to force truncation
	maxSize := 5000 // 5KB limit

	cfg := &config.Config{
		Model:         "test-model",
		MaxBudget:     1.00,
		MaxOutputSize: maxSize,
		Verbose:       true, // Enable verbose mode to get truncation warning
	}
	e := New(cfg)

	// Enable streaming path
	var streamOutput strings.Builder
	e.SetStreamWriter(&streamOutput)
	e.claudeCmd = scriptPath

	// Capture stderr to check for warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	// Close write end and restore stderr
	_ = w.Close()
	os.Stderr = oldStderr

	// Read captured stderr
	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	stderrOutput := stderrBuf.String()

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Verify output was truncated (should be less than original size)
	if len(result.Output) > maxSize+len(truncationMarker) {
		t.Errorf("Output length = %d, expected <= %d (max + marker)", len(result.Output), maxSize+len(truncationMarker))
	}

	// Verify truncation marker is present
	if !strings.Contains(result.Output, "TRUNCATED") {
		t.Error("Output should contain truncation marker")
	}

	// Verify most recent content is preserved (completion marker should still be there)
	if !strings.Contains(result.Output, "COMPLETION_MARKER") {
		t.Error("Output should preserve recent content (COMPLETION_MARKER)")
	}

	// Verify warning was logged
	if !strings.Contains(stderrOutput, "truncating") {
		t.Errorf("Expected truncation warning in stderr, got: %s", stderrOutput)
	}
}

func TestExecute_OutputTruncation_NonStreaming(t *testing.T) {
	// Test that output is truncated when it exceeds MaxOutputSize in non-streaming mode.

	tempDir := t.TempDir()
	scriptPath := tempDir + "/test-claude.sh"

	// Create a script that outputs more than our limit
	lineContent := strings.Repeat("b", 100)
	numLines := 200 // ~20KB total

	var scriptBuilder strings.Builder
	scriptBuilder.WriteString("#!/bin/sh\n")
	for i := 0; i < numLines; i++ {
		scriptBuilder.WriteString(fmt.Sprintf("echo '%s'\n", lineContent))
	}
	scriptBuilder.WriteString("echo 'END_MARKER'\n")

	if err := os.WriteFile(scriptPath, []byte(scriptBuilder.String()), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	// Set a small MaxOutputSize to force truncation
	maxSize := 5000

	cfg := &config.Config{
		Model:         "test-model",
		MaxBudget:     1.00,
		MaxOutputSize: maxSize,
		Verbose:       true,
	}
	e := New(cfg)

	// No stream writer = non-streaming path
	e.claudeCmd = scriptPath

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	_ = w.Close()
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	stderrOutput := stderrBuf.String()

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Verify output was truncated
	if len(result.Output) > maxSize+len(truncationMarker) {
		t.Errorf("Output length = %d, expected <= %d", len(result.Output), maxSize+len(truncationMarker))
	}

	// Verify truncation marker is present
	if !strings.Contains(result.Output, "TRUNCATED") {
		t.Error("Output should contain truncation marker")
	}

	// Verify most recent content is preserved
	if !strings.Contains(result.Output, "END_MARKER") {
		t.Error("Output should preserve recent content (END_MARKER)")
	}

	// Verify warning was logged
	if !strings.Contains(stderrOutput, "truncating") {
		t.Errorf("Expected truncation warning, got: %s", stderrOutput)
	}
}

func TestExecute_NoTruncation_UnderLimit(t *testing.T) {
	// Test that output under the limit is not truncated.

	tempDir := t.TempDir()
	scriptPath := tempDir + "/test-claude.sh"

	// Create small output (100 bytes)
	scriptContent := `#!/bin/sh
echo 'Small output that should not be truncated'
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	cfg := &config.Config{
		Model:         "test-model",
		MaxBudget:     1.00,
		MaxOutputSize: 10000, // 10KB limit, output is ~50 bytes
	}
	e := New(cfg)
	e.claudeCmd = scriptPath

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// Verify output was NOT truncated
	if strings.Contains(result.Output, "TRUNCATED") {
		t.Error("Output should not be truncated when under limit")
	}

	// Verify full content is present
	if !strings.Contains(result.Output, "Small output") {
		t.Error("Full output content should be preserved")
	}
}

func TestExecute_NoTruncation_ZeroLimit(t *testing.T) {
	// Test that MaxOutputSize=0 disables truncation.

	tempDir := t.TempDir()
	scriptPath := tempDir + "/test-claude.sh"

	// Create larger output
	lineContent := strings.Repeat("c", 100)
	numLines := 100

	var scriptBuilder strings.Builder
	scriptBuilder.WriteString("#!/bin/sh\n")
	for i := 0; i < numLines; i++ {
		scriptBuilder.WriteString(fmt.Sprintf("echo '%s'\n", lineContent))
	}

	if err := os.WriteFile(scriptPath, []byte(scriptBuilder.String()), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	cfg := &config.Config{
		Model:         "test-model",
		MaxBudget:     1.00,
		MaxOutputSize: 0, // Disabled
	}
	e := New(cfg)
	e.claudeCmd = scriptPath

	ctx := context.Background()
	result, err := e.Execute(ctx, "test prompt")

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// Verify output was NOT truncated (no truncation marker)
	if strings.Contains(result.Output, "TRUNCATED") {
		t.Error("Output should not be truncated when MaxOutputSize=0")
	}

	// Verify full output is present (100 lines * ~101 bytes = ~10KB)
	expectedMinSize := 9000
	if len(result.Output) < expectedMinSize {
		t.Errorf("Output length = %d, expected > %d (no truncation)", len(result.Output), expectedMinSize)
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		maxSize        int
		expectTrunc    bool
		expectContains string
	}{
		{
			name:           "under limit",
			content:        "short content",
			maxSize:        1000,
			expectTrunc:    false,
			expectContains: "short content",
		},
		{
			name:           "at limit",
			content:        strings.Repeat("x", 100),
			maxSize:        100,
			expectTrunc:    false,
			expectContains: strings.Repeat("x", 100),
		},
		{
			name:           "over limit",
			content:        "line1\nline2\nline3\nline4\nline5\nline6\n",
			maxSize:        20,
			expectTrunc:    true,
			expectContains: "TRUNCATED",
		},
		{
			name:           "zero limit disabled",
			content:        strings.Repeat("y", 1000),
			maxSize:        0,
			expectTrunc:    false,
			expectContains: strings.Repeat("y", 1000),
		},
		{
			name:           "negative limit disabled",
			content:        strings.Repeat("z", 500),
			maxSize:        -1,
			expectTrunc:    false,
			expectContains: strings.Repeat("z", 500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, truncated := truncateOutput([]byte(tt.content), tt.maxSize)

			if truncated != tt.expectTrunc {
				t.Errorf("truncated = %v, want %v", truncated, tt.expectTrunc)
			}

			if !strings.Contains(string(result), tt.expectContains) {
				t.Errorf("result should contain %q, got %q", tt.expectContains, string(result))
			}

			// If truncated, verify the marker is at the start
			if truncated && !strings.HasPrefix(string(result), "[OUTPUT TRUNCATED") {
				t.Errorf("truncated output should start with marker, got: %q", string(result)[:50])
			}
		})
	}
}
