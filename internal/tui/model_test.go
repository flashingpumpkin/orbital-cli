package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/flashingpumpkin/orbital/internal/util"
)

func TestNewModel(t *testing.T) {
	m := NewModel()

	if m.outputLines.Len() != 0 {
		t.Errorf("expected empty outputLines, got %d", m.outputLines.Len())
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

	if m.outputLines.Len() != 2 {
		t.Errorf("expected 2 output lines, got %d", m.outputLines.Len())
	}

	if m.outputLines.Get(0) != "Line 1" {
		t.Errorf("expected 'Line 1', got %q", m.outputLines.Get(0))
	}
}

func TestModelClearOutput(t *testing.T) {
	m := NewModel()

	m.AppendOutput("Line 1")
	m.AppendOutput("Line 2")
	m.ClearOutput()

	if m.outputLines.Len() != 0 {
		t.Errorf("expected 0 output lines after clear, got %d", m.outputLines.Len())
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
		got := util.FormatNumber(tt.input)
		if got != tt.want {
			t.Errorf("util.FormatNumber(%d) = %q, want %q", tt.input, got, tt.want)
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

	// Updated icons from styles.go
	tests := []struct {
		task     Task
		wantIcon string
	}{
		{Task{Status: "completed", Content: "Done task"}, IconComplete},  // ●
		{Task{Status: "in_progress", Content: "Working task"}, IconInProgress}, // →
		{Task{Status: "pending", Content: "Pending task"}, IconPending}, // ○
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

func TestTruncateFromStart(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		targetWidth int
		wantPrefix  string // expected prefix ("..." for truncated)
		wantSuffix  string // expected ending
		wantWidth   int    // expected visible width (approximate)
	}{
		{
			name:        "short string no truncation",
			input:       "file.go",
			targetWidth: 20,
			wantPrefix:  "",
			wantSuffix:  "file.go",
			wantWidth:   7,
		},
		{
			name:        "long path truncated",
			input:       "/Users/test/Projects/orbital/internal/tui/model.go",
			targetWidth: 30,
			wantPrefix:  "...",
			wantSuffix:  "model.go",
			wantWidth:   30, // Should fill target width
		},
		{
			name:        "exact fit no truncation",
			input:       "exactly-thirty-characters.txt",
			targetWidth: 30,
			wantPrefix:  "",
			wantSuffix:  ".txt",
			wantWidth:   29,
		},
		{
			name:        "zero width returns ellipsis",
			input:       "test.go",
			targetWidth: 0,
			wantPrefix:  "...",
			wantSuffix:  "...",
			wantWidth:   3,
		},
		{
			name:        "preserves filename",
			input:       "docs/notes/2026-01-25-notes-223905-continuous-improvement.md",
			targetWidth: 40,
			wantPrefix:  "...",
			wantSuffix:  "continuous-improvement.md",
			wantWidth:   40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateFromStart(tt.input, tt.targetWidth)

			// Check prefix
			if tt.wantPrefix != "" && !strings.HasPrefix(result, tt.wantPrefix) {
				t.Errorf("truncateFromStart() = %q, want prefix %q", result, tt.wantPrefix)
			}

			// Check suffix
			if !strings.HasSuffix(result, tt.wantSuffix) {
				t.Errorf("truncateFromStart() = %q, want suffix %q", result, tt.wantSuffix)
			}

			// Check width doesn't exceed target
			width := ansi.StringWidth(result)
			if tt.targetWidth > 0 && width > tt.targetWidth+3 { // Allow for "..." prefix
				t.Errorf("truncateFromStart() width = %d, want <= %d", width, tt.targetWidth+3)
			}
		})
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

func TestRenderScrollAreaRespectScrollState(t *testing.T) {
	t.Run("tailing shows most recent lines", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add many lines
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Ensure we're tailing (default)
		if !model.outputTailing {
			t.Fatal("expected outputTailing to be true by default")
		}

		view := model.View()

		// Should see the last line (Line 50)
		if !strings.Contains(view, "Line 50") {
			t.Error("expected tailing to show Line 50 (most recent)")
		}
	})

	t.Run("scroll 0 shows first lines", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add many lines
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Set scroll to 0 (top)
		model.outputTailing = false
		model.outputScroll = 0

		view := model.View()

		// Should see the first line
		if !strings.Contains(view, "Line 1") {
			t.Error("expected scroll 0 to show Line 1")
		}
		// Should NOT see the last line (too far down)
		if strings.Contains(view, "Line 50") {
			t.Error("expected scroll 0 NOT to show Line 50")
		}
	})

	t.Run("scroll offset shows lines from that position", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add many lines
		for i := 0; i < 100; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Set scroll to middle
		model.outputTailing = false
		model.outputScroll = 50

		view := model.View()

		// Should see Line 51 (offset 50 is 0-indexed)
		if !strings.Contains(view, "Line 51") {
			t.Error("expected scroll 50 to show Line 51")
		}
		// Should NOT see first or last line
		if strings.Contains(view, "Line 1") {
			t.Error("expected scroll 50 NOT to show Line 1")
		}
		if strings.Contains(view, "Line 100") {
			t.Error("expected scroll 50 NOT to show Line 100")
		}
	})

	t.Run("scroll offset clamped when invalid", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add only 10 lines
		for i := 0; i < 10; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Set scroll to invalid high value
		model.outputTailing = false
		model.outputScroll = 1000

		// Should render without panic, showing available content
		view := model.View()

		// All lines should be visible since content is short
		if !strings.Contains(view, "Line 1") {
			t.Error("expected all content visible when scroll is clamped")
		}
		if !strings.Contains(view, "Line 10") {
			t.Error("expected all content visible when scroll is clamped")
		}
	})

	t.Run("short output with padding", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add only 3 lines
		model.AppendOutput("First line")
		model.AppendOutput("Second line")
		model.AppendOutput("Third line")

		// Render should succeed (with padding)
		view := model.View()

		if !strings.Contains(view, "First line") {
			t.Error("expected short output to show all lines")
		}
		if !strings.Contains(view, "Third line") {
			t.Error("expected short output to show all lines")
		}
	})
}

