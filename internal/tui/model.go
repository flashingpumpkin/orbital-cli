package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flashingpumpkin/orbital/internal/tasks"
)

// Task is an alias to the shared tasks.Task type for TUI use.
type Task = tasks.Task

// SessionInfo contains the file paths for the current session.
type SessionInfo struct {
	SpecFiles   []string
	NotesFile   string
	StateFile   string
	ContextFile string
}

// ProgressInfo contains iteration and cost metrics.
type ProgressInfo struct {
	Iteration    int
	MaxIteration int
	StepName     string
	StepPosition int
	StepTotal    int
	GateRetries  int
	MaxRetries   int
	TokensIn     int
	TokensOut    int
	Cost         float64
	Budget       float64
}

// StatsMsg is a message containing updated token and cost statistics.
type StatsMsg struct {
	TokensIn  int
	TokensOut int
	Cost      float64
}

// Model is the main bubbletea model for the orbit TUI.
type Model struct {
	// Layout
	layout Layout

	// Content
	outputLines []string
	scrollPos   int
	tasks       []Task
	progress    ProgressInfo
	session     SessionInfo
	worktree    WorktreeInfo

	// Styles
	styles Styles

	// State
	ready bool
}

// Styles contains all lipgloss styles for the UI.
type Styles struct {
	Border          lipgloss.Style
	BorderDim       lipgloss.Style
	Label           lipgloss.Style
	Value           lipgloss.Style
	Warning         lipgloss.Style
	Error           lipgloss.Style
	Success         lipgloss.Style
	TaskPending     lipgloss.Style
	TaskInProgress  lipgloss.Style
	TaskComplete    lipgloss.Style
	ScrollArea      lipgloss.Style
	TooSmallMessage lipgloss.Style
	WorktreeLabel   lipgloss.Style
	WorktreeValue   lipgloss.Style
}

// NewModel creates a new TUI model.
func NewModel() Model {
	return Model{
		outputLines: make([]string, 0),
		tasks:       make([]Task, 0),
		styles:      defaultStyles(),
		progress: ProgressInfo{
			Iteration:    1,
			MaxIteration: 50,
		},
	}
}

