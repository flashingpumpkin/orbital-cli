package dtui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/flashingpumpkin/orbital/internal/daemon"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	groupStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("255"))

	sessionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	statusRunning = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	statusCompleted = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	statusFailed = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	statusStopped = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	chatUserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	chatAssistantStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
)

// View renders the model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.view {
	case ViewManager:
		return m.viewManager()
	case ViewSession:
		return m.viewSession()
	case ViewChat:
		return m.viewChat()
	case ViewNewSession:
		return m.viewNewSession()
	case ViewHelp:
		return m.viewHelp()
	default:
		return m.viewManager()
	}
}

// viewManager renders the manager view.
func (m Model) viewManager() string {
	var b strings.Builder

	// Header
	title := titleStyle.Render("Orbital Session Manager")
	b.WriteString(title + "\n")
	b.WriteString(strings.Repeat("─", min(m.width, 80)) + "\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n")
	}

	// Session list
	items := m.getVisibleItems()
	if len(items) == 0 {
		b.WriteString(dimStyle.Render("No sessions. Press 'n' to start a new session.") + "\n")
	} else {
		for i, item := range items {
			line := m.renderListItem(item, i == m.selectedIdx)
			b.WriteString(line + "\n")
		}
	}

	// Fill remaining space
	contentHeight := strings.Count(b.String(), "\n")
	for i := contentHeight; i < m.height-4; i++ {
		b.WriteString("\n")
	}

	// Help bar
	b.WriteString("\n" + strings.Repeat("─", min(m.width, 80)) + "\n")
	b.WriteString(m.renderManagerHelp())

	return b.String()
}

// renderListItem renders a single list item.
func (m Model) renderListItem(item listItem, selected bool) string {
	if item.isGroup {
		return m.renderGroup(item.group, selected)
	}
	return m.renderSession(item.session, selected)
}

// renderGroup renders a group header.
func (m Model) renderGroup(group string, selected bool) string {
	expanded := m.expandedGroups[group]
	arrow := "▶"
	if expanded {
		arrow = "▼"
	}

	// Count sessions in group
	count := 0
	for _, s := range m.sessions {
		switch group {
		case "running":
			if s.Status == daemon.StatusRunning || s.Status == daemon.StatusMerging {
				count++
			}
		case "completed":
			if s.Status == daemon.StatusCompleted || s.Status == daemon.StatusMerged {
				count++
			}
		case "failed":
			if s.Status == daemon.StatusFailed || s.Status == daemon.StatusConflict {
				count++
			}
		case "stopped":
			if s.Status == daemon.StatusStopped || s.Status == daemon.StatusInterrupted {
				count++
			}
		}
	}

	// Style based on group type
	var style lipgloss.Style
	switch group {
	case "running":
		style = statusRunning
	case "completed":
		style = statusCompleted
	case "failed":
		style = statusFailed
	case "stopped":
		style = statusStopped
	default:
		style = groupStyle
	}

	label := fmt.Sprintf("%s %s (%d)", arrow, capitalizeFirst(group), count)
	line := style.Render(label)

	if selected {
		line = selectedStyle.Render(label)
	}

	return line
}

// renderSession renders a session item.
func (m Model) renderSession(s *daemon.Session, selected bool) string {
	// Get spec file name
	specName := "unknown"
	if len(s.SpecFiles) > 0 {
		specName = filepath.Base(s.SpecFiles[0])
	}

	// Progress bar
	progress := m.renderProgress(s.Iteration, s.MaxIterations)

	// Cost
	cost := fmt.Sprintf("$%.2f", s.TotalCost)

	// Status indicator
	statusIcon := m.renderStatusIcon(s.Status)

	// Time ago
	timeAgo := m.formatTimeAgo(s.StartedAt)

	// Worktree indicator
	worktree := ""
	if s.Worktree != nil {
		worktree = fmt.Sprintf(" [%s]", s.Worktree.Branch)
	}

	line := fmt.Sprintf("  %s %-25s %s %8s %s%s",
		statusIcon,
		truncate(specName, 25),
		progress,
		cost,
		timeAgo,
		worktree,
	)

	if selected {
		return selectedStyle.Render(line)
	}
	return sessionStyle.Render(line)
}

