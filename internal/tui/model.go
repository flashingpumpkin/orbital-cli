package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/flashingpumpkin/orbital/internal/tasks"
)

// Task is an alias to the shared tasks.Task type for TUI use.
type Task = tasks.Task

// TabType represents the type of content in a tab.
type TabType int

const (
	// TabOutput is the streaming output tab.
	TabOutput TabType = iota
	// TabFile is a file content tab.
	TabFile
)

// Tab represents a single tab in the tab bar.
type Tab struct {
	Name     string  // Display name for the tab
	Type     TabType // Type of tab content
	FilePath string  // Path to file (for TabFile type)
}

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
	tasks       []Task
	progress    ProgressInfo
	session     SessionInfo
	worktree    WorktreeInfo

	// Tabs
	tabs         []Tab             // List of tabs
	activeTab    int               // Index of active tab
	fileContents map[string]string // Cached file contents by path
	fileScroll   map[string]int    // Scroll offset per file

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
	TabActive       lipgloss.Style
	TabInactive     lipgloss.Style
	TabBar          lipgloss.Style
}

// NewModel creates a new TUI model.
func NewModel() Model {
	return Model{
		outputLines:  make([]string, 0),
		tasks:        make([]Task, 0),
		tabs:         []Tab{{Name: "Output", Type: TabOutput}},
		activeTab:    0,
		fileContents: make(map[string]string),
		fileScroll:   make(map[string]int),
		styles:       defaultStyles(),
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
		TabActive:       lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("240")).Bold(true).Padding(0, 1),
		TabInactive:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1),
		TabBar:          lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// FileContentMsg contains loaded file content.
type FileContentMsg struct {
	Path    string
	Content string
	Error   error
}

// loadFileCmd creates a command to load file content.
func loadFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		content, err := os.ReadFile(path)
		if err != nil {
			return FileContentMsg{Path: path, Error: err}
		}
		return FileContentMsg{Path: path, Content: string(content)}
	}
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
		m.tabs = m.buildTabs()
		// Clamp activeTab to valid range if tabs changed
		if m.activeTab >= len(m.tabs) {
			m.activeTab = 0
		}
		return m, nil

	case WorktreeMsg:
		m.worktree = WorktreeInfo(msg)
		if m.ready {
			m.layout = CalculateLayout(m.layout.Width, m.layout.Height, len(m.tasks), m.worktree.Path != "")
		}
		return m, nil

	case FileContentMsg:
		if msg.Error != nil {
			m.fileContents[msg.Path] = "Error loading file: " + msg.Error.Error()
		} else {
			m.fileContents[msg.Path] = msg.Content
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "left", "h":
			return m.prevTab()
		case "right", "l":
			return m.nextTab()
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx < len(m.tabs) {
				return m.switchToTab(idx)
			}
		case "tab":
			return m.nextTab()
		case "shift+tab":
			return m.prevTab()
		case "up", "k":
			return m.scrollUp()
		case "down", "j":
			return m.scrollDown()
		case "pgup":
			return m.scrollPageUp()
		case "pgdown":
			return m.scrollPageDown()
		case "r":
			return m.reloadCurrentFile()
		}

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Check if click is in tab bar area (first row)
			if msg.Y == 0 {
				return m.handleTabClick(msg.X)
			}
		}
	}

	return m, nil
}

// buildTabs creates the tab list based on session info.
func (m Model) buildTabs() []Tab {
	tabs := []Tab{{Name: "Output", Type: TabOutput}}

	// Add spec files
	for _, path := range m.session.SpecFiles {
		tabs = append(tabs, Tab{
			Name:     "Spec: " + filepath.Base(path),
			Type:     TabFile,
			FilePath: path,
		})
	}

	// Add notes file
	if m.session.NotesFile != "" {
		tabs = append(tabs, Tab{
			Name:     "Notes",
			Type:     TabFile,
			FilePath: m.session.NotesFile,
		})
	}

	// Add context files
	if m.session.ContextFile != "" {
		for _, path := range strings.Split(m.session.ContextFile, ", ") {
			path = strings.TrimSpace(path)
			if path != "" {
				tabs = append(tabs, Tab{
					Name:     "Ctx: " + filepath.Base(path),
					Type:     TabFile,
					FilePath: path,
				})
			}
		}
	}

	return tabs
}

// prevTab switches to the previous tab.
func (m Model) prevTab() (tea.Model, tea.Cmd) {
	if m.activeTab > 0 {
		return m.switchToTab(m.activeTab - 1)
	}
	return m, nil
}

// nextTab switches to the next tab.
func (m Model) nextTab() (tea.Model, tea.Cmd) {
	if m.activeTab < len(m.tabs)-1 {
		return m.switchToTab(m.activeTab + 1)
	}
	return m, nil
}