func TestScrollUpOutputTab(t *testing.T) {
	t.Run("tailing unlocks and positions one line up from bottom", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling (more than viewport height)
		for i := 0; i < 30; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Verify initial state: tailing is true
		if !model.outputTailing {
			t.Error("expected outputTailing to be true initially")
		}

		// Press up arrow
		keyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Verify tailing is now false
		if model.outputTailing {
			t.Error("expected outputTailing to be false after scroll up")
		}

		// Verify scroll position is one line up from bottom
		wrappedLines := model.wrapAllOutputLines()
		height := model.layout.ScrollAreaHeight
		maxOffset := len(wrappedLines) - height
		expectedScroll := maxOffset - 1

		if model.outputScroll != expectedScroll {
			t.Errorf("expected outputScroll to be %d, got %d", expectedScroll, model.outputScroll)
		}
	})

	t.Run("not tailing decrements scroll offset", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 30; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Scroll up once to unlock tailing
		keyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		previousScroll := model.outputScroll

		// Scroll up again
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		if model.outputScroll != previousScroll-1 {
			t.Errorf("expected outputScroll to be %d, got %d", previousScroll-1, model.outputScroll)
		}
	})

	t.Run("at top does not go negative", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 30; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Manually set scroll to top
		model.outputTailing = false
		model.outputScroll = 0

		// Press up arrow
		keyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		if model.outputScroll != 0 {
			t.Errorf("expected outputScroll to stay at 0, got %d", model.outputScroll)
		}
	})

	t.Run("output fits in viewport does nothing", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions with large height
		msg := tea.WindowSizeMsg{Width: 80, Height: 50}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add only a few lines (less than viewport)
		for i := 0; i < 5; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Press up arrow
		keyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Should still be tailing since content fits
		if !model.outputTailing {
			t.Error("expected outputTailing to remain true when content fits in viewport")
		}

		if model.outputScroll != 0 {
			t.Errorf("expected outputScroll to remain 0, got %d", model.outputScroll)
		}
	})

	t.Run("file tab scrolling still works", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Set up session with a spec file to create file tabs
		model.SetSession(SessionInfo{
			SpecFiles: []string{"/path/to/spec.md"},
		})
		model.tabs = model.buildTabs()

		// Switch to file tab (tab 1)
		model.activeTab = 1

		// Set file content and initial scroll
		model.fileContents["/path/to/spec.md"] = strings.Repeat("Line\n", 50)
		model.fileScroll["/path/to/spec.md"] = 10

		// Press up arrow
		keyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Verify file scroll decremented
		if model.fileScroll["/path/to/spec.md"] != 9 {
			t.Errorf("expected file scroll to be 9, got %d", model.fileScroll["/path/to/spec.md"])
		}

		// Verify output scroll unchanged
		if model.outputScroll != 0 {
			t.Errorf("expected output scroll to remain 0, got %d", model.outputScroll)
		}
	})
}

