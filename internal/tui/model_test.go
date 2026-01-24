package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	m := NewModel()

	if len(m.outputLines) != 0 {
		t.Errorf("expected empty outputLines, got %d", len(m.outputLines))
	}

	if len(m.tasks) != 0 {
		t.Errorf("expected empty tasks, got %d", len(m.tasks))
	}

	if m.ready {
		t.Error("expected model not to be ready initially")
	}
}

func TestModelInit(t *testing.T) {
	m := NewModel()
	cmd := m.Init()

	if cmd != nil {
		t.Error("expected Init() to return nil")
	}
}

func TestModelUpdateWindowSize(t *testing.T) {
	m := NewModel()

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, cmd := m.Update(msg)

	model := updatedModel.(Model)

	if cmd != nil {
		t.Error("expected no command from window size update")
	}

	if !model.ready {
		t.Error("expected model to be ready after window size message")
	}

	if model.layout.Width != 120 {
		t.Errorf("expected width 120, got %d", model.layout.Width)
	}

	if model.layout.Height != 40 {
		t.Errorf("expected height 40, got %d", model.layout.Height)
	}
}

func TestModelUpdateQuit(t *testing.T) {
	m := NewModel()

	// Simulate q key press
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Error("expected quit command from 'q' key")
	}
}

func TestModelViewNotReady(t *testing.T) {
	m := NewModel()
	view := m.View()

	if view != "Initializing..." {
		t.Errorf("expected 'Initializing...' when not ready, got %q", view)
	}
}

func TestModelViewTooSmall(t *testing.T) {
	m := NewModel()

	// Simulate a too-small window
	msg := tea.WindowSizeMsg{Width: 60, Height: 40}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	view := model.View()

	if !strings.Contains(view, "too narrow") {
		t.Errorf("expected 'too narrow' message, got %q", view)
	}
}

func TestModelViewFull(t *testing.T) {
	m := NewModel()

	// Set up the model with valid dimensions
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	// Set some data
	model.SetProgress(ProgressInfo{
		Iteration:    3,
		MaxIteration: 50,
		StepName:     "implement",
		StepPosition: 2,
		StepTotal:    4,
		TokensIn:     45231,
		TokensOut:    12847,
		Cost:         2.34,
		Budget:       10.00,
	})

	model.SetSession(SessionInfo{
		SpecFiles: []string{"docs/plans/auth-feature.md"},
		NotesFile: ".orbital/notes.md",
		StateFile: ".orbital/state.json",
	})

	view := model.View()

	// Check that key elements are present
	if !strings.Contains(view, "Iteration") {
		t.Error("expected 'Iteration' in view")
	}

	if !strings.Contains(view, "Tokens") {
		t.Error("expected 'Tokens' in view")
	}

	if !strings.Contains(view, "Cost") {
		t.Error("expected 'Cost' in view")
	}

	if !strings.Contains(view, "Spec") {
		t.Error("expected 'Spec' in view")
	}
}

func TestModelSetTasks(t *testing.T) {
	m := NewModel()

	// Set up valid dimensions first
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	tasks := []Task{
		{ID: "1", Content: "Set up auth middleware", Status: "completed"},
		{ID: "2", Content: "Implement login endpoint", Status: "in_progress"},
		{ID: "3", Content: "Add session management", Status: "pending"},
	}

	model.SetTasks(tasks)

	if len(model.tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(model.tasks))
	}

	// Layout should be recalculated
	if model.layout.TaskPanelHeight == 0 {
		t.Error("expected task panel height to be non-zero after setting tasks")
	}
}

func TestModelAppendOutput(t *testing.T) {
	m := NewModel()

	m.AppendOutput("Line 1")
	m.AppendOutput("Line 2")

	if len(m.outputLines) != 2 {
		t.Errorf("expected 2 output lines, got %d", len(m.outputLines))
	}

	if m.outputLines[0] != "Line 1" {
		t.Errorf("expected 'Line 1', got %q", m.outputLines[0])
	}
}

