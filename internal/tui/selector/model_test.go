package selector

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flashingpumpkin/orbital/internal/session"
	"github.com/flashingpumpkin/orbital/internal/util"
)

// sendKey simulates a key press and returns the updated model.
func sendKey(m Model, key string) Model {
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return newModel.(Model)
}

// sendSpecialKey simulates a special key press.
func sendSpecialKey(m Model, keyType tea.KeyType) Model {
	newModel, _ := m.Update(tea.KeyMsg{Type: keyType})
	return newModel.(Model)
}

// sendWindowSize sends a window size message.
func sendWindowSize(m Model, width, height int) Model {
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return newModel.(Model)
}

func TestNew(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "test", Valid: true},
	}
	m := New(sessions)

	if len(m.sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(m.sessions))
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}
	if m.showCleanup {
		t.Error("expected showCleanup to be false")
	}
}

func TestInit(t *testing.T) {
	m := New(nil)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := New(nil)
	if m.ready {
		t.Error("model should not be ready before window size")
	}

	m = sendWindowSize(m, 80, 24)
	if !m.ready {
		t.Error("model should be ready after window size")
	}
	if m.width != 80 {
		t.Errorf("expected width 80, got %d", m.width)
	}
	if m.height != 24 {
		t.Errorf("expected height 24, got %d", m.height)
	}
}

func TestNavigationUpDown(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "first", Valid: true},
		{ID: "2", Name: "second", Valid: true},
		{ID: "3", Name: "third", Valid: true},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)

	// Initial position
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}

	// Move down
	m = sendKey(m, "j")
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}

	// Move down with arrow
	m = sendSpecialKey(m, tea.KeyDown)
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", m.cursor)
	}

	// Move down at bottom (should stay)
	m = sendKey(m, "j")
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", m.cursor)
	}

	// Move up
	m = sendKey(m, "k")
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}

	// Move up with arrow
	m = sendSpecialKey(m, tea.KeyUp)
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}

	// Move up at top (should stay)
	m = sendKey(m, "k")
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}
}

func TestQuitKeys(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"q key", "q"},
		{"esc key", "esc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New([]session.Session{{Valid: true}})
			m = sendWindowSize(m, 80, 24)

			var cmd tea.Cmd
			if tt.key == "esc" {
				var newModel tea.Model
				newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
				m = newModel.(Model)
			} else {
				var newModel tea.Model
				newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
				m = newModel.(Model)
			}

			if !m.quitting {
				t.Error("expected quitting to be true")
			}
			if !m.result.Cancelled {
				t.Error("expected result.Cancelled to be true")
			}
			if cmd == nil {
				t.Error("expected quit command")
			}
		})
	}
}

func TestSelectValidSession(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "first", Valid: true},
		{ID: "2", Name: "second", Valid: true},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)

	// Move to second session
	m = sendKey(m, "j")

	// Select it
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if !m.quitting {
		t.Error("expected quitting to be true")
	}
	if m.result.Session == nil {
		t.Error("expected result.Session to be set")
	}
	if m.result.Session.ID != "2" {
		t.Errorf("expected session ID '2', got '%s'", m.result.Session.ID)
	}
	if m.result.Cancelled {
		t.Error("expected result.Cancelled to be false")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestSelectInvalidSessionShowsCleanupDialog(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "invalid-session", Valid: false, InvalidReason: "test reason"},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)

	// Try to select invalid session
	m = sendSpecialKey(m, tea.KeyEnter)

	if !m.showCleanup {
		t.Error("expected cleanup dialog to be shown")
	}
	if m.quitting {
		t.Error("expected quitting to be false")
	}
	if m.result.Session != nil {
		t.Error("expected result.Session to be nil")
	}
}