func TestScrollDownOutputTab(t *testing.T) {
	t.Run("tailing does nothing", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 30; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Verify initial state: tailing is true
		if !model.outputTailing {
			t.Error("expected outputTailing to be true initially")
		}

		// Press down arrow
		keyMsg := tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Should still be tailing
		if !model.outputTailing {
			t.Error("expected outputTailing to remain true when already at bottom")
		}

		// Scroll should remain at 0
		if model.outputScroll != 0 {
			t.Errorf("expected outputScroll to remain 0, got %d", model.outputScroll)
		}
	})

	t.Run("not tailing increments scroll offset", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 30; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Scroll up to unlock tailing
		keyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Scroll up a few more times to get some distance from bottom
		for i := 0; i < 5; i++ {
			updatedModel, _ = model.Update(keyMsg)
			model = updatedModel.(Model)
		}

		previousScroll := model.outputScroll

		// Now scroll down
		keyMsg = tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		if model.outputScroll != previousScroll+1 {
			t.Errorf("expected outputScroll to be %d, got %d", previousScroll+1, model.outputScroll)
		}

		// Should not be tailing yet (not at bottom)
		if model.outputTailing {
			t.Error("expected outputTailing to be false when not at bottom")
		}
	})

	t.Run("reaching bottom re-locks to tail mode", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 30; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Scroll up to unlock tailing
		keyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Should be one line up from bottom
		if model.outputTailing {
			t.Error("expected outputTailing to be false after scroll up")
		}

		// Now scroll down to return to bottom
		keyMsg = tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Should re-lock to tail mode
		if !model.outputTailing {
			t.Error("expected outputTailing to be true after scrolling to bottom")
		}
	})

	t.Run("new output auto-tails when in tail mode", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions (larger height to ensure scroll area has room)
		msg := tea.WindowSizeMsg{Width: 80, Height: 40}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 30; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Verify we're tailing
		if !model.outputTailing {
			t.Error("expected outputTailing to be true initially")
		}

		// Add more output with unique identifier
		model.AppendOutput("UNIQUE_NEW_OUTPUT_LINE")

		// Render the view to verify it shows the new content
		view := model.View()

		// The new line should be visible (it's at the bottom, and we're tailing)
		if !strings.Contains(view, "UNIQUE_NEW_OUTPUT_LINE") {
			t.Errorf("expected new output to be visible when tailing, view: %s", view)
		}
	})

	t.Run("file tab scrolling still works", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Set up session with a spec file to create file tabs
		model.SetSession(SessionInfo{
			SpecFiles: []string{"/path/to/spec.md"},
		})
		model.tabs = model.buildTabs()

		// Switch to file tab (tab 1)
		model.activeTab = 1

		// Set file content and initial scroll
		model.fileContents["/path/to/spec.md"] = strings.Repeat("Line\n", 50)
		model.fileScroll["/path/to/spec.md"] = 5

		// Press down arrow
		keyMsg := tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Verify file scroll incremented
		if model.fileScroll["/path/to/spec.md"] != 6 {
			t.Errorf("expected file scroll to be 6, got %d", model.fileScroll["/path/to/spec.md"])
		}

		// Verify output scroll unchanged
		if model.outputScroll != 0 {
			t.Errorf("expected output scroll to remain 0, got %d", model.outputScroll)
		}
	})
}

func TestScrollPageUpOutputTab(t *testing.T) {
	t.Run("tailing unlocks and jumps up one page", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions (must be >= 24 for minimum terminal height)
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling (more than 2 pages)
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Verify initial state: tailing is true
		if !model.outputTailing {
			t.Error("expected outputTailing to be true initially")
		}

		// Press page up
		keyMsg := tea.KeyMsg{Type: tea.KeyPgUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Verify tailing is now false
		if model.outputTailing {
			t.Error("expected outputTailing to be false after page up")
		}

		// Verify scroll position is one page up from bottom
		wrappedLines := model.wrapAllOutputLines()
		height := model.layout.ScrollAreaHeight
		maxOffset := len(wrappedLines) - height
		expectedScroll := maxOffset - height
		if expectedScroll < 0 {
			expectedScroll = 0
		}

		if model.outputScroll != expectedScroll {
			t.Errorf("expected outputScroll to be %d, got %d", expectedScroll, model.outputScroll)
		}
	})

	t.Run("scrolled clamps to 0 near top", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions (must be >= 24 for minimum terminal height)
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Manually set scroll position near top
		model.outputTailing = false
		model.outputScroll = 3 // Less than one page height

		// Press page up
		keyMsg := tea.KeyMsg{Type: tea.KeyPgUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Should clamp to 0
		if model.outputScroll != 0 {
			t.Errorf("expected outputScroll to clamp to 0, got %d", model.outputScroll)
		}
	})

	t.Run("output fits in viewport does nothing", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions with large height
		msg := tea.WindowSizeMsg{Width: 80, Height: 50}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add only a few lines (less than viewport)
		for i := 0; i < 5; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Press page up
		keyMsg := tea.KeyMsg{Type: tea.KeyPgUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Should still be tailing since content fits
		if !model.outputTailing {
			t.Error("expected outputTailing to remain true when content fits in viewport")
		}
	})

	t.Run("file tab scrolling still works", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions (must be >= 24 for minimum terminal height)
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Set up session with a spec file to create file tabs
		model.SetSession(SessionInfo{
			SpecFiles: []string{"/path/to/spec.md"},
		})
		model.tabs = model.buildTabs()

		// Switch to file tab (tab 1)
		model.activeTab = 1

		// Set file content and initial scroll
		model.fileContents["/path/to/spec.md"] = strings.Repeat("Line\n", 100)
		model.fileScroll["/path/to/spec.md"] = 50

		// Press page up
		keyMsg := tea.KeyMsg{Type: tea.KeyPgUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Verify file scroll moved up by page height
		height := model.layout.ScrollAreaHeight
		expectedScroll := 50 - height
		if expectedScroll < 0 {
			expectedScroll = 0
		}
		if model.fileScroll["/path/to/spec.md"] != expectedScroll {
			t.Errorf("expected file scroll to be %d, got %d", expectedScroll, model.fileScroll["/path/to/spec.md"])
		}
	})
}

