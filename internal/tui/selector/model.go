package selector

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flashingpumpkin/orbital/internal/session"
)

// Result contains the outcome of the session selection.
type Result struct {
	// Session is the selected session, or nil if cancelled.
	Session *session.Session

	// CleanupPaths contains paths that the user confirmed for cleanup.
	CleanupPaths []string

	// Cancelled is true if the user quit without selecting.
	Cancelled bool
}

// Model is the bubbletea model for session selection.
type Model struct {
	sessions []session.Session
	cursor   int
	width    int
	height   int
	ready    bool
	quitting bool
	result   Result

	// Cleanup dialog state
	showCleanup   bool
	cleanupChoice int // 0 = yes, 1 = no

	styles Styles
}

// New creates a new session selector model.
func New(sessions []session.Session) Model {
	return Model{
		sessions:      sessions,
		cursor:        0,
		cleanupChoice: 1, // Default to "No"
		styles:        DefaultStyles(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		if m.showCleanup {
			return m.updateCleanupDialog(msg)
		}
		return m.updateSessionList(msg)
	}

	return m, nil
}

// updateSessionList handles key events in the main session list view.
func (m Model) updateSessionList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.quitting = true
		m.result.Cancelled = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.sessions)-1 {
			m.cursor++
		}
		return m, nil

	case "enter":
		if len(m.sessions) == 0 {
			return m, nil
		}
		selected := m.sessions[m.cursor]
		if !selected.Valid {
			// Show cleanup confirmation dialog
			m.showCleanup = true
			m.cleanupChoice = 1 // Default to "No"
			return m, nil
		}
		// Valid session selected
		m.quitting = true
		m.result.Session = &selected
		return m, tea.Quit
	}

	return m, nil
}

// updateCleanupDialog handles key events in the cleanup confirmation dialog.
func (m Model) updateCleanupDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.showCleanup = false
		return m, nil

	case "left", "h":
		m.cleanupChoice = 0
		return m, nil

	case "right", "l":
		m.cleanupChoice = 1
		return m, nil

	case "y":
		// Shortcut for Yes
		m.cleanupChoice = 0
		return m.confirmCleanup()

	case "n":
		// Shortcut for No
		m.showCleanup = false
		return m, nil

	case "enter":
		return m.confirmCleanup()

	case "tab":
		m.cleanupChoice = (m.cleanupChoice + 1) % 2
		return m, nil
	}

	return m, nil
}

// confirmCleanup processes the cleanup confirmation.
func (m Model) confirmCleanup() (tea.Model, tea.Cmd) {
	if m.cleanupChoice == 0 {
		// User confirmed cleanup
		selected := m.sessions[m.cursor]
		if selected.Path() != "" {
			m.result.CleanupPaths = append(m.result.CleanupPaths, selected.Path())
		}
		// Remove the session from the list
		m.sessions = append(m.sessions[:m.cursor], m.sessions[m.cursor+1:]...)
		if m.cursor >= len(m.sessions) && m.cursor > 0 {
			m.cursor--
		}
	}
	m.showCleanup = false

	// If no sessions left, quit
	if len(m.sessions) == 0 {
		m.quitting = true
		m.result.Cancelled = true
		return m, tea.Quit
	}

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.showCleanup {
		return m.viewCleanupDialog()
	}

	return m.viewSessionList()
}

// viewSessionList renders the main session selection view.
func (m Model) viewSessionList() string {
	var b strings.Builder

	// Title
	b.WriteString(" ")
	b.WriteString(m.styles.Title.Render("Orbital Continue - Select Session"))
	b.WriteString("\n")
	b.WriteString(m.renderSeparator())
	b.WriteString("\n")

	// Sessions
	if len(m.sessions) == 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.Warning.Render("   No sessions found"))
		b.WriteString("\n")
	} else {
		for i, s := range m.sessions {
			b.WriteString(m.renderSession(i, s))
		}
	}

	// Help bar
	b.WriteString("\n")
	b.WriteString(m.renderSeparator())
	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render(" up/down navigate  enter select  q quit"))

	return b.String()
}