// switchToTab switches to a specific tab by index.
func (m Model) switchToTab(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.tabs) {
		return m, nil
	}

	m.activeTab = idx
	tab := m.tabs[idx]

	// If it's a file tab and we haven't loaded the content yet, load it
	if tab.Type == TabFile && tab.FilePath != "" {
		if _, ok := m.fileContents[tab.FilePath]; !ok {
			return m, loadFileCmd(tab.FilePath)
		}
	}

	return m, nil
}

// handleTabClick handles a mouse click on the tab bar.
func (m Model) handleTabClick(x int) (tea.Model, tea.Cmd) {
	// Calculate tab positions based on rendered width (must match renderTabBar)
	currentX := 0
	for i, tab := range m.tabs {
		// Tab name with number prefix (must match renderTabBar logic)
		name := tab.Name
		if i < 9 {
			name = intToString(i+1) + ":" + name
		}
		// Tab width is name length + 2 (for padding from style) + 1 (for separator)
		tabWidth := ansi.StringWidth(name) + 3
		if x >= currentX && x < currentX+tabWidth {
			return m.switchToTab(i)
		}
		currentX += tabWidth
	}
	return m, nil
}

// scrollUp scrolls the current file tab up.
func (m Model) scrollUp() (tea.Model, tea.Cmd) {
	if m.activeTab == 0 || len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		if offset, ok := m.fileScroll[tab.FilePath]; ok && offset > 0 {
			m.fileScroll[tab.FilePath] = offset - 1
		}
	}
	return m, nil
}

// scrollDown scrolls the current file tab down.
func (m Model) scrollDown() (tea.Model, tea.Cmd) {
	if m.activeTab == 0 || len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		content, ok := m.fileContents[tab.FilePath]
		if !ok {
			return m, nil
		}
		lines := strings.Split(content, "\n")
		maxOffset := len(lines) - m.layout.ScrollAreaHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		offset := m.fileScroll[tab.FilePath]
		if offset < maxOffset {
			m.fileScroll[tab.FilePath] = offset + 1
		}
	}
	return m, nil
}

// scrollPageUp scrolls the current file tab up by a page.
func (m Model) scrollPageUp() (tea.Model, tea.Cmd) {
	if m.activeTab == 0 || len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		offset := m.fileScroll[tab.FilePath]
		newOffset := offset - m.layout.ScrollAreaHeight
		if newOffset < 0 {
			newOffset = 0
		}
		m.fileScroll[tab.FilePath] = newOffset
	}
	return m, nil
}

// scrollPageDown scrolls the current file tab down by a page.
func (m Model) scrollPageDown() (tea.Model, tea.Cmd) {
	if m.activeTab == 0 || len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		content, ok := m.fileContents[tab.FilePath]
		if !ok {
			return m, nil
		}
		lines := strings.Split(content, "\n")
		maxOffset := len(lines) - m.layout.ScrollAreaHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		offset := m.fileScroll[tab.FilePath]
		newOffset := offset + m.layout.ScrollAreaHeight
		if newOffset > maxOffset {
			newOffset = maxOffset
		}
		m.fileScroll[tab.FilePath] = newOffset
	}
	return m, nil
}