func TestScrollPageDownOutputTab(t *testing.T) {
	t.Run("tailing does nothing", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions (must be >= 24 for minimum terminal height)
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Verify initial state: tailing is true
		if !model.outputTailing {
			t.Error("expected outputTailing to be true initially")
		}

		// Press page down
		keyMsg := tea.KeyMsg{Type: tea.KeyPgDown}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Should still be tailing
		if !model.outputTailing {
			t.Error("expected outputTailing to remain true when already at bottom")
		}
	})

	t.Run("scrolled jumps down one page", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions (must be >= 24 for minimum terminal height)
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 100; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Manually set scroll position near top
		model.outputTailing = false
		model.outputScroll = 10

		previousScroll := model.outputScroll
		height := model.layout.ScrollAreaHeight

		// Press page down
		keyMsg := tea.KeyMsg{Type: tea.KeyPgDown}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Should jump down by page height
		expectedScroll := previousScroll + height
		if model.outputScroll != expectedScroll {
			t.Errorf("expected outputScroll to be %d, got %d", expectedScroll, model.outputScroll)
		}

		// Should not be tailing yet
		if model.outputTailing {
			t.Error("expected outputTailing to be false when not at bottom")
		}
	})

	t.Run("reaching bottom re-locks to tail mode", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions (must be >= 24 for minimum terminal height)
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Calculate max offset
		wrappedLines := model.wrapAllOutputLines()
		height := model.layout.ScrollAreaHeight
		maxOffset := len(wrappedLines) - height

		// Set scroll position close to bottom (less than one page away)
		model.outputTailing = false
		model.outputScroll = maxOffset - (height / 2)

		// Press page down
		keyMsg := tea.KeyMsg{Type: tea.KeyPgDown}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Should re-lock to tail mode
		if !model.outputTailing {
			t.Error("expected outputTailing to be true after reaching bottom")
		}

		// Should be at max offset
		if model.outputScroll != maxOffset {
			t.Errorf("expected outputScroll to be %d, got %d", maxOffset, model.outputScroll)
		}
	})

	t.Run("file tab scrolling still works", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions (must be >= 24 for minimum terminal height)
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Set up session with a spec file to create file tabs
		model.SetSession(SessionInfo{
			SpecFiles: []string{"/path/to/spec.md"},
		})
		model.tabs = model.buildTabs()

		// Switch to file tab (tab 1)
		model.activeTab = 1

		// Set file content and initial scroll
		model.fileContents["/path/to/spec.md"] = strings.Repeat("Line\n", 100)
		model.fileScroll["/path/to/spec.md"] = 10

		previousScroll := model.fileScroll["/path/to/spec.md"]
		height := model.layout.ScrollAreaHeight

		// Press page down
		keyMsg := tea.KeyMsg{Type: tea.KeyPgDown}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Verify file scroll moved down by page height
		expectedScroll := previousScroll + height
		if model.fileScroll["/path/to/spec.md"] != expectedScroll {
			t.Errorf("expected file scroll to be %d, got %d", expectedScroll, model.fileScroll["/path/to/spec.md"])
		}
	})
}

func TestWrapAllOutputLines(t *testing.T) {
	m := NewModel()

	// Set up valid dimensions
	msg := tea.WindowSizeMsg{Width: 80, Height: 20}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	// Add some lines
	model.AppendOutput("Short line")
	model.AppendOutput("This is a much longer line that should wrap when the terminal is narrow enough to require wrapping")

	wrappedLines := model.wrapAllOutputLines()

	// Should have more than 2 lines due to wrapping
	if len(wrappedLines) < 2 {
		t.Errorf("expected at least 2 wrapped lines, got %d", len(wrappedLines))
	}
}