func TestCleanupDialogNavigation(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "invalid-session", Valid: false, InvalidReason: "test"},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)
	m.showCleanup = true
	m.cleanupChoice = 1 // Default is No

	// Move to Yes
	m = sendKey(m, "h")
	if m.cleanupChoice != 0 {
		t.Errorf("expected cleanupChoice 0 (yes), got %d", m.cleanupChoice)
	}

	// Move back to No
	m = sendKey(m, "l")
	if m.cleanupChoice != 1 {
		t.Errorf("expected cleanupChoice 1 (no), got %d", m.cleanupChoice)
	}

	// Tab cycles
	m = sendKey(m, "tab")
	if m.cleanupChoice != 0 {
		t.Errorf("expected cleanupChoice 0 after tab, got %d", m.cleanupChoice)
	}
}

func TestCleanupDialogEsc(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "invalid-session", Valid: false, InvalidReason: "test"},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)
	m.showCleanup = true

	// Press escape
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)

	if m.showCleanup {
		t.Error("expected cleanup dialog to be closed")
	}
	if m.quitting {
		t.Error("expected quitting to be false")
	}
}

func TestCleanupDialogYesShortcut(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "invalid-session", Valid: false, InvalidReason: "test"},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)
	m.showCleanup = true

	// Press y for yes
	m = sendKey(m, "y")

	if m.showCleanup {
		t.Error("expected cleanup dialog to be closed")
	}
	if len(m.sessions) != 0 {
		t.Errorf("expected session to be removed, got %d sessions", len(m.sessions))
	}
}

func TestCleanupDialogNoShortcut(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "invalid-session", Valid: false, InvalidReason: "test"},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)
	m.showCleanup = true

	// Press n for no
	m = sendKey(m, "n")

	if m.showCleanup {
		t.Error("expected cleanup dialog to be closed")
	}
	if len(m.result.CleanupPaths) != 0 {
		t.Errorf("expected 0 cleanup paths, got %d", len(m.result.CleanupPaths))
	}
	if len(m.sessions) != 1 {
		t.Errorf("expected session to remain, got %d sessions", len(m.sessions))
	}
}

func TestCleanupConfirmYes(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "invalid-session", Valid: false, InvalidReason: "test"},
		{ID: "2", Name: "valid-session", Valid: true},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)
	m.showCleanup = true
	m.cleanupChoice = 0 // Yes

	// Confirm
	m = sendSpecialKey(m, tea.KeyEnter)

	if m.showCleanup {
		t.Error("expected cleanup dialog to be closed")
	}
	if len(m.sessions) != 1 {
		t.Errorf("expected 1 session remaining, got %d", len(m.sessions))
	}
	if m.sessions[0].ID != "2" {
		t.Errorf("expected remaining session to be '2', got '%s'", m.sessions[0].ID)
	}
}

func TestCleanupConfirmNo(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "invalid-session", Valid: false, InvalidReason: "test"},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)
	m.showCleanup = true
	m.cleanupChoice = 1 // No

	// Confirm
	m = sendSpecialKey(m, tea.KeyEnter)

	if m.showCleanup {
		t.Error("expected cleanup dialog to be closed")
	}
	if len(m.sessions) != 1 {
		t.Errorf("expected session to remain, got %d sessions", len(m.sessions))
	}
	if len(m.result.CleanupPaths) != 0 {
		t.Errorf("expected 0 cleanup paths, got %d", len(m.result.CleanupPaths))
	}
}

func TestCleanupLastSessionQuitsIfNoSessionsRemain(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "invalid-session", Valid: false, InvalidReason: "test"},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)
	m.showCleanup = true
	m.cleanupChoice = 0 // Yes

	// Confirm - this removes the only session
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if !m.quitting {
		t.Error("expected quitting to be true when no sessions remain")
	}
	if !m.result.Cancelled {
		t.Error("expected result.Cancelled to be true")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestCursorAdjustsAfterCleanup(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "first", Valid: true},
		{ID: "2", Name: "second", Valid: false, InvalidReason: "test"},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)

	// Move to second (invalid) session
	m = sendKey(m, "j")
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}

	// Show cleanup and confirm removal
	m.showCleanup = true
	m.cleanupChoice = 0

	m = sendSpecialKey(m, tea.KeyEnter)

	// Cursor should adjust since we removed the item at cursor position
	if m.cursor != 0 {
		t.Errorf("expected cursor to adjust to 0, got %d", m.cursor)
	}
	if len(m.sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(m.sessions))
	}
}

