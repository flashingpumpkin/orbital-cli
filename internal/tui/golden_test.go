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

// TestGoldenLongPaths tests the TUI with very long file paths.
// This exercises path truncation logic in the session panel.
func TestGoldenLongPaths(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    1,
		MaxIteration: 50,
		TokensIn:     1000,
		TokensOut:    500,
		Cost:         0.25,
		Budget:       10.00,
	}
	opts.Session = &SessionInfo{
		SpecFiles:   []string{"/very/deeply/nested/directory/structure/that/goes/on/and/on/specs/implementation-plan-for-feature-xyz.md"},
		NotesFile:   "/another/extremely/long/path/to/notes/directory/structure/session-notes-2026-01-26.md",
		StateFile:   "/deeply/nested/state/directory/with/many/levels/.orbital/state.json",
		ContextFile: "/path/to/context/with/long/name/context-file-for-session.md",
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	assertGolden(t, []byte(output))
}

// TestGoldenUnicodeContent tests the TUI with Unicode characters in content.
// This exercises proper handling of wide characters and emoji.
func TestGoldenUnicodeContent(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    2,
		MaxIteration: 50,
		TokensIn:     2000,
		TokensOut:    1000,
		Cost:         0.50,
		Budget:       10.00,
	}
	opts.Tasks = []Task{
		{ID: "1", Content: "实现用户认证系统", Status: "completed"},      // Chinese
		{ID: "2", Content: "Добавить поддержку API", Status: "in_progress"}, // Russian
		{ID: "3", Content: "🚀 Deploy to production 🎉", Status: "pending"},  // Emoji
	}
	opts.OutputLines = []string{
		"Processing: こんにちは世界",
		"Status: ✅ Complete",
		"Warning: ⚠️ Check configuration",
		"Building: 🔨 compiling...",
		"Result: café résumé naïve",
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	assertGolden(t, []byte(output))
}

// TestGoldenANSISequences tests the TUI with ANSI escape sequences in content.
// Note: With NO_COLOR=1, styling is disabled, but this tests that content
// containing ANSI sequences is handled gracefully.
func TestGoldenANSISequences(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    3,
		MaxIteration: 50,
		TokensIn:     3000,
		TokensOut:    1500,
		Cost:         0.75,
		Budget:       10.00,
	}
	// Simulate output that might contain ANSI codes (e.g., from Claude's responses)
	opts.OutputLines = []string{
		"\x1b[32mSuccess:\x1b[0m Operation completed",
		"\x1b[31mError:\x1b[0m Something went wrong",
		"\x1b[1mBold text\x1b[0m and normal text",
		"\x1b[33;1mWarning:\x1b[0m Check your input",
		"Plain text without any formatting",
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	assertGolden(t, []byte(output))
}