func TestWideTerminalRendering(t *testing.T) {
	// Test that the TUI renders correctly with wide terminals (200+ columns)
	m := NewModel()

	// Set up with wide terminal dimensions
	msg := tea.WindowSizeMsg{Width: 220, Height: 40}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	// Set some data to render
	model.SetProgress(ProgressInfo{
		Iteration:    5,
		MaxIteration: 50,
		StepName:     "implement",
		StepPosition: 2,
		StepTotal:    4,
		TokensIn:     100000,
		TokensOut:    50000,
		Cost:         5.50,
		Budget:       100.00,
	})

	model.SetSession(SessionInfo{
		SpecFiles:   []string{"docs/plans/very-long-filename-that-tests-truncation-behavior.md"},
		NotesFile:   ".orbital/notes.md",
		StateFile:   ".orbital/state.json",
		ContextFile: "docs/context/long-context-file-path.md",
	})

	model.SetTasks([]Task{
		{ID: "1", Content: "Complete a very long task description that should be handled correctly in wide terminals", Status: "completed"},
		{ID: "2", Content: "Another long task that is currently in progress", Status: "in_progress"},
	})

	view := model.View()

	// Verify key elements are present
	if !strings.Contains(view, "ORBITAL") {
		t.Error("expected 'ORBITAL' brand in view")
	}

	if !strings.Contains(view, "Iteration") {
		t.Error("expected 'Iteration' in view")
	}

	if !strings.Contains(view, "implement") {
		t.Error("expected step name 'implement' in view")
	}

	// Verify wide terminal doesn't break borders
	if !strings.Contains(view, BoxTopLeft) {
		t.Error("expected top-left border character")
	}
	if !strings.Contains(view, BoxTopRight) {
		t.Error("expected top-right border character")
	}

	// Verify content width calculation
	if model.layout.ContentWidth() != 218 { // 220 - 2 for borders
		t.Errorf("expected content width 218, got %d", model.layout.ContentWidth())
	}
}

func TestMinimumTerminalRendering(t *testing.T) {
	// Test that the TUI renders correctly at minimum viable size (80x24)
	m := NewModel()

	// Set up with minimum terminal dimensions
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	if model.layout.TooSmall {
		t.Error("expected 80x24 to be viable, but TooSmall is true")
	}

	// Set some data to render
	model.SetProgress(ProgressInfo{
		Iteration:    5,
		MaxIteration: 50,
		StepName:     "implement",
		StepPosition: 2,
		StepTotal:    4,
		TokensIn:     100000,
		TokensOut:    50000,
		Cost:         5.50,
		Budget:       100.00,
	})

	view := model.View()

	// Verify it renders without panic
	if view == "" {
		t.Error("expected non-empty view at minimum size")
	}

	// Verify key elements are present
	if !strings.Contains(view, "ORBITAL") {
		t.Error("expected 'ORBITAL' brand in view")
	}

	// Verify content width calculation
	if model.layout.ContentWidth() != 78 { // 80 - 2 for borders
		t.Errorf("expected content width 78, got %d", model.layout.ContentWidth())
	}
}

func TestEmptyOutputState(t *testing.T) {
	// Test that the empty output state displays correctly
	m := NewModel()

	// Set up valid dimensions
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	// Don't add any output - test empty state
	view := model.View()

	// Verify the waiting message is shown
	if !strings.Contains(view, "Waiting for output") {
		t.Error("expected 'Waiting for output' message in empty state")
	}
}

func TestRenderScrollAreaEdgeCases(t *testing.T) {
	t.Run("narrow terminal does not panic with negative padding", func(t *testing.T) {
		m := NewModel()

		// Set up with very narrow terminal (but above minimum)
		msg := tea.WindowSizeMsg{Width: 80, Height: 24}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Don't add any output - test empty state with centred message
		view := model.View()

		// Should render without panic
		if view == "" {
			t.Error("expected non-empty view")
		}

		// Should show waiting message
		if !strings.Contains(view, "Waiting for output") {
			t.Error("expected waiting message")
		}
	})

	t.Run("zero height scroll area returns empty string", func(t *testing.T) {
		m := NewModel()

		// Set up dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 24}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Force layout to have zero scroll area height
		model.layout.ScrollAreaHeight = 0

		// Should return empty string, not panic
		result := model.renderScrollArea()
		if result != "" {
			t.Errorf("expected empty string for zero height, got %q", result)
		}
	})

	t.Run("negative height scroll area returns empty string", func(t *testing.T) {
		m := NewModel()

		// Set up dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 24}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Force layout to have negative scroll area height
		model.layout.ScrollAreaHeight = -5

		// Should return empty string, not panic
		result := model.renderScrollArea()
		if result != "" {
			t.Errorf("expected empty string for negative height, got %q", result)
		}
	})
}

