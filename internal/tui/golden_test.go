package tui

import (
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
)

// GoldenTestOptions configures a golden file test.
type GoldenTestOptions struct {
	Width       int
	Height      int
	Progress    *ProgressInfo
	Session     *SessionInfo
	Tasks       []Task
	OutputLines []string
}

// DefaultGoldenOptions returns sensible defaults for golden file testing.
func DefaultGoldenOptions() GoldenTestOptions {
	return GoldenTestOptions{
		Width:  80,
		Height: 24,
	}
}

// assertGolden compares output against a golden file.
// Run with -update flag to update golden files.
// The golden file name is derived from the test function name.
func assertGolden(t *testing.T, output []byte) {
	t.Helper()
	golden.RequireEqual(t, output)
}

// createGoldenTestModel creates a Model configured for golden file testing.
// It sets NO_COLOR=1 to ensure deterministic output across environments.
func createGoldenTestModel(t *testing.T, opts GoldenTestOptions) *teatest.TestModel {
	t.Helper()

	// Ensure deterministic colour output
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	// Create model
	model := NewModel()

	// Create test model with specified terminal size
	tm := teatest.NewTestModel(
		t,
		model,
		teatest.WithInitialTermSize(opts.Width, opts.Height),
	)

	// Send window size message to initialize layout
	tm.Send(tea.WindowSizeMsg{Width: opts.Width, Height: opts.Height})

	// Allow the model to process the message
	time.Sleep(10 * time.Millisecond)

	return tm
}

// renderToString renders a Model to a string for snapshot comparison.
// This bypasses teatest for simpler, faster testing of view output.
func renderToString(t *testing.T, opts GoldenTestOptions) string {
	t.Helper()

	// Ensure deterministic colour output
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	// Create and configure model
	model := NewModel()

	// Initialize with window size
	msg := tea.WindowSizeMsg{Width: opts.Width, Height: opts.Height}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(Model)

	// Apply progress if provided
	if opts.Progress != nil {
		model.SetProgress(*opts.Progress)
	}

	// Apply session if provided
	if opts.Session != nil {
		model.SetSession(*opts.Session)
	}

	// Apply tasks if provided
	if opts.Tasks != nil {
		model.SetTasks(opts.Tasks)
	}

	// Apply output lines if provided
	for _, line := range opts.OutputLines {
		model.AppendOutput(line)
	}

	// Render and return
	return model.View()
}

// TestGoldenEmpty tests the empty TUI state.
func TestGoldenEmpty(t *testing.T) {
	opts := DefaultGoldenOptions()

	output := renderToString(t, opts)

	// Verify basic structure exists
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Compare against golden file
	assertGolden(t, []byte(output))
}

// TestGoldenWithProgress tests the TUI with progress information.
func TestGoldenWithProgress(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    5,
		MaxIteration: 50,
		StepName:     "implement",
		StepPosition: 1,
		StepTotal:    3,
		TokensIn:     12345,
		TokensOut:    6789,
		Cost:         1.50,
		Budget:       10.00,
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Compare against golden file
	assertGolden(t, []byte(output))
}

// TestGoldenSingleTask tests the TUI with a single task.
func TestGoldenSingleTask(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    1,
		MaxIteration: 50,
		TokensIn:     1000,
		TokensOut:    500,
		Cost:         0.25,
		Budget:       10.00,
	}
	opts.Tasks = []Task{
		{ID: "1", Content: "Set up authentication", Status: "in_progress"},
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Compare against golden file
	assertGolden(t, []byte(output))
}

// TestGoldenMultipleTasks tests the TUI with multiple tasks in different states.
func TestGoldenMultipleTasks(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    3,
		MaxIteration: 50,
		TokensIn:     5000,
		TokensOut:    2500,
		Cost:         0.75,
		Budget:       10.00,
	}
	opts.Tasks = []Task{
		{ID: "1", Content: "Set up authentication", Status: "completed"},
		{ID: "2", Content: "Implement login endpoint", Status: "in_progress"},
		{ID: "3", Content: "Add session management", Status: "pending"},
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Compare against golden file
	assertGolden(t, []byte(output))
}

// TestGoldenScrollingContent tests the TUI with output that requires scrolling.
func TestGoldenScrollingContent(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    10,
		MaxIteration: 50,
		TokensIn:     50000,
		TokensOut:    25000,
		Cost:         5.00,
		Budget:       10.00,
	}

	// Add enough output lines to trigger scrolling
	opts.OutputLines = make([]string, 50)
	for i := range opts.OutputLines {
		opts.OutputLines[i] = "This is output line content for testing scroll behaviour"
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Compare against golden file
	assertGolden(t, []byte(output))
}

// TestGoldenNarrowTerminal tests the TUI in a narrow terminal.
func TestGoldenNarrowTerminal(t *testing.T) {
	opts := GoldenTestOptions{
		Width:  80, // Minimum supported width
		Height: 24,
		Progress: &ProgressInfo{
			Iteration:    1,
			MaxIteration: 50,
			TokensIn:     1000,
			TokensOut:    500,
			Cost:         0.10,
			Budget:       10.00,
		},
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Compare against golden file
	assertGolden(t, []byte(output))
}

// TestGoldenTeatestIntegration demonstrates the full teatest integration harness.
// This test verifies that the teatest-based harness works correctly for later
// golden file comparison tests.
func TestGoldenTeatestIntegration(t *testing.T) {
	opts := DefaultGoldenOptions()

	tm := createGoldenTestModel(t, opts)

	// Send quit command to terminate the program
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// Get final output (with timeout to prevent hanging)
	output, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if len(output) == 0 {
		t.Fatal("expected non-empty output from teatest harness")
	}
}