func TestModelClearOutput(t *testing.T) {
	m := NewModel()

	m.AppendOutput("Line 1")
	m.AppendOutput("Line 2")
	m.ClearOutput()

	if len(m.outputLines) != 0 {
		t.Errorf("expected 0 output lines after clear, got %d", len(m.outputLines))
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{123, "123"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0.00, "$0.00"},
		{1.50, "$1.50"},
		{10.05, "$10.05"},
		{100.99, "$100.99"},
		{1234.56, "$1,234.56"},
	}

	for _, tt := range tests {
		got := formatCurrency(tt.input)
		if got != tt.want {
			t.Errorf("formatCurrency(%f) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatFraction(t *testing.T) {
	tests := []struct {
		a, b int
		want string
	}{
		{1, 10, "1/10"},
		{3, 50, "3/50"},
		{100, 100, "100/100"},
	}

	for _, tt := range tests {
		got := formatFraction(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("formatFraction(%d, %d) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestModelRenderTaskWithIcons(t *testing.T) {
	m := NewModel()

	// Set up valid dimensions
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	tests := []struct {
		task     Task
		wantIcon string
	}{
		{Task{Status: "completed", Content: "Done task"}, "✓"},
		{Task{Status: "in_progress", Content: "Working task"}, "→"},
		{Task{Status: "pending", Content: "Pending task"}, "○"},
	}

	for _, tt := range tests {
		rendered := model.renderTask(tt.task)
		if !strings.Contains(rendered, tt.wantIcon) {
			t.Errorf("renderTask(%s) should contain %q, got %q", tt.task.Status, tt.wantIcon, rendered)
		}
	}
}

func TestModelRenderProgressWarningColour(t *testing.T) {
	m := NewModel()

	// Set up valid dimensions
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	// Test iteration warning (>80%)
	model.SetProgress(ProgressInfo{
		Iteration:    45,
		MaxIteration: 50,
		TokensIn:     1000,
		TokensOut:    500,
		Cost:         9.00,
		Budget:       10.00, // 90% used
	})

	view := model.View()

	// The view should render without panicking
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestWrapLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		width    int
		wantLen  int
		wantFull bool // whether full content should be preserved
	}{
		{
			name:     "short line no wrap",
			line:     "Hello world",
			width:    50,
			wantLen:  1,
			wantFull: true,
		},
		{
			name:     "long line wraps",
			line:     "This is a longer line that should wrap to multiple lines when the width is narrow",
			width:    30,
			wantLen:  3, // approximate
			wantFull: true,
		},
		{
			name:     "very long word breaks at char boundary",
			line:     "/Users/test/Projects/some-project/internal/very/deep/nested/path/file.go",
			width:    40,
			wantLen:  2,
			wantFull: true,
		},
		{
			name:     "zero width returns original",
			line:     "Test line",
			width:    0,
			wantLen:  1,
			wantFull: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapLine(tt.line, tt.width)

			if len(result) < tt.wantLen {
				t.Errorf("wrapLine() returned %d lines, want at least %d", len(result), tt.wantLen)
			}

			if tt.wantFull {
				// Check that all content is preserved (excluding indent spaces)
				combined := strings.Join(result, "")
				combined = strings.ReplaceAll(combined, "    ", "") // remove continuation indents
				// The combined result should contain all the original non-space characters
				originalNoSpace := strings.ReplaceAll(tt.line, " ", "")
				combinedNoSpace := strings.ReplaceAll(combined, " ", "")
				if combinedNoSpace != originalNoSpace {
					t.Errorf("content not preserved: got %q, want %q", combinedNoSpace, originalNoSpace)
				}
			}
		})
	}
}

func TestWrapLineWithANSI(t *testing.T) {
	// Test that ANSI codes are preserved across wrapping
	ansiYellow := "\x1b[33m"
	ansiReset := "\x1b[0m"
	line := ansiYellow + "This is coloured text that should wrap but preserve ANSI codes" + ansiReset

	result := wrapLine(line, 30)

	if len(result) < 2 {
		t.Errorf("expected wrapped lines, got %d lines", len(result))
	}

	// Check first line contains the ANSI start code
	if !strings.Contains(result[0], ansiYellow) {
		t.Error("first line should contain ANSI colour code")
	}

	// Verify content is preserved
	combined := strings.Join(result, "")
	if !strings.Contains(combined, "coloured text") {
		t.Error("wrapped content should preserve text")
	}
}

func TestWrapLineContinuationIndent(t *testing.T) {
	// Test that continuation lines are indented
	line := "This is a long line that will definitely wrap to multiple lines"

	result := wrapLine(line, 25)

	if len(result) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(result))
	}

	// First line should not have continuation indent
	if strings.HasPrefix(result[0], "    ") {
		t.Error("first line should not have continuation indent")
	}

	// Second line should have continuation indent
	if !strings.HasPrefix(result[1], "    ") {
		t.Errorf("continuation line should start with 4 spaces, got %q", result[1])
	}
}

func TestRenderScrollAreaWrap(t *testing.T) {
	m := NewModel()

	// Set up valid dimensions with narrow width
	msg := tea.WindowSizeMsg{Width: 80, Height: 30}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	// Add a very long line
	longLine := "This is a very long output line that should wrap instead of being truncated with ellipsis like before"
	model.AppendOutput(longLine)

	view := model.View()

	// The full content should be visible (after wrapping)
	// Check for key parts of the content that would have been truncated before
	if !strings.Contains(view, "truncated") {
		t.Errorf("full content should be preserved after wrapping, got: %s", view)
	}
}