func TestRenderFileContentEdgeCases(t *testing.T) {
	t.Run("zero height returns empty string", func(t *testing.T) {
		m := NewModel()

		// Set up dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 24}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Set up file content
		model.fileContents["/test/file.txt"] = "Test content\nLine 2\nLine 3"

		// Force layout to have zero scroll area height
		model.layout.ScrollAreaHeight = 0

		// Should return empty string, not panic
		result := model.renderFileContent("/test/file.txt")
		if result != "" {
			t.Errorf("expected empty string for zero height, got %q", result)
		}
	})

	t.Run("negative height returns empty string", func(t *testing.T) {
		m := NewModel()

		// Set up dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 24}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Set up file content
		model.fileContents["/test/file.txt"] = "Test content\nLine 2\nLine 3"

		// Force layout to have negative scroll area height
		model.layout.ScrollAreaHeight = -5

		// Should return empty string, not panic
		result := model.renderFileContent("/test/file.txt")
		if result != "" {
			t.Errorf("expected empty string for negative height, got %q", result)
		}
	})

	t.Run("very narrow content width does not panic", func(t *testing.T) {
		m := NewModel()

		// Set up dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 24}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Set up file content with long lines
		model.fileContents["/test/file.txt"] = "This is a very long line that would normally need truncation"

		// Force layout to have very narrow content width (below the 6 char line number column)
		model.layout.Width = 8 // ContentWidth() will return 6

		// Should render without panic
		result := model.renderFileContent("/test/file.txt")
		if result == "" {
			t.Error("expected non-empty result")
		}
	})

	t.Run("zero content width does not panic", func(t *testing.T) {
		m := NewModel()

		// Set up dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 24}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Set up file content
		model.fileContents["/test/file.txt"] = "Test content"

		// Force layout to have zero width (ContentWidth() returns -2 for width=0)
		model.layout.Width = 0

		// Should render without panic
		result := model.renderFileContent("/test/file.txt")
		// Result may be empty or minimal, but no panic
		_ = result
	})

	t.Run("file not loaded shows loading message without panic", func(t *testing.T) {
		m := NewModel()

		// Set up dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 24}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Don't set file content - test loading state
		// Force very narrow width
		model.layout.Width = 10

		// Should render loading message without panic
		result := model.renderFileContent("/test/file.txt")
		if !strings.Contains(result, "Loading") {
			t.Error("expected loading message")
		}
	})
}

func TestWindowResizeScrollClamping(t *testing.T) {
	t.Run("resize larger maintains scroll position", func(t *testing.T) {
		m := NewModel()

		// Set up initial dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Scroll to a specific position
		model.outputTailing = false
		model.outputScroll = 10

		// Resize to larger terminal
		msg = tea.WindowSizeMsg{Width: 80, Height: 40}
		updatedModel, _ = model.Update(msg)
		model = updatedModel.(Model)

		// Scroll position should be maintained (still valid)
		if model.outputScroll != 10 {
			t.Errorf("expected outputScroll to remain 10, got %d", model.outputScroll)
		}

		// Should still not be tailing
		if model.outputTailing {
			t.Error("expected outputTailing to remain false")
		}
	})

	t.Run("resize smaller clamps scroll position", func(t *testing.T) {
		m := NewModel()

		// Set up initial dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 40}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Calculate max offset for initial size
		wrappedLines := model.wrapAllOutputLines()
		initialHeight := model.layout.ScrollAreaHeight
		initialMaxOffset := len(wrappedLines) - initialHeight

		// Set scroll near the max offset
		model.outputTailing = false
		model.outputScroll = initialMaxOffset - 2

		// Resize to smaller terminal
		msg = tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ = model.Update(msg)
		model = updatedModel.(Model)

		// Calculate new max offset
		newHeight := model.layout.ScrollAreaHeight
		newMaxOffset := len(wrappedLines) - newHeight
		if newMaxOffset < 0 {
			newMaxOffset = 0
		}

		// Scroll position should be clamped to new max
		if model.outputScroll > newMaxOffset {
			t.Errorf("expected outputScroll to be clamped to %d, got %d", newMaxOffset, model.outputScroll)
		}
	})

	t.Run("resize larger fits all output resumes tailing", func(t *testing.T) {
		m := NewModel()

		// Set up initial dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add only 10 lines (less than what will fit in larger terminal)
		for i := 0; i < 10; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Scroll up to unlock tailing
		model.outputTailing = false
		model.outputScroll = 2

		// Resize to much larger terminal where all content fits
		msg = tea.WindowSizeMsg{Width: 80, Height: 50}
		updatedModel, _ = model.Update(msg)
		model = updatedModel.(Model)

		// Should resume tailing since all content fits
		if !model.outputTailing {
			t.Error("expected outputTailing to be true when all content fits after resize")
		}

		// Scroll should be reset to 0
		if model.outputScroll != 0 {
			t.Errorf("expected outputScroll to be 0, got %d", model.outputScroll)
		}
	})

	t.Run("tailing continues after resize", func(t *testing.T) {
		m := NewModel()

		// Set up initial dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Verify we're tailing (default)
		if !model.outputTailing {
			t.Fatal("expected outputTailing to be true by default")
		}

		// Resize terminal
		msg = tea.WindowSizeMsg{Width: 80, Height: 40}
		updatedModel, _ = model.Update(msg)
		model = updatedModel.(Model)

		// Should still be tailing
		if !model.outputTailing {
			t.Error("expected outputTailing to remain true after resize")
		}

		// Verify view shows most recent content
		view := model.View()
		if !strings.Contains(view, "Line 50") {
			t.Error("expected tailing to show Line 50 after resize")
		}
	})
}