// renderProgress renders a progress bar.
func (m Model) renderProgress(current, max int) string {
	if max <= 0 {
		max = 50
	}

	width := 10
	filled := (current * width) / max
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("●", filled) + strings.Repeat("○", width-filled)
	return fmt.Sprintf("[%d/%d] %s", current, max, bar)
}

// renderStatusIcon renders a status icon.
func (m Model) renderStatusIcon(status daemon.SessionStatus) string {
	switch status {
	case daemon.StatusRunning:
		return statusRunning.Render("●")
	case daemon.StatusMerging:
		return statusRunning.Render("⟳")
	case daemon.StatusCompleted:
		return statusCompleted.Render("✓")
	case daemon.StatusMerged:
		return statusCompleted.Render("⊕")
	case daemon.StatusFailed:
		return statusFailed.Render("✗")
	case daemon.StatusConflict:
		return statusFailed.Render("⚠")
	case daemon.StatusStopped:
		return statusStopped.Render("■")
	case daemon.StatusInterrupted:
		return statusStopped.Render("⊘")
	default:
		return dimStyle.Render("?")
	}
}

// formatTimeAgo formats a time as a human-readable string.
func (m Model) formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// renderManagerHelp renders the help bar for manager view.
func (m Model) renderManagerHelp() string {
	items := []string{
		"[Enter] View",
		"[n] New",
		"[s] Stop",
		"[r] Resume",
		"[m] Merge",
		"[c] Chat",
		"[?] Help",
		"[q] Quit",
	}
	return helpStyle.Render(strings.Join(items, "  "))
}