// TestGoldenTerminalTooNarrow tests the TUI error message when terminal is below minimum width.
// The minimum supported width is 80 columns (MinTerminalWidth). When the terminal is narrower,
// the TUI displays an error message instead of the normal interface.
func TestGoldenTerminalTooNarrow(t *testing.T) {
	opts := GoldenTestOptions{
		Width:  60, // Below MinTerminalWidth (80)
		Height: 24,
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	assertGolden(t, []byte(output))
}

// TestGoldenFooterHighTokens tests the footer with high token counts and costs.
// This exercises number formatting with thousands separators and currency display.
func TestGoldenFooterHighTokens(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    45,
		MaxIteration: 50,
		StepName:     "review",
		StepPosition: 3,
		StepTotal:    3,
		TokensIn:     1234567,
		TokensOut:    987654,
		Cost:         87.50,
		Budget:       100.00,
	}
	opts.Tasks = []Task{
		{ID: "1", Content: "Implement feature", Status: "completed"},
		{ID: "2", Content: "Run tests", Status: "completed"},
		{ID: "3", Content: "Review code", Status: "in_progress"},
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	assertGolden(t, []byte(output))
}

// TestGoldenFooterMaxTasks tests the footer with maximum visible tasks (6).
// The task panel shows at most 6 tasks before scrolling.
// Uses a taller terminal (32 rows) to ensure task panel fits.
func TestGoldenFooterMaxTasks(t *testing.T) {
	opts := GoldenTestOptions{
		Width:  80,
		Height: 32, // Taller to fit 6 tasks
		Progress: &ProgressInfo{
			Iteration:    10,
			MaxIteration: 50,
			TokensIn:     10000,
			TokensOut:    5000,
			Cost:         2.50,
			Budget:       10.00,
		},
		Tasks: []Task{
			{ID: "1", Content: "First task completed", Status: "completed"},
			{ID: "2", Content: "Second task completed", Status: "completed"},
			{ID: "3", Content: "Third task in progress", Status: "in_progress"},
			{ID: "4", Content: "Fourth task pending", Status: "pending"},
			{ID: "5", Content: "Fifth task pending", Status: "pending"},
			{ID: "6", Content: "Sixth task pending", Status: "pending"},
		},
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	assertGolden(t, []byte(output))
}

// TestGoldenFooterOverflowTasks tests the footer when there are more than 6 tasks.
// This exercises the scroll indicator showing how many tasks are hidden.
// Uses a taller terminal (32 rows) to ensure task panel fits.
func TestGoldenFooterOverflowTasks(t *testing.T) {
	opts := GoldenTestOptions{
		Width:  80,
		Height: 32, // Taller to fit overflow tasks
		Progress: &ProgressInfo{
			Iteration:    5,
			MaxIteration: 50,
			TokensIn:     5000,
			TokensOut:    2500,
			Cost:         1.25,
			Budget:       10.00,
		},
		Tasks: []Task{
			{ID: "1", Content: "Task one completed", Status: "completed"},
			{ID: "2", Content: "Task two completed", Status: "completed"},
			{ID: "3", Content: "Task three in progress", Status: "in_progress"},
			{ID: "4", Content: "Task four pending", Status: "pending"},
			{ID: "5", Content: "Task five pending", Status: "pending"},
			{ID: "6", Content: "Task six pending", Status: "pending"},
			{ID: "7", Content: "Task seven pending", Status: "pending"},
			{ID: "8", Content: "Task eight pending", Status: "pending"},
			{ID: "9", Content: "Task nine pending", Status: "pending"},
		},
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	assertGolden(t, []byte(output))
}

// TestGoldenFooterZeroBudget tests the footer with zero budget.
// This exercises division-by-zero protection in ratio calculations.
func TestGoldenFooterZeroBudget(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    5,
		MaxIteration: 50,
		TokensIn:     5000,
		TokensOut:    2500,
		Cost:         1.25,
		Budget:       0.00, // Zero budget
	}
	opts.Tasks = []Task{
		{ID: "1", Content: "Working task", Status: "in_progress"},
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	assertGolden(t, []byte(output))
}

// TestGoldenFooterFullProgress tests the footer with near-complete progress.
// This exercises the progress bar at high fill levels and warning styling.
func TestGoldenFooterFullProgress(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    49,
		MaxIteration: 50,
		StepName:     "implement",
		StepPosition: 1,
		StepTotal:    1,
		TokensIn:     500000,
		TokensOut:    250000,
		Cost:         95.00,
		Budget:       100.00, // 95% budget consumed
	}
	opts.Tasks = []Task{
		{ID: "1", Content: "Final task almost done", Status: "in_progress"},
	}

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	assertGolden(t, []byte(output))
}

// TestGoldenFooterNoTasksWithSession tests the footer with no tasks but session info.
// This verifies the layout when only session panel is shown without task panel.
func TestGoldenFooterNoTasksWithSession(t *testing.T) {
	opts := DefaultGoldenOptions()
	opts.Progress = &ProgressInfo{
		Iteration:    1,
		MaxIteration: 50,
		TokensIn:     1000,
		TokensOut:    500,
		Cost:         0.25,
		Budget:       10.00,
	}
	opts.Session = &SessionInfo{
		SpecFiles:   []string{"docs/plans/feature-spec.md"},
		NotesFile:   "docs/notes/session-notes.md",
		StateFile:   ".orbital/state.json",
		ContextFile: "context.md",
	}
	// No tasks - tests layout without task panel

	output := renderToString(t, opts)

	if output == "" {
		t.Fatal("expected non-empty output")
	}

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