// renderSession renders a single session entry.
func (m Model) renderSession(idx int, s session.Session) string {
	var b strings.Builder
	isSelected := idx == m.cursor

	// Cursor and session number
	cursor := "   "
	if isSelected {
		if s.Valid {
			cursor = m.styles.Cursor.Render(" > ")
		} else {
			cursor = m.styles.CursorInvalid.Render(" > ")
		}
	}

	// Session type indicator and name
	var typeIcon string
	if s.Type == session.SessionTypeWorktree {
		typeIcon = "wt "
	} else {
		typeIcon = "   "
	}

	// Style based on validity
	nameStyle := m.styles.SessionValid
	labelStyle := m.styles.Label
	valueStyle := m.styles.Value
	if !s.Valid {
		nameStyle = m.styles.SessionInvalid
		labelStyle = m.styles.SessionInvalid
		valueStyle = m.styles.ValueDim
	}

	// Line 1: Name
	b.WriteString("\n")
	b.WriteString(cursor)
	b.WriteString(nameStyle.Render(typeIcon + s.DisplayName()))
	if s.Type == session.SessionTypeWorktree {
		b.WriteString(labelStyle.Render(" (worktree)"))
	}
	b.WriteString("\n")

	// Line 2: Branch (for worktrees) or Specs
	if branch := s.Branch(); branch != "" {
		b.WriteString("       ")
		b.WriteString(labelStyle.Render("Branch: "))
		b.WriteString(valueStyle.Render(branch))
		b.WriteString("\n")
	}

	// Line 3: Spec files
	if len(s.SpecFiles) > 0 {
		b.WriteString("       ")
		b.WriteString(labelStyle.Render("Specs: "))
		specs := formatSpecs(s.SpecFiles)
		b.WriteString(valueStyle.Render(specs))
		b.WriteString("\n")
	}

	// Line 4: Created time
	b.WriteString("       ")
	b.WriteString(labelStyle.Render("Created: "))
	b.WriteString(valueStyle.Render(formatTimeAgo(s.CreatedAt)))
	b.WriteString("\n")

	// Line 5: Status indicator
	b.WriteString("       ")
	if s.Valid {
		b.WriteString(m.styles.Success.Render("v Valid"))
	} else {
		b.WriteString(m.styles.Warning.Render("! " + s.InvalidReason))
	}
	b.WriteString("\n")

	return b.String()
}

// viewCleanupDialog renders the cleanup confirmation dialog.
func (m Model) viewCleanupDialog() string {
	var b strings.Builder

	if m.cursor >= len(m.sessions) {
		return ""
	}
	s := m.sessions[m.cursor]

	// Title
	b.WriteString(" ")
	b.WriteString(m.styles.DialogTitle.Render("Remove Stale Session?"))
	b.WriteString("\n")
	b.WriteString(m.renderSeparator())
	b.WriteString("\n\n")

	// Explanation
	b.WriteString(" ")
	b.WriteString(m.styles.DialogText.Render("The session \""+s.DisplayName()+"\" cannot be resumed:"))
	b.WriteString("\n")
	b.WriteString(" ")
	b.WriteString(m.styles.Warning.Render(s.InvalidReason))
	b.WriteString("\n\n")

	b.WriteString(" ")
	b.WriteString(m.styles.DialogText.Render("Remove this stale entry from state?"))
	b.WriteString("\n")
	if s.Type == session.SessionTypeWorktree {
		b.WriteString(" ")
		b.WriteString(m.styles.Label.Render("This will delete the worktree-state.json entry."))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Buttons
	b.WriteString(" ")
	if m.cleanupChoice == 0 {
		b.WriteString(m.styles.ButtonActive.Render("Yes, remove"))
	} else {
		b.WriteString(m.styles.ButtonInactive.Render("Yes, remove"))
	}
	b.WriteString("  ")
	if m.cleanupChoice == 1 {
		b.WriteString(m.styles.ButtonActive.Render("No, go back"))
	} else {
		b.WriteString(m.styles.ButtonInactive.Render("No, go back"))
	}
	b.WriteString("\n\n")

	// Help
	b.WriteString(m.renderSeparator())
	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render(" left/right select  y/n quick choice  enter confirm  esc cancel"))

	return b.String()
}

// renderSeparator renders a horizontal line.
func (m Model) renderSeparator() string {
	width := m.width
	if width <= 0 {
		width = 80 // Default width
	}
	return m.styles.Separator.Render(strings.Repeat("-", width))
}

// Result returns the selection result. Call after the model has quit.
func (m Model) Result() Result {
	return m.result
}

// Run executes the selector and returns the result.
func Run(sessions []session.Session) (*Result, error) {
	model := New(sessions)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	result := finalModel.(Model).Result()
	return &result, nil
}

// formatSpecs formats spec file paths for display.
func formatSpecs(specs []string) string {
	if len(specs) == 0 {
		return "(none)"
	}
	if len(specs) == 1 {
		// Truncate long paths
		path := specs[0]
		if len(path) > 40 {
			path = "..." + path[len(path)-37:]
		}
		return path
	}
	return intToString(len(specs)) + " files"
}

// formatTimeAgo formats a time as relative duration.
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return intToString(mins) + " minutes ago"
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return intToString(hours) + " hours ago"
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return intToString(days) + " days ago"
}

// intToString converts an integer to string without strconv.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}
	var result strings.Builder
	for n > 0 {
		result.WriteString(string(rune('0' + n%10)))
		n /= 10
	}
	// Reverse
	s := result.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