// viewSession renders the session detail view.
func (m Model) viewSession() string {
	var b strings.Builder

	if m.currentSession == nil {
		return "No session selected"
	}

	s := m.currentSession

	// Header
	specName := "unknown"
	if len(s.SpecFiles) > 0 {
		specName = filepath.Base(s.SpecFiles[0])
	}
	title := titleStyle.Render(specName)
	b.WriteString(title + "\n")

	// Status line
	statusLine := fmt.Sprintf("Status: %s   Iteration: %d/%d   Cost: $%.2f",
		m.renderStatusIcon(s.Status)+" "+string(s.Status),
		s.Iteration,
		s.MaxIterations,
		s.TotalCost,
	)
	if s.Worktree != nil {
		statusLine += fmt.Sprintf("   Branch: %s", s.Worktree.Branch)
	}
	b.WriteString(dimStyle.Render(statusLine) + "\n")
	b.WriteString(strings.Repeat("─", min(m.width, 80)) + "\n\n")

	// Output
	if len(m.outputBuffer) == 0 {
		b.WriteString(dimStyle.Render("Waiting for output...") + "\n")
		if m.loading {
			b.WriteString(m.spinner.View() + "\n")
		}
	} else {
		// Render output lines
		outputHeight := m.height - 10
		start := 0
		if len(m.outputBuffer) > outputHeight {
			start = len(m.outputBuffer) - outputHeight
		}

		for i := start; i < len(m.outputBuffer); i++ {
			msg := m.outputBuffer[i]
			line := m.renderOutputMessage(msg)
			b.WriteString(line + "\n")
		}
	}

	// Fill space
	contentHeight := strings.Count(b.String(), "\n")
	for i := contentHeight; i < m.height-4; i++ {
		b.WriteString("\n")
	}

	// Follow indicator
	followIndicator := ""
	if m.following {
		followIndicator = " [following]"
	}

	// Help bar
	b.WriteString("\n" + strings.Repeat("─", min(m.width, 80)) + "\n")
	help := fmt.Sprintf("[b] Back  [s] Stop  [m] Merge  [c] Chat  [f] Follow%s", followIndicator)
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

// renderOutputMessage renders an output message.
func (m Model) renderOutputMessage(msg daemon.OutputMsg) string {
	switch msg.Type {
	case "text":
		return msg.Content
	case "tool":
		return dimStyle.Render(msg.Content)
	case "status":
		return statusRunning.Render("● " + msg.Content)
	case "error":
		return errorStyle.Render("✗ " + msg.Content)
	default:
		return msg.Content
	}
}

// viewChat renders the chat view.
func (m Model) viewChat() string {
	var b strings.Builder

	if m.currentSession == nil {
		return "No session selected"
	}

	specName := "unknown"
	if len(m.currentSession.SpecFiles) > 0 {
		specName = filepath.Base(m.currentSession.SpecFiles[0])
	}

	// Header
	title := titleStyle.Render("Chat: " + specName)
	b.WriteString(title + "\n")

	// Context info
	context := fmt.Sprintf("Context: %s", strings.Join(m.currentSession.SpecFiles, ", "))
	b.WriteString(dimStyle.Render(context) + "\n")
	b.WriteString(strings.Repeat("─", min(m.width, 80)) + "\n\n")

	// Chat history
	if len(m.chatHistory) == 0 {
		b.WriteString(dimStyle.Render("Start a conversation about this session's specs...") + "\n\n")
	} else {
		for _, msg := range m.chatHistory {
			if msg.Role == "user" {
				b.WriteString(chatUserStyle.Render("You: ") + msg.Content + "\n\n")
			} else {
				b.WriteString(chatAssistantStyle.Render("Claude: ") + msg.Content + "\n\n")
			}
		}
	}

	// Loading indicator
	if m.loading {
		b.WriteString(m.spinner.View() + " Thinking...\n")
	}

	// Fill space
	contentHeight := strings.Count(b.String(), "\n")
	for i := contentHeight; i < m.height-6; i++ {
		b.WriteString("\n")
	}

	// Input
	b.WriteString(strings.Repeat("─", min(m.width, 80)) + "\n")
	b.WriteString("> " + m.chatInput.View() + "\n")
	b.WriteString(helpStyle.Render("[Enter] Send  [Esc] Back"))

	return b.String()
}

// viewNewSession renders the new session dialog.
func (m Model) viewNewSession() string {
	var b strings.Builder

	title := titleStyle.Render("New Session")
	b.WriteString(title + "\n")
	b.WriteString(strings.Repeat("─", min(m.width, 80)) + "\n\n")

	b.WriteString("Spec file:\n")
	b.WriteString("> " + m.newSessionInput.View() + "\n\n")

	// Options
	worktreeCheck := "[ ]"
	if m.newSessionOpts.worktree {
		worktreeCheck = "[x]"
	}
	b.WriteString(fmt.Sprintf("Options:\n  %s Worktree isolation (Tab to toggle)\n", worktreeCheck))
	b.WriteString(fmt.Sprintf("  Budget: $%.2f\n", m.newSessionOpts.budget))

	// Fill space
	contentHeight := strings.Count(b.String(), "\n")
	for i := contentHeight; i < m.height-4; i++ {
		b.WriteString("\n")
	}

	b.WriteString("\n" + strings.Repeat("─", min(m.width, 80)) + "\n")
	b.WriteString(helpStyle.Render("[Enter] Start  [Tab] Toggle worktree  [Esc] Cancel"))

	return b.String()
}

// viewHelp renders the help view.
func (m Model) viewHelp() string {
	var b strings.Builder

	title := titleStyle.Render("Help")
	b.WriteString(title + "\n")
	b.WriteString(strings.Repeat("─", min(m.width, 80)) + "\n\n")

	help := `Manager View:
  ↑/k, ↓/j     Navigate sessions
  ←/h, →/l     Collapse/expand groups
  Enter        View session details
  n            Start new session
  s            Stop selected session
  r            Resume interrupted session
  m            Merge worktree (if applicable)
  c            Open chat for session
  q            Quit

Session View:
  ↑/k, ↓/j     Scroll output
  f            Follow output (auto-scroll)
  s            Stop session
  m            Merge worktree
  c            Open chat
  b/Esc        Back to manager

Chat View:
  Enter        Send message
  Esc          Back to session view

General:
  ?            Toggle help
  Ctrl+C       Force quit
`
	b.WriteString(help)

	// Fill space
	contentHeight := strings.Count(b.String(), "\n")
	for i := contentHeight; i < m.height-4; i++ {
		b.WriteString("\n")
	}

	b.WriteString("\n" + strings.Repeat("─", min(m.width, 80)) + "\n")
	b.WriteString(helpStyle.Render("Press any key to close"))

	return b.String()
}

// truncate truncates a string to the given length.
func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

// min returns the minimum of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