func TestRenderLineWidths(t *testing.T) {
	// Test that all rendered lines have the correct width (equal to terminal width)
	m := NewModel()

	// Set up valid dimensions
	terminalWidth := 100
	msg := tea.WindowSizeMsg{Width: terminalWidth, Height: 40}
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

	model.AppendOutput("Test output line")

	view := model.View()

	// Split into lines and verify each line width
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		// Skip the help bar (last line) which doesn't have borders
		if i == len(lines)-1 {
			continue
		}

		lineWidth := ansi.StringWidth(line)
		if lineWidth != terminalWidth {
			t.Errorf("line %d has width %d, expected %d: %q", i, lineWidth, terminalWidth, line)
		}
	}
}

func TestRenderTotalLineCount(t *testing.T) {
	// Test that total rendered lines equals terminal height
	m := NewModel()

	// Set up valid dimensions
	terminalHeight := 40
	msg := tea.WindowSizeMsg{Width: 120, Height: terminalHeight}
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

	// Add some output
	for i := 0; i < 10; i++ {
		model.AppendOutput("Output line " + util.IntToString(i+1))
	}

	view := model.View()
	lines := strings.Split(view, "\n")

	// The total rendered lines should equal terminal height
	if len(lines) != terminalHeight {
		t.Errorf("Total rendered lines = %d, expected terminal height = %d", len(lines), terminalHeight)
		t.Logf("View:\n%s", view)
	}
}

func TestWrappedLinesCaching(t *testing.T) {
	t.Run("cache populated after window size set", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add some lines
		model.AppendOutput("Line 1")
		model.AppendOutput("Line 2")

		// Cache should be populated
		if model.wrappedLinesCache == nil {
			t.Error("expected wrappedLinesCache to be populated after adding output")
		}

		if model.cacheLineCount != 2 {
			t.Errorf("expected cacheLineCount to be 2, got %d", model.cacheLineCount)
		}
	})

	t.Run("cache reused on scroll operations", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add enough lines to enable scrolling
		for i := 0; i < 50; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1))
		}

		// Get the wrapped lines
		wrapped1 := model.wrapAllOutputLines()

		// Scroll up
		keyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Get wrapped lines again - should be same slice (cached)
		wrapped2 := model.wrapAllOutputLines()

		// The slices should have same content
		if len(wrapped1) != len(wrapped2) {
			t.Errorf("expected same wrapped lines length, got %d vs %d", len(wrapped1), len(wrapped2))
		}
	})

	t.Run("cache invalidated on window width change", func(t *testing.T) {
		m := NewModel()

		// Set up initial dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add a long line that will wrap differently at different widths
		model.AppendOutput("This is a very long line that will definitely wrap when the terminal is narrow enough")

		initialCache := model.wrappedLinesCache
		initialCacheWidth := model.cacheWidth

		// Resize to different width
		msg = tea.WindowSizeMsg{Width: 40, Height: 30}
		updatedModel, _ = model.Update(msg)
		model = updatedModel.(Model)

		// Cache should be invalidated and rebuilt
		if model.cacheWidth == initialCacheWidth {
			t.Error("expected cacheWidth to change after resize")
		}

		// Cache should be rebuilt (different content due to wrapping)
		// The narrow width should produce more wrapped lines
		if len(model.wrappedLinesCache) <= len(initialCache) {
			t.Error("expected more wrapped lines at narrower width")
		}
	})

	t.Run("cache updated incrementally on new line", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add initial lines
		model.AppendOutput("Line 1")
		model.AppendOutput("Line 2")

		initialLen := len(model.wrappedLinesCache)

		// Add another line
		model.AppendOutput("Line 3")

		// Cache should have one more line (assuming no wrapping)
		if len(model.wrappedLinesCache) != initialLen+1 {
			t.Errorf("expected cache length to be %d, got %d", initialLen+1, len(model.wrappedLinesCache))
		}

		if model.cacheLineCount != 3 {
			t.Errorf("expected cacheLineCount to be 3, got %d", model.cacheLineCount)
		}
	})

	t.Run("cache cleared on ClearOutput", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 30}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add some lines
		model.AppendOutput("Line 1")
		model.AppendOutput("Line 2")

		// Verify cache exists
		if model.wrappedLinesCache == nil {
			t.Fatal("expected cache to be populated")
		}

		// Clear output
		model.ClearOutput()

		// Cache should be cleared
		if model.wrappedLinesCache != nil {
			t.Error("expected wrappedLinesCache to be nil after ClearOutput")
		}

		if model.cacheLineCount != 0 {
			t.Errorf("expected cacheLineCount to be 0, got %d", model.cacheLineCount)
		}
	})

	t.Run("scrolling 1000 times is fast with caching", func(t *testing.T) {
		m := NewModel()

		// Set up valid dimensions
		msg := tea.WindowSizeMsg{Width: 80, Height: 20}
		updatedModel, _ := m.Update(msg)
		model := updatedModel.(Model)

		// Add many lines
		for i := 0; i < 5000; i++ {
			model.AppendOutput("Line " + util.IntToString(i+1) + " with some additional content to make it longer")
		}

		// Scroll up to unlock tailing
		keyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = model.Update(keyMsg)
		model = updatedModel.(Model)

		// Time scrolling 1000 times - should be fast due to caching
		// (We can't easily measure time in a unit test, but we verify it doesn't rebuild)
		for i := 0; i < 500; i++ {
			// Scroll up
			keyMsg = tea.KeyMsg{Type: tea.KeyUp}
			updatedModel, _ = model.Update(keyMsg)
			model = updatedModel.(Model)

			// Scroll down
			keyMsg = tea.KeyMsg{Type: tea.KeyDown}
			updatedModel, _ = model.Update(keyMsg)
			model = updatedModel.(Model)
		}

		// Verify cache is still valid (line count unchanged)
		if model.cacheLineCount != 5000 {
			t.Errorf("expected cacheLineCount to remain 5000, got %d", model.cacheLineCount)
		}

		// The view should render correctly
		view := model.View()
		if view == "" {
			t.Error("expected non-empty view after scrolling")
		}
	})
}