func TestViewNotReady(t *testing.T) {
	m := New(nil)
	view := m.View()
	if view != "Initializing..." {
		t.Errorf("expected 'Initializing...' when not ready, got '%s'", view)
	}
}

func TestViewSessionList(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "test-session", Valid: true, CreatedAt: time.Now()},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)

	view := m.View()

	if !containsString(view, "ORBITAL CONTINUE") {
		t.Error("expected view to contain title")
	}
	if !containsString(view, "test-session") {
		t.Error("expected view to contain session name")
	}
	if !containsString(view, "Valid") {
		t.Error("expected view to contain valid indicator")
	}
}

func TestViewCleanupDialog(t *testing.T) {
	sessions := []session.Session{
		{ID: "1", Name: "stale-session", Valid: false, InvalidReason: "Session not found"},
	}
	m := New(sessions)
	m = sendWindowSize(m, 80, 24)
	m.showCleanup = true

	view := m.View()

	if !containsString(view, "Remove Stale Session") {
		t.Error("expected view to contain dialog title")
	}
	if !containsString(view, "stale-session") {
		t.Error("expected view to contain session name")
	}
	if !containsString(view, "Session not found") {
		t.Error("expected view to contain invalid reason")
	}
	if !containsString(view, "Yes, remove") {
		t.Error("expected view to contain yes button")
	}
	if !containsString(view, "No, go back") {
		t.Error("expected view to contain no button")
	}
}

func TestViewEmptySessions(t *testing.T) {
	m := New(nil)
	m = sendWindowSize(m, 80, 24)

	view := m.View()

	if !containsString(view, "No sessions found") {
		t.Error("expected view to show no sessions message")
	}
}

func TestFormatSpecs(t *testing.T) {
	tests := []struct {
		name     string
		specs    []string
		expected string
	}{
		{"empty", []string{}, "(none)"},
		{"single short", []string{"file.md"}, "file.md"},
		{"single long", []string{"/very/long/path/that/exceeds/the/maximum/allowed/length.md"}, "...exceeds/the/maximum/allowed/length.md"},
		{"multiple", []string{"a.md", "b.md", "c.md"}, "3 files"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSpecs(tt.specs)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFormatTimeAgo(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{"zero", time.Time{}, "unknown"},
		{"just now", time.Now().Add(-30 * time.Second), "just now"},
		{"1 minute", time.Now().Add(-1 * time.Minute), "1 minute ago"},
		{"5 minutes", time.Now().Add(-5 * time.Minute), "5 minutes ago"},
		{"1 hour", time.Now().Add(-1 * time.Hour), "1 hour ago"},
		{"3 hours", time.Now().Add(-3 * time.Hour), "3 hours ago"},
		{"1 day", time.Now().Add(-24 * time.Hour), "1 day ago"},
		{"3 days", time.Now().Add(-72 * time.Hour), "3 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeAgo(tt.time)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestIntToString(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123, "123"},
		{-5, "-5"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := util.IntToString(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestResultMethod(t *testing.T) {
	m := New(nil)
	m.result.Cancelled = true
	m.result.CleanupPaths = []string{"/path1", "/path2"}

	result := m.Result()

	if !result.Cancelled {
		t.Error("expected Cancelled to be true")
	}
	if len(result.CleanupPaths) != 2 {
		t.Errorf("expected 2 cleanup paths, got %d", len(result.CleanupPaths))
	}
}

// containsString checks if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
