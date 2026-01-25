package selector

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flashingpumpkin/orbital/internal/session"
	"github.com/flashingpumpkin/orbital/internal/util"
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
	width := m.width
	if width <= 0 {
		width = 80
	}

	// Top border
	b.WriteString(RenderTopBorder(width, m.styles.Border))
	b.WriteString("\n")

	// Header with brand
	b.WriteString(m.renderBorderedLine("  "+m.styles.Brand.Render("◆ ORBITAL CONTINUE"), width))
	b.WriteString("\n")
	b.WriteString(RenderMidBorder(width, m.styles.Border))
	b.WriteString("\n")

	// Sessions
	if len(m.sessions) == 0 {
		b.WriteString(m.renderBorderedLine("", width))
		b.WriteString("\n")
		b.WriteString(m.renderBorderedLine("   "+m.styles.Warning.Render("No sessions found"), width))
		b.WriteString("\n")
		b.WriteString(m.renderBorderedLine("", width))
		b.WriteString("\n")
	} else {
		for i, s := range m.sessions {
			b.WriteString(m.renderSession(i, s))
		}
	}

	// Bottom border
	b.WriteString(RenderMidBorder(width, m.styles.Border))
	b.WriteString("\n")

	// Help bar (outside frame)
	b.WriteString("  ")
	b.WriteString(m.styles.HelpKey.Render("↑/↓"))
	b.WriteString(m.styles.Help.Render(" navigate  "))
	b.WriteString(m.styles.HelpKey.Render("enter"))
	b.WriteString(m.styles.Help.Render(" select  "))
	b.WriteString(m.styles.HelpKey.Render("q"))
	b.WriteString(m.styles.Help.Render(" quit"))

	// Final bottom border
	b.WriteString("\n")
	b.WriteString(RenderBottomBorder(width, m.styles.Border))

	return b.String()
}

// renderBorderedLine renders a line with vertical borders.
func (m Model) renderBorderedLine(content string, width int) string {
	border := m.styles.Border.Render(boxVertical)
	contentWidth := width - 2 // Account for borders
	// Simple padding calculation (not fully ANSI-aware for brevity)
	padding := contentWidth - len(content)
	if padding < 0 {
		padding = 0
	}
	return border + content + strings.Repeat(" ", padding) + border
}

// renderSession renders a single session entry.
func (m Model) renderSession(idx int, s session.Session) string {
	var b strings.Builder
	isSelected := idx == m.cursor
	width := m.width
	if width <= 0 {
		width = 80
	}

	// Cursor and session number
	cursor := "   "
	if isSelected {
		if s.Valid {
			cursor = m.styles.Cursor.Render(" > ")
		} else {
			cursor = m.styles.CursorInvalid.Render(" > ")
		}
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

	// Status indicator on same line as name
	var statusStr string
	if s.Valid {
		statusStr = m.styles.Success.Render("Valid")
	} else {
		statusStr = m.styles.Warning.Render("Stale")
	}

	// Line 1: Empty line for spacing
	b.WriteString(m.renderBorderedLine("", width))
	b.WriteString("\n")

	// Line 2: Name with status on right
	nameLine := cursor + nameStyle.Render(s.DisplayName())
	// Right-align status
	nameLen := 3 + len(s.DisplayName()) // cursor + name
	statusLen := 5                          // "Valid" or "Stale"
	padding := width - 2 - nameLen - statusLen - 2
	if padding < 1 {
		padding = 1
	}
	b.WriteString(m.styles.Border.Render(boxVertical) + nameLine + strings.Repeat(" ", padding) + statusStr + " " + m.styles.Border.Render(boxVertical))
	b.WriteString("\n")

	// Line 3: Spec files
	if len(s.SpecFiles) > 0 {
		specs := formatSpecs(s.SpecFiles)
		specLine := "       " + labelStyle.Render("Specs: ") + valueStyle.Render(specs)
		b.WriteString(m.renderBorderedLine(specLine, width))
		b.WriteString("\n")
	}

	// Line 5: Created time
	createdLine := "       " + labelStyle.Render("Created: ") + valueStyle.Render(formatTimeAgo(s.CreatedAt))
	b.WriteString(m.renderBorderedLine(createdLine, width))
	b.WriteString("\n")

	// Line 6: Invalid reason if applicable
	if !s.Valid {
		reasonLine := "       " + m.styles.Warning.Render("! "+s.InvalidReason)
		b.WriteString(m.renderBorderedLine(reasonLine, width))
		b.WriteString("\n")
	}

	return b.String()
}

// viewCleanupDialog renders the cleanup confirmation dialog.
func (m Model) viewCleanupDialog() string {
	var b strings.Builder
	width := m.width
	if width <= 0 {
		width = 80
	}

	if m.cursor >= len(m.sessions) {
		return ""
	}
	s := m.sessions[m.cursor]

	// Top border
	b.WriteString(RenderTopBorder(width, m.styles.Border))
	b.WriteString("\n")

	// Title
	b.WriteString(m.renderBorderedLine("  "+m.styles.DialogTitle.Render("Remove Stale Session?"), width))
	b.WriteString("\n")
	b.WriteString(RenderMidBorder(width, m.styles.Border))
	b.WriteString("\n")

	// Empty line
	b.WriteString(m.renderBorderedLine("", width))
	b.WriteString("\n")

	// Explanation
	b.WriteString(m.renderBorderedLine("  "+m.styles.DialogText.Render("The session \""+s.DisplayName()+"\" cannot be resumed:"), width))
	b.WriteString("\n")
	b.WriteString(m.renderBorderedLine("  "+m.styles.Warning.Render(s.InvalidReason), width))
	b.WriteString("\n")

	b.WriteString(m.renderBorderedLine("", width))
	b.WriteString("\n")

	b.WriteString(m.renderBorderedLine("  "+m.styles.DialogText.Render("Remove this stale entry from state?"), width))
	b.WriteString("\n")
	b.WriteString(m.renderBorderedLine("", width))
	b.WriteString("\n")

	// Buttons
	var buttonLine string
	if m.cleanupChoice == 0 {
		buttonLine = "  " + m.styles.ButtonActive.Render("Yes, remove") + "  " + m.styles.ButtonInactive.Render("No, go back")
	} else {
		buttonLine = "  " + m.styles.ButtonInactive.Render("Yes, remove") + "  " + m.styles.ButtonActive.Render("No, go back")
	}
	b.WriteString(m.renderBorderedLine(buttonLine, width))
	b.WriteString("\n")

	b.WriteString(m.renderBorderedLine("", width))
	b.WriteString("\n")

	// Bottom border
	b.WriteString(RenderMidBorder(width, m.styles.Border))
	b.WriteString("\n")

	// Help bar (outside frame)
	b.WriteString("  ")
	b.WriteString(m.styles.HelpKey.Render("←/→"))
	b.WriteString(m.styles.Help.Render(" select  "))
	b.WriteString(m.styles.HelpKey.Render("y/n"))
	b.WriteString(m.styles.Help.Render(" quick choice  "))
	b.WriteString(m.styles.HelpKey.Render("enter"))
	b.WriteString(m.styles.Help.Render(" confirm  "))
	b.WriteString(m.styles.HelpKey.Render("esc"))
	b.WriteString(m.styles.Help.Render(" cancel"))

	// Final bottom border
	b.WriteString("\n")
	b.WriteString(RenderBottomBorder(width, m.styles.Border))

	return b.String()
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
	return util.IntToString(len(specs)) + " files"
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
		return util.IntToString(mins) + " minutes ago"
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return util.IntToString(hours) + " hours ago"
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return util.IntToString(days) + " days ago"
}