// defaultStyles returns the default style configuration.
func defaultStyles() Styles {
	return Styles{
		Border:          lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		BorderDim:       lipgloss.NewStyle().Foreground(lipgloss.Color("236")),
		Label:           lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Value:           lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Warning:         lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		Error:           lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		Success:         lipgloss.NewStyle().Foreground(lipgloss.Color("82")),
		TaskPending:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		TaskInProgress:  lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		TaskComplete:    lipgloss.NewStyle().Foreground(lipgloss.Color("82")),
		ScrollArea:      lipgloss.NewStyle(),
		TooSmallMessage: lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		WorktreeLabel:   lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true),
		WorktreeValue:   lipgloss.NewStyle().Foreground(lipgloss.Color("183")),
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
		m.layout = CalculateLayout(msg.Width, msg.Height, len(m.tasks), m.worktree.Path != "")
		m.ready = true
		return m, nil

	case StatsMsg:
		m.progress.TokensIn = msg.TokensIn
		m.progress.TokensOut = msg.TokensOut
		m.progress.Cost = msg.Cost
		return m, nil

	case OutputLineMsg:
		m.outputLines = append(m.outputLines, string(msg))
		return m, nil

	case TasksMsg:
		m.tasks = msg
		if m.ready {
			m.layout = CalculateLayout(m.layout.Width, m.layout.Height, len(m.tasks), m.worktree.Path != "")
		}
		return m, nil

	case ProgressMsg:
		m.progress = ProgressInfo(msg)
		return m, nil

	case SessionMsg:
		m.session = SessionInfo(msg)
		return m, nil

	case WorktreeMsg:
		m.worktree = WorktreeInfo(msg)
		if m.ready {
			m.layout = CalculateLayout(m.layout.Width, m.layout.Height, len(m.tasks), m.worktree.Path != "")
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.layout.TooSmall {
		return m.renderTooSmall()
	}

	return m.renderFull()
}

// renderTooSmall renders the "terminal too small" message.
func (m Model) renderTooSmall() string {
	return m.styles.TooSmallMessage.Render(m.layout.TooSmallMessage)
}

// renderFull renders the complete UI with all panels.
func (m Model) renderFull() string {
	var sections []string

	// Scrolling output area
	sections = append(sections, m.renderScrollArea())

	// Horizontal separator
	sections = append(sections, m.renderSeparator())

	// Task panel (if tasks exist)
	if m.layout.TaskPanelHeight > 0 {
		sections = append(sections, m.renderTaskPanel())
		sections = append(sections, m.renderSeparator())
	}

	// Progress panel
	sections = append(sections, m.renderProgressPanel())

	// Horizontal separator
	sections = append(sections, m.renderSeparator())

	// Session info panel
	sections = append(sections, m.renderSessionPanel())

	// Worktree panel (if worktree mode is active)
	if m.layout.WorktreePanelHeight > 0 {
		sections = append(sections, m.renderSeparator())
		sections = append(sections, m.renderWorktreePanel())
	}

	return strings.Join(sections, "\n")
}

// renderScrollArea renders the scrolling output region.
func (m Model) renderScrollArea() string {
	height := m.layout.ScrollAreaHeight
	width := m.layout.ContentWidth()

	// Get visible lines
	startIdx := 0
	if len(m.outputLines) > height {
		startIdx = len(m.outputLines) - height
	}

	var lines []string
	for i := startIdx; i < len(m.outputLines) && len(lines) < height; i++ {
		line := m.outputLines[i]
		// Truncate long lines
		if len(line) > width {
			line = line[:width-3] + "..."
		}
		lines = append(lines, line)
	}

	// Pad with empty lines if needed
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// renderSeparator renders a horizontal separator line.
func (m Model) renderSeparator() string {
	return m.styles.BorderDim.Render(strings.Repeat("─", m.layout.Width))
}

// renderTaskPanel renders the task list panel.
func (m Model) renderTaskPanel() string {
	var lines []string

	// Header
	header := m.styles.Label.Render("  Tasks")
	if m.layout.HasTaskOverflow(len(m.tasks)) {
		header += m.styles.Label.Render(" (scroll)")
	}
	lines = append(lines, header)

	// Tasks
	visible := m.layout.TasksVisible()
	for i := 0; i < visible && i < len(m.tasks); i++ {
		task := m.tasks[i]
		lines = append(lines, m.renderTask(task))
	}

	return strings.Join(lines, "\n")
}

// renderTask renders a single task line.
func (m Model) renderTask(task Task) string {
	var icon string
	var style lipgloss.Style

	switch task.Status {
	case "completed":
		icon = "✓"
		style = m.styles.TaskComplete
	case "in_progress":
		icon = "→"
		style = m.styles.TaskInProgress
	default:
		icon = "○"
		style = m.styles.TaskPending
	}

	content := task.Content
	maxLen := m.layout.ContentWidth() - 6 // icon + spacing
	if len(content) > maxLen {
		content = content[:maxLen-3] + "..."
	}

	return style.Render("  " + icon + " " + content)
}

// renderProgressPanel renders the progress and metrics panel.
func (m Model) renderProgressPanel() string {
	p := m.progress

	// Line 1: Iteration and step info
	iterStr := m.formatIteration(p.Iteration, p.MaxIteration)
	stepStr := m.formatStep(p.StepName, p.StepPosition, p.StepTotal)
	gateStr := ""
	if p.GateRetries > 0 || p.MaxRetries > 0 {
		gateStr = m.formatGateRetries(p.GateRetries, p.MaxRetries)
	}

	line1Parts := []string{iterStr}
	if stepStr != "" {
		line1Parts = append(line1Parts, stepStr)
	}
	if gateStr != "" {
		line1Parts = append(line1Parts, gateStr)
	}
	line1 := "  " + strings.Join(line1Parts, " │ ")

	// Line 2: Tokens and cost
	tokensStr := m.formatTokens(p.TokensIn, p.TokensOut)
	costStr := m.formatCost(p.Cost, p.Budget)
	line2 := "  " + tokensStr + " │ " + costStr

	return line1 + "\n" + line2
}

// formatIteration formats the iteration counter with optional warning colour.
func (m Model) formatIteration(current, max int) string {
	label := m.styles.Label.Render("Iteration ")
	ratio := float64(current) / float64(max)

	var value string
	if ratio > 0.8 {
		value = m.styles.Warning.Render(formatFraction(current, max))
	} else {
		value = m.styles.Value.Render(formatFraction(current, max))
	}

	return label + value
}

// formatStep formats the step name and position.
func (m Model) formatStep(name string, pos, total int) string {
	if name == "" {
		return ""
	}
	label := m.styles.Label.Render("Step: ")
	value := m.styles.Value.Render(name)
	if total > 0 {
		value += m.styles.Label.Render(" (") + m.styles.Value.Render(formatFraction(pos, total)) + m.styles.Label.Render(")")
	}
	return label + value
}

// formatGateRetries formats the gate retry count.
func (m Model) formatGateRetries(retries, max int) string {
	label := m.styles.Label.Render("Gate retries: ")
	return label + m.styles.Value.Render(formatFraction(retries, max))
}

// formatTokens formats token counts with thousands separator.
func (m Model) formatTokens(in, out int) string {
	label := m.styles.Label.Render("Tokens: ")
	inStr := m.styles.Value.Render(formatNumber(in))
	outStr := m.styles.Value.Render(formatNumber(out))
	return label + inStr + m.styles.Label.Render(" in / ") + outStr + m.styles.Label.Render(" out")
}

// formatCost formats cost with optional warning colour.
func (m Model) formatCost(cost, budget float64) string {
	label := m.styles.Label.Render("Cost: ")
	ratio := cost / budget

	var costStr string
	if ratio > 0.8 {
		costStr = m.styles.Warning.Render(formatCurrency(cost))
	} else {
		costStr = m.styles.Value.Render(formatCurrency(cost))
	}

	budgetStr := m.styles.Label.Render(" / ") + m.styles.Value.Render(formatCurrency(budget))
	return label + costStr + budgetStr
}

// renderSessionPanel renders the session info panel.
func (m Model) renderSessionPanel() string {
	s := m.session

	// Line 1: Spec file(s)
	specStr := m.formatPaths("Spec", s.SpecFiles)

	// Line 2: Notes and state files
	var line2Parts []string
	if s.NotesFile != "" {
		line2Parts = append(line2Parts, m.formatPath("Notes", s.NotesFile))
	}
	if s.StateFile != "" {
		line2Parts = append(line2Parts, m.formatPath("State", s.StateFile))
	}
	if s.ContextFile != "" {
		line2Parts = append(line2Parts, m.formatPath("Context", s.ContextFile))
	}

	line1 := "  " + specStr
	line2 := "  " + strings.Join(line2Parts, " │ ")

	return line1 + "\n" + line2
}

// renderWorktreePanel renders the worktree info panel.
func (m Model) renderWorktreePanel() string {
	w := m.worktree

	// Icon and label
	icon := m.styles.WorktreeLabel.Render("⎇")

	// If name is available, show it prominently
	var nameStr string
	if w.Name != "" {
		nameStr = m.styles.WorktreeLabel.Render(" Worktree: ") + m.styles.WorktreeValue.Render(w.Name)
	} else {
		// Fallback to path if no name
		path := w.Path
		maxPathLen := 40
		if len(path) > maxPathLen {
			path = "..." + path[len(path)-maxPathLen+3:]
		}
		nameStr = m.styles.WorktreeLabel.Render(" Worktree: ") + m.styles.WorktreeValue.Render(path)
	}

	// Branch
	branchLabel := m.styles.Label.Render(" │ Branch: ")
	branchStr := m.styles.WorktreeValue.Render(w.Branch)

	return "  " + icon + nameStr + branchLabel + branchStr
}

// formatPath formats a single file path with truncation.
func (m Model) formatPath(label, path string) string {
	labelStr := m.styles.Label.Render(label + ": ")
	maxLen := 40
	if len(path) > maxLen {
		path = "..." + path[len(path)-maxLen+3:]
	}
	return labelStr + m.styles.Value.Render(path)
}

// formatPaths formats multiple file paths.
func (m Model) formatPaths(label string, paths []string) string {
	labelStr := m.styles.Label.Render(label + ": ")
	if len(paths) == 0 {
		return labelStr + m.styles.Value.Render("(none)")
	}
	if len(paths) == 1 {
		path := paths[0]
		maxLen := 60
		if len(path) > maxLen {
			path = "..." + path[len(path)-maxLen+3:]
		}
		return labelStr + m.styles.Value.Render(path)
	}
	return labelStr + m.styles.Value.Render(formatNumber(len(paths))+" files")
}

// Helper functions for formatting

func formatFraction(a, b int) string {
	return intToString(a) + "/" + intToString(b)
}

func formatNumber(n int) string {
	// Simple thousands separator
	s := intToString(n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(r)
	}
	return result.String()
}

func formatCurrency(amount float64) string {
	// Format as $X.XX with proper rounding
	// Add 0.005 to handle floating point precision issues
	totalCents := int(amount*100 + 0.5)
	whole := totalCents / 100
	cents := totalCents % 100
	return "$" + formatNumber(whole) + "." + padLeft(intToString(cents), 2, '0')
}

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

func padLeft(s string, length int, pad rune) string {
	for len(s) < length {
		s = string(pad) + s
	}
	return s
}

// SetProgress updates the progress information.
func (m *Model) SetProgress(p ProgressInfo) {
	m.progress = p
}

// SetSession updates the session information.
func (m *Model) SetSession(s SessionInfo) {
	m.session = s
}

// SetTasks updates the task list.
func (m *Model) SetTasks(tasks []Task) {
	m.tasks = tasks
	// Recalculate layout with new task count
	if m.ready {
		m.layout = CalculateLayout(m.layout.Width, m.layout.Height, len(tasks), m.worktree.Path != "")
	}
}

// SetWorktree updates the worktree information.
func (m *Model) SetWorktree(w WorktreeInfo) {
	m.worktree = w
	// Recalculate layout when worktree info changes
	if m.ready {
		m.layout = CalculateLayout(m.layout.Width, m.layout.Height, len(m.tasks), w.Path != "")
	}
}

// AppendOutput adds a line to the output buffer.
func (m *Model) AppendOutput(line string) {
	m.outputLines = append(m.outputLines, line)
}

// ClearOutput clears the output buffer.
func (m *Model) ClearOutput() {
	m.outputLines = m.outputLines[:0]
}