// reloadCurrentFile reloads the content of the current file tab.
func (m Model) reloadCurrentFile() (tea.Model, tea.Cmd) {
	if m.activeTab == 0 || len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		// Clear cached content to trigger reload
		delete(m.fileContents, tab.FilePath)
		return m, loadFileCmd(tab.FilePath)
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

	// Tab bar at the top
	sections = append(sections, m.renderTabBar())
	sections = append(sections, m.renderSeparator())

	// Main content area (output or file content)
	sections = append(sections, m.renderMainContent())

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

// renderTabBar renders the tab bar with all tabs.
func (m Model) renderTabBar() string {
	var tabs []string

	for i, tab := range m.tabs {
		var style lipgloss.Style
		if i == m.activeTab {
			style = m.styles.TabActive
		} else {
			style = m.styles.TabInactive
		}

		// Add number hint for keyboard navigation
		name := tab.Name
		if i < 9 {
			name = intToString(i+1) + ":" + name
		}

		tabs = append(tabs, style.Render(name))
	}

	return strings.Join(tabs, m.styles.TabBar.Render("│"))
}

// renderMainContent renders either the output stream or file content based on active tab.
func (m Model) renderMainContent() string {
	if m.activeTab == 0 || m.activeTab >= len(m.tabs) {
		return m.renderScrollArea()
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile {
		return m.renderFileContent(tab.FilePath)
	}

	return m.renderScrollArea()
}

// renderFileContent renders the content of a file.
func (m Model) renderFileContent(path string) string {
	height := m.layout.ScrollAreaHeight
	width := m.layout.ContentWidth()

	content, ok := m.fileContents[path]
	if !ok {
		// File not loaded yet
		lines := make([]string, height)
		lines[0] = m.styles.Label.Render("  Loading " + path + "...")
		return strings.Join(lines, "\n")
	}

	// Split content into lines
	fileLines := strings.Split(content, "\n")

	// Get scroll offset
	offset := m.fileScroll[path]
	if offset < 0 {
		offset = 0
	}
	if offset > len(fileLines)-height {
		offset = len(fileLines) - height
		if offset < 0 {
			offset = 0
		}
	}

	// Build visible lines with line numbers
	var lines []string
	for i := 0; i < height; i++ {
		lineIdx := offset + i
		if lineIdx >= len(fileLines) {
			lines = append(lines, "")
			continue
		}

		line := fileLines[lineIdx]
		lineNum := lineIdx + 1

		// Format line number (right-aligned, 5 chars)
		numStr := intToString(lineNum)
		for len(numStr) < 5 {
			numStr = " " + numStr
		}
		numStr = m.styles.Label.Render(numStr + "│")

		// Truncate long lines (ANSI-aware)
		visibleWidth := width - 6 // Account for line number column
		if ansi.StringWidth(line) > visibleWidth {
			line = ansi.Truncate(line, visibleWidth-3, "...")
		}

		lines = append(lines, numStr+line)
	}

	return strings.Join(lines, "\n")
}

// renderScrollArea renders the scrolling output region.
func (m Model) renderScrollArea() string {
	height := m.layout.ScrollAreaHeight
	width := m.layout.ContentWidth()

	// First, wrap all output lines to fit within width
	var wrappedLines []string
	for _, line := range m.outputLines {
		wrapped := wrapLine(line, width)
		wrappedLines = append(wrappedLines, wrapped...)
	}

	// Get visible lines (show most recent)
	startIdx := 0
	if len(wrappedLines) > height {
		startIdx = len(wrappedLines) - height
	}

	var lines []string
	for i := startIdx; i < len(wrappedLines) && len(lines) < height; i++ {
		lines = append(lines, wrappedLines[i])
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

// wrapLine wraps a single line to fit within the given width, preserving ANSI codes.
// Returns a slice of wrapped lines. Continuation lines are indented with 4 spaces.
func wrapLine(line string, width int) []string {
	if width <= 0 {
		return []string{line}
	}

	// Use ansi.StringWidth to measure visible width (excludes ANSI escape sequences)
	visibleWidth := ansi.StringWidth(line)
	if visibleWidth <= width {
		return []string{line}
	}

	const continuationIndent = "    " // 4 spaces for continuation lines
	continuationWidth := width - len(continuationIndent)
	if continuationWidth <= 10 {
		// Terminal too narrow for meaningful wrapping
		continuationWidth = width
	}

	var result []string
	remaining := line
	isFirst := true

	for len(remaining) > 0 {
		targetWidth := width
		if !isFirst {
			targetWidth = continuationWidth
		}

		if ansi.StringWidth(remaining) <= targetWidth {
			if isFirst {
				result = append(result, remaining)
			} else {
				result = append(result, continuationIndent+remaining)
			}
			break
		}

		// Find a good break point
		breakIdx := findBreakPoint(remaining, targetWidth)
		if breakIdx <= 0 {
			// No good break point, force break at width
			breakIdx = truncateToWidth(remaining, targetWidth)
			if breakIdx <= 0 {
				breakIdx = len(remaining)
			}
		}

		chunk := remaining[:breakIdx]
		if isFirst {
			result = append(result, chunk)
		} else {
			result = append(result, continuationIndent+chunk)
		}

		remaining = strings.TrimLeft(remaining[breakIdx:], " ")
		isFirst = false
	}

	return result
}

// findBreakPoint finds the best position to break a line at or before targetWidth.
// Returns the index after the last space that fits, or 0 if no good break point.
func findBreakPoint(s string, targetWidth int) int {
	lastSpace := -1
	currentWidth := 0
	inEscape := false

	for i, r := range s {
		// Track ANSI escape sequences
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			// ANSI sequences end with a letter
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		// Measure visible character width
		charWidth := ansi.StringWidth(string(r))
		if currentWidth+charWidth > targetWidth {
			// We've exceeded the target width
			if lastSpace >= 0 {
				return lastSpace + 1 // Include the space, then trim later
			}
			return i // Force break at current position
		}

		currentWidth += charWidth
		if r == ' ' {
			lastSpace = i
		}
	}

	return 0 // Line fits, no break needed
}

// truncateToWidth returns the byte index where the visible width reaches targetWidth.
func truncateToWidth(s string, targetWidth int) int {
	currentWidth := 0
	inEscape := false

	for i, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		charWidth := ansi.StringWidth(string(r))
		if currentWidth+charWidth > targetWidth {
			return i
		}
		currentWidth += charWidth
	}

	return len(s)
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