func TestRenderFullLayoutConsistency(t *testing.T) {
	// Test that renderFull produces the correct number of lines
	// matching the layout calculation
	tests := []struct {
		name       string
		width      int
		height     int
		taskCount  int
		outputLines int
	}{
		{"no tasks no output", 120, 40, 0, 0},
		{"no tasks with output", 120, 40, 0, 10},
		{"3 tasks with output", 120, 40, 3, 10},
		{"max tasks with output", 120, 40, 6, 10},
		{"overflow tasks", 120, 40, 10, 10},
		{"minimum terminal", 80, 24, 0, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel()

			msg := tea.WindowSizeMsg{Width: tt.width, Height: tt.height}
			updatedModel, _ := m.Update(msg)
			model := updatedModel.(Model)

			// Add tasks
			var tasks []Task
			for i := 0; i < tt.taskCount; i++ {
				tasks = append(tasks, Task{
					ID:      util.IntToString(i + 1),
					Content: "Task " + util.IntToString(i+1),
					Status:  "pending",
				})
			}
			model.SetTasks(tasks)

			// Add output lines
			for i := 0; i < tt.outputLines; i++ {
				model.AppendOutput("Output line " + util.IntToString(i+1))
			}

			// Set session info
			model.SetSession(SessionInfo{
				SpecFiles: []string{"spec.md"},
				NotesFile: "notes.md",
				StateFile: "state.json",
			})

			view := model.View()
			lines := strings.Split(view, "\n")

			if len(lines) != tt.height {
				t.Errorf("Rendered %d lines, expected %d (terminal height)", len(lines), tt.height)
				
				// Debug: print breakdown
				t.Logf("Layout breakdown:")
				t.Logf("  HeaderPanelHeight: %d", model.layout.HeaderPanelHeight)
				t.Logf("  TabBarHeight: %d", model.layout.TabBarHeight)
				t.Logf("  ScrollAreaHeight: %d", model.layout.ScrollAreaHeight)
				t.Logf("  TaskPanelHeight: %d", model.layout.TaskPanelHeight)
				t.Logf("  ProgressPanelHeight: %d", model.layout.ProgressPanelHeight)
				t.Logf("  SessionPanelHeight: %d", model.layout.SessionPanelHeight)
				t.Logf("  HelpBarHeight: %d", model.layout.HelpBarHeight)
			}
		})
	}
}

func TestRenderLineWidthsWithLargeValues(t *testing.T) {
	// Test that lines are properly truncated when content exceeds available width.
	// This tests the fix for lines wrapping when token counts or costs are very large.
	m := NewModel()

	// Use minimum terminal width
	terminalWidth := MinTerminalWidth
	msg := tea.WindowSizeMsg{Width: terminalWidth, Height: 40}
	updatedModel, _ := m.Update(msg)
	model := updatedModel.(Model)

	// Set very large token counts that would overflow on narrow terminal
	model.SetProgress(ProgressInfo{
		Iteration:    999,
		MaxIteration: 999,
		StepName:     "very-long-step-name-that-might-overflow",
		StepPosition: 99,
		StepTotal:    99,
		GateRetries:  9,
		MaxRetries:   9,
		TokensIn:     999999999,  // Very large: "999,999,999" = 11 chars
		TokensOut:    999999999,  // Very large: "999,999,999" = 11 chars
		Cost:         99999.99,   // Large cost
		Budget:       100000.00,  // Large budget
	})

	model.SetSession(SessionInfo{
		SpecFiles: []string{"/very/long/path/to/some/deeply/nested/directory/structure/spec-file.md"},
		NotesFile: "/very/long/path/to/notes/that/could/overflow.md",
		StateFile: "/very/long/path/to/state/file.json",
	})

	view := model.View()

	// Split into lines and verify each line width does not exceed terminal width
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		// Skip the help bar (last line) which doesn't have borders
		if i == len(lines)-1 {
			continue
		}

		lineWidth := ansi.StringWidth(line)
		if lineWidth > terminalWidth {
			t.Errorf("line %d exceeds terminal width: has %d, max %d: %q", i, lineWidth, terminalWidth, line)
		}
	}
}
