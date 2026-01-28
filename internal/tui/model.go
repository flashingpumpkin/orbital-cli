package tui

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/flashingpumpkin/orbital/internal/tasks"
	"github.com/flashingpumpkin/orbital/internal/util"
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
	Iteration        int
	MaxIteration     int
	StepName         string
	StepPosition     int
	StepTotal        int
	GateRetries      int
	MaxRetries       int
	TokensIn         int
	TokensOut        int
	Cost             float64
	Budget           float64
	ContextWindow    int           // Model's context window size (e.g., 200000 for opus/sonnet/haiku)
	IterationTimeout time.Duration // Configured timeout for iterations
	IterationStart   time.Time     // When current iteration/step started
	IsGateStep       bool          // True if current step is a gate (timer hidden for gates)
	WorkflowName     string        // Name of the active workflow (e.g., "autonomous", "tdd")
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
	outputLines *RingBuffer     // Ring buffer for bounded memory usage
	viewport    viewport.Model  // Viewport for output scrolling
	tasks       []Task
	progress    ProgressInfo
	session     SessionInfo

	// Tabs
	tabs          []Tab                      // List of tabs
	activeTab     int                        // Index of active tab
	fileContents  map[string]string          // Cached file contents by path
	fileViewports map[string]viewport.Model  // Viewport per file tab
	fileModTimes  map[string]time.Time       // Last known modification times per file

	// Output scrolling
	outputTailing bool // Whether the output window is locked to the bottom (auto-scrolling)

	// Styles
	styles Styles

	// State
	ready bool
}

// NewModel creates a new TUI model with default dark theme.
func NewModel() Model {
	return NewModelWithTheme(ThemeDark)
}

// NewModelWithTheme creates a new TUI model with the specified theme.
func NewModelWithTheme(theme Theme) Model {
	vp := viewport.New(0, 0)
	return Model{
		outputLines:   NewRingBuffer(DefaultMaxOutputLines),
		viewport:      vp,
		tasks:         make([]Task, 0),
		tabs:          []Tab{{Name: "Output", Type: TabOutput}},
		activeTab:     0,
		fileContents:  make(map[string]string),
		fileViewports: make(map[string]viewport.Model),
		fileModTimes:  make(map[string]time.Time),
		outputTailing: true,
		styles:        GetStyles(theme),
		progress: ProgressInfo{
			Iteration:    1,
			MaxIteration: 50,
		},
	}
}


// fileRefreshInterval is the interval between file refresh checks.
const fileRefreshInterval = 2 * time.Second

// fileRefreshTickMsg signals that it's time to check for file changes.
type fileRefreshTickMsg time.Time

// fileRefreshTick creates a tick command for file refresh.
func fileRefreshTick() tea.Cmd {
	return tea.Tick(fileRefreshInterval, func(t time.Time) tea.Msg {
		return fileRefreshTickMsg(t)
	})
}

// timerTickInterval is the interval for the countdown timer updates.
const timerTickInterval = time.Second

// timerTickMsg signals that it's time to update the countdown timer.
type timerTickMsg time.Time

// timerTick creates a tick command for the countdown timer.
func timerTick() tea.Cmd {
	return tea.Tick(timerTickInterval, func(t time.Time) tea.Msg {
		return timerTickMsg(t)
	})
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(fileRefreshTick(), timerTick())
}

// FileContentMsg contains loaded file content.
type FileContentMsg struct {
	Path    string
	Content string
	Error   error
}

// maxFileSize is the maximum file size to load (1MB).
const maxFileSize = 1024 * 1024

// loadFileCmd creates a command to load file content.
func loadFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		// Check file size first
		info, err := os.Stat(path)
		if err != nil {
			return FileContentMsg{Path: path, Error: err}
		}
		if info.Size() > maxFileSize {
			return FileContentMsg{
				Path:    path,
				Content: "(File too large to display: " + formatFileSize(info.Size()) + ")",
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return FileContentMsg{Path: path, Error: err}
		}
		return FileContentMsg{Path: path, Content: string(content)}
	}
}

// formatFileSize formats a file size in human-readable form.
func formatFileSize(size int64) string {
	if size < 1024 {
		return util.IntToString(int(size)) + " B"
	}
	if size < 1024*1024 {
		return util.IntToString(int(size/1024)) + " KB"
	}
	return util.IntToString(int(size/(1024*1024))) + " MB"
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = CalculateLayout(msg.Width, msg.Height, len(m.tasks))
		m.ready = true

		// Update output viewport dimensions
		m.viewport.Width = m.layout.ContentWidth()
		m.viewport.Height = m.layout.ScrollAreaHeight

		// Rebuild viewport content from ring buffer
		m.syncViewportContent()

		// Sync tailing state with viewport position after resize.
		// If the resize puts the user at the bottom, enable tailing;
		// if they were at the bottom but resize changed that, keep tailing.
		if m.outputTailing || m.viewport.AtBottom() {
			m.outputTailing = true
			m.viewport.GotoBottom()
		}

		// Update all file viewports dimensions.
		// Iterate over fileContents (not fileViewports) to ensure files loaded
		// before valid dimensions are established get their viewports created.
		for path := range m.fileContents {
			m.syncFileViewport(path)
		}

		return m, nil

	case StatsMsg:
		m.progress.TokensIn = msg.TokensIn
		m.progress.TokensOut = msg.TokensOut
		m.progress.Cost = msg.Cost
		return m, nil

	case OutputLineMsg:
		m.outputLines.Push(string(msg))
		m.syncViewportContent()
		return m, nil

	case TasksMsg:
		m.tasks = msg
		if m.ready {
			m.layout = CalculateLayout(m.layout.Width, m.layout.Height, len(m.tasks))
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

	case FileContentMsg:
		if msg.Error != nil {
			m.fileContents[msg.Path] = "Error loading file: " + msg.Error.Error()
		} else {
			m.fileContents[msg.Path] = msg.Content
		}
		// Update modification time
		if info, err := os.Stat(msg.Path); err == nil {
			m.fileModTimes[msg.Path] = info.ModTime()
		}
		// Create or update viewport for this file
		m.syncFileViewport(msg.Path)
		return m, nil

	case fileRefreshTickMsg:
		// Schedule next tick
		cmd := fileRefreshTick()

		// Only check file changes when on a file tab (not Output tab)
		if m.activeTab > 0 && m.activeTab < len(m.tabs) {
			tab := m.tabs[m.activeTab]
			if tab.Type == TabFile && tab.FilePath != "" {
				// Check if file has been modified
				if info, err := os.Stat(tab.FilePath); err == nil {
					lastMod, exists := m.fileModTimes[tab.FilePath]
					if !exists || info.ModTime().After(lastMod) {
						// File changed, reload it
						return m, tea.Batch(cmd, loadFileCmd(tab.FilePath))
					}
				}
			}
		}
		return m, cmd

	case timerTickMsg:
		// Just schedule next tick - the timer display updates on each render
		return m, timerTick()

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
			return m.handleScrollUp()
		case "down", "j":
			return m.handleScrollDown()
		case "pgup":
			return m.handleScrollPageUp()
		case "pgdown":
			return m.handleScrollPageDown()
		case "home":
			return m.handleScrollHome()
		case "end":
			return m.handleScrollEnd()
		case "r":
			return m.reloadCurrentFile()
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			return m.handleScrollUp()
		case tea.MouseButtonWheelDown:
			return m.handleScrollDown()
		case tea.MouseButtonLeft:
			if msg.Action == tea.MouseActionPress && msg.Y == 0 {
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

	// Add context files (handle both ", " and "," separators)
	if m.session.ContextFile != "" {
		// Split on comma, then trim spaces from each part
		for _, path := range strings.Split(m.session.ContextFile, ",") {
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
	width := m.layout.Width
	sepWidth := 1 // separator "│" width
	currentX := 0

	for i, tab := range m.tabs {
		// Tab name with number prefix (must match renderTabBar logic)
		name := tab.Name
		if i < 9 {
			name = util.IntToString(i+1) + ":" + name
		}
		// Tab width is name length + 2 (for padding from style)
		tabWidth := ansi.StringWidth(name) + 2

		// Check if this tab would overflow (match renderTabBar logic)
		neededWidth := tabWidth
		if i > 0 {
			neededWidth += sepWidth
		}
		if currentX+neededWidth > width {
			break // Tab is truncated, not clickable
		}

		// Add separator width for tabs after the first
		clickStart := currentX
		if i > 0 {
			clickStart += sepWidth
		}

		if x >= clickStart && x < clickStart+tabWidth {
			return m.switchToTab(i)
		}

		currentX += neededWidth
	}
	return m, nil
}

// handleScrollUp handles scroll up for the current tab.
func (m Model) handleScrollUp() (tea.Model, tea.Cmd) {
	// Handle output tab (tab 0)
	if m.activeTab == 0 {
		// Disable tailing when user scrolls up
		if m.outputTailing {
			m.outputTailing = false
		}
		m.viewport.ScrollUp(1)
		return m, nil
	}

	// Handle file tabs
	if len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		if vp, ok := m.fileViewports[tab.FilePath]; ok {
			vp.ScrollUp(1)
			m.fileViewports[tab.FilePath] = vp
		}
	}
	return m, nil
}

// handleScrollDown handles scroll down for the current tab.
func (m Model) handleScrollDown() (tea.Model, tea.Cmd) {
	// Handle output tab (tab 0)
	if m.activeTab == 0 {
		m.viewport.ScrollDown(1)
		// Re-enable tailing if we've scrolled to the bottom
		if m.viewport.AtBottom() {
			m.outputTailing = true
		}
		return m, nil
	}

	// Handle file tabs
	if len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		if vp, ok := m.fileViewports[tab.FilePath]; ok {
			vp.ScrollDown(1)
			m.fileViewports[tab.FilePath] = vp
		}
	}
	return m, nil
}

// handleScrollPageUp handles page up for the current tab.
func (m Model) handleScrollPageUp() (tea.Model, tea.Cmd) {
	// Handle output tab (tab 0)
	if m.activeTab == 0 {
		// Disable tailing when user scrolls up
		if m.outputTailing {
			m.outputTailing = false
		}
		m.viewport.HalfPageUp()
		return m, nil
	}

	// Handle file tabs
	if len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		if vp, ok := m.fileViewports[tab.FilePath]; ok {
			vp.HalfPageUp()
			m.fileViewports[tab.FilePath] = vp
		}
	}
	return m, nil
}

// handleScrollPageDown handles page down for the current tab.
func (m Model) handleScrollPageDown() (tea.Model, tea.Cmd) {
	// Handle output tab (tab 0)
	if m.activeTab == 0 {
		m.viewport.HalfPageDown()
		// Re-enable tailing if we've scrolled to the bottom
		if m.viewport.AtBottom() {
			m.outputTailing = true
		}
		return m, nil
	}

	// Handle file tabs
	if len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		if vp, ok := m.fileViewports[tab.FilePath]; ok {
			vp.HalfPageDown()
			m.fileViewports[tab.FilePath] = vp
		}
	}
	return m, nil
}

// handleScrollHome handles home key for the current tab.
func (m Model) handleScrollHome() (tea.Model, tea.Cmd) {
	if m.activeTab == 0 {
		m.outputTailing = false
		m.viewport.GotoTop()
		return m, nil
	}

	// Handle file tabs
	if len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		if vp, ok := m.fileViewports[tab.FilePath]; ok {
			vp.GotoTop()
			m.fileViewports[tab.FilePath] = vp
		}
	}
	return m, nil
}

// handleScrollEnd handles end key for the current tab.
func (m Model) handleScrollEnd() (tea.Model, tea.Cmd) {
	if m.activeTab == 0 {
		m.outputTailing = true
		m.viewport.GotoBottom()
		return m, nil
	}

	// Handle file tabs
	if len(m.tabs) <= m.activeTab {
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	if tab.Type == TabFile && tab.FilePath != "" {
		if vp, ok := m.fileViewports[tab.FilePath]; ok {
			vp.GotoBottom()
			m.fileViewports[tab.FilePath] = vp
		}
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
		// Clear cached content and viewport to trigger reload
		delete(m.fileContents, tab.FilePath)
		delete(m.fileViewports, tab.FilePath)
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

	// Top border
	sections = append(sections, RenderTopBorder(m.layout.Width, m.styles.Border))

	// Header panel with brand and metrics
	sections = append(sections, m.renderHeader())
	sections = append(sections, RenderDoubleBorder(m.layout.Width, m.styles.Border))

	// Tab bar
	sections = append(sections, m.renderTabBar())
	sections = append(sections, RenderDoubleBorder(m.layout.Width, m.styles.Border))

	// Main content area (output or file content)
	sections = append(sections, m.renderMainContent())
	sections = append(sections, RenderDoubleBorder(m.layout.Width, m.styles.Border))

	// Task panel (if tasks exist)
	if m.layout.TaskPanelHeight > 0 {
		sections = append(sections, m.renderTaskPanel())
		sections = append(sections, RenderDoubleBorder(m.layout.Width, m.styles.Border))
	}

	// Progress panel
	sections = append(sections, m.renderProgressPanel())
	sections = append(sections, RenderDoubleBorder(m.layout.Width, m.styles.Border))

	// Session info panel
	sections = append(sections, m.renderSessionPanel())

	// Bottom border
	sections = append(sections, RenderBottomBorder(m.layout.Width, m.styles.Border))

	// Help bar (outside the main frame)
	sections = append(sections, m.renderHelpBar())

	return strings.Join(sections, "\n")
}

// renderHeader renders the header panel with brand and key metrics.
func (m Model) renderHeader() string {
	width := m.layout.ContentWidth()
	p := m.progress

	// Left side: brand
	brand := m.styles.Brand.Render(IconBrand + " ORBITAL")

	// Right side: iteration and cost
	iterStr := formatFraction(p.Iteration, p.MaxIteration)
	costStr := formatCurrency(p.Cost) + "/" + formatCurrency(p.Budget)

	// Apply warning colour if thresholds exceeded
	// Guard against division by zero when MaxIteration or Budget is 0
	var iterRatio, costRatio float64
	if p.MaxIteration > 0 {
		iterRatio = float64(p.Iteration) / float64(p.MaxIteration)
	}
	if p.Budget > 0 {
		costRatio = p.Cost / p.Budget
	}

	var iterStyled, costStyled string
	if p.MaxIteration > 0 && iterRatio > 0.8 {
		iterStyled = m.styles.Warning.Render(iterStr)
	} else {
		iterStyled = m.styles.Value.Render(iterStr)
	}
	if p.Budget > 0 && costRatio > 0.8 {
		costStyled = m.styles.Warning.Render(costStr)
	} else {
		costStyled = m.styles.Value.Render(costStr)
	}

	metrics := m.styles.Label.Render("Iteration ") + iterStyled +
		m.styles.Label.Render("  " + InnerVertical + "  ") +
		costStyled

	// Calculate padding between brand and metrics
	// Account for the 2 extra space characters (after left border and before right border)
	brandWidth := ansi.StringWidth(IconBrand + " ORBITAL")
	metricsWidth := ansi.StringWidth("Iteration " + iterStr + "  " + InnerVertical + "  " + costStr)
	padding := width - brandWidth - metricsWidth - 2
	if padding < 1 {
		padding = 1
	}

	// Build the line content
	content := " " + brand + strings.Repeat(" ", padding) + metrics + " "
	// Truncate if content exceeds available width
	if ansi.StringWidth(content) > width {
		content = ansi.Truncate(content, width, "")
	}

	return m.styles.Border.Render(BoxVertical) + content + m.styles.Border.Render(BoxVertical)
}

// renderHelpBar renders the help text below the main frame.
func (m Model) renderHelpBar() string {
	help := "  " + m.styles.HelpKey.Render("↑/↓") + m.styles.HelpBar.Render(" scroll  ") +
		m.styles.HelpKey.Render("←/→") + m.styles.HelpBar.Render(" tab  ") +
		m.styles.HelpKey.Render("1-9") + m.styles.HelpBar.Render(" jump  ") +
		m.styles.HelpKey.Render("r") + m.styles.HelpBar.Render(" reload  ") +
		m.styles.HelpKey.Render("q") + m.styles.HelpBar.Render(" quit")
	return help
}

// renderTabBar renders the tab bar with all tabs, truncating if needed.
func (m Model) renderTabBar() string {
	contentWidth := m.layout.ContentWidth()
	separator := m.styles.TabBar.Render(InnerVertical)
	sepWidth := ansi.StringWidth(InnerVertical)

	var tabs []string
	currentWidth := 0

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
			name = util.IntToString(i+1) + ":" + name
		}

		// Calculate tab width (name + padding from style)
		tabWidth := ansi.StringWidth(name) + 2 // +2 for padding

		// Check if this tab would overflow
		neededWidth := tabWidth
		if len(tabs) > 0 {
			neededWidth += sepWidth
		}

		if currentWidth+neededWidth > contentWidth {
			// Truncate: show "..." indicator and stop
			if currentWidth+sepWidth+5 <= contentWidth { // 5 = "..." + minimal padding
				tabs = append(tabs, m.styles.TabInactive.Render("..."))
			}
			break
		}

		tabs = append(tabs, style.Render(name))
		currentWidth += neededWidth
	}

	tabContent := strings.Join(tabs, separator)
	// Pad to fill width
	tabWidth := ansi.StringWidth(tabContent)
	padding := contentWidth - tabWidth
	if padding < 0 {
		// Content exceeds available width - truncate to fit
		tabContent = ansi.Truncate(tabContent, contentWidth, "")
		padding = 0
	}

	return m.styles.Border.Render(BoxVertical) + tabContent + strings.Repeat(" ", padding) + m.styles.Border.Render(BoxVertical)
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

// renderFileContent renders the content of a file using viewport for scrolling.
func (m Model) renderFileContent(path string) string {
	height := m.layout.ScrollAreaHeight
	contentWidth := m.layout.ContentWidth()

	// Guard against invalid dimensions
	if height <= 0 {
		return ""
	}
	if contentWidth < 0 {
		contentWidth = 0
	}

	border := m.styles.Border.Render(BoxVertical)

	content, ok := m.fileContents[path]
	if !ok {
		// File not loaded yet
		var lines []string
		loadingLine := m.styles.Label.Render("  Loading " + path + "...")
		loadingWidth := ansi.StringWidth("  Loading " + path + "...")
		padding := contentWidth - loadingWidth
		if padding < 0 {
			padding = 0
		}
		lines = append(lines, border+loadingLine+strings.Repeat(" ", padding)+border)
		emptyLine := border + strings.Repeat(" ", contentWidth) + border
		for len(lines) < height {
			lines = append(lines, emptyLine)
		}
		return strings.Join(lines, "\n")
	}

	// Get viewport for scroll position
	vp, vpOk := m.fileViewports[path]
	offset := 0
	if vpOk {
		offset = vp.YOffset
	}

	// Split content into lines
	fileLines := strings.Split(content, "\n")

	// Clamp offset to valid range
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
			emptyLine := border + strings.Repeat(" ", contentWidth) + border
			lines = append(lines, emptyLine)
			continue
		}

		line := fileLines[lineIdx]
		lineNum := lineIdx + 1

		// Format line number (right-aligned, 5 chars)
		numStr := util.IntToString(lineNum)
		for len(numStr) < 5 {
			numStr = " " + numStr
		}
		numStr = m.styles.Label.Render(numStr + InnerVertical)

		// Truncate long lines (ANSI-aware)
		visibleWidth := contentWidth - 6 // Account for line number column
		if visibleWidth < 1 {
			visibleWidth = 1 // Minimum visible width to avoid negative truncation
		}
		if ansi.StringWidth(line) > visibleWidth {
			truncateWidth := visibleWidth - 3
			if truncateWidth < 1 {
				truncateWidth = 1
			}
			line = ansi.Truncate(line, truncateWidth, "...")
		}

		// Pad line to content width
		lineContent := numStr + line
		lineWidth := ansi.StringWidth(numStr) + ansi.StringWidth(line)
		padding := contentWidth - lineWidth
		if padding < 0 {
			padding = 0
		}

		lines = append(lines, border+lineContent+strings.Repeat(" ", padding)+border)
	}

	return strings.Join(lines, "\n")
}

// renderScrollArea renders the scrolling output region using the viewport.
func (m Model) renderScrollArea() string {
	height := m.layout.ScrollAreaHeight
	contentWidth := m.layout.ContentWidth()

	// Guard against invalid dimensions
	if height <= 0 {
		return ""
	}
	if contentWidth < 0 {
		contentWidth = 0
	}

	border := m.styles.Border.Render(BoxVertical)
	emptyLine := border + strings.Repeat(" ", contentWidth) + border

	// Empty state: show waiting message
	if m.outputLines.Len() == 0 {
		var lines []string
		midHeight := height / 2
		for i := 0; i < midHeight-1; i++ {
			lines = append(lines, emptyLine)
		}
		// Centred waiting message
		waitMsg := m.styles.Label.Render("Waiting for output...")
		waitWidth := ansi.StringWidth(waitMsg) // Measure the styled message, not raw text
		leftPad := (contentWidth - waitWidth) / 2
		rightPad := contentWidth - waitWidth - leftPad
		// Guard against negative padding (terminal too narrow for message)
		if leftPad < 0 {
			leftPad = 0
		}
		if rightPad < 0 {
			rightPad = 0
		}
		lines = append(lines, border+strings.Repeat(" ", leftPad)+waitMsg+strings.Repeat(" ", rightPad)+border)
		for len(lines) < height {
			lines = append(lines, emptyLine)
		}
		return strings.Join(lines, "\n")
	}

	// Get viewport content
	viewContent := m.viewport.View()
	viewLines := strings.Split(viewContent, "\n")

	// Build output with borders
	var lines []string
	for i := 0; i < height; i++ {
		var line string
		if i < len(viewLines) {
			line = viewLines[i]
		}
		// Pad line to content width
		lineWidth := ansi.StringWidth(line)
		padding := contentWidth - lineWidth
		if padding < 0 {
			// Truncate if line exceeds width
			line = ansi.Truncate(line, contentWidth, "")
			padding = 0
		}
		lines = append(lines, border+line+strings.Repeat(" ", padding)+border)
	}

	return strings.Join(lines, "\n")
}

// renderTaskPanel renders the task list panel.
func (m Model) renderTaskPanel() string {
	var lines []string
	contentWidth := m.layout.ContentWidth()
	border := m.styles.Border.Render(BoxVertical)

	// Header
	headerText := m.styles.Header.Render("Tasks")
	if m.layout.HasTaskOverflow(len(m.tasks)) {
		headerText += m.styles.Label.Render(" (scroll)")
	}
	headerWidth := ansi.StringWidth("Tasks")
	if m.layout.HasTaskOverflow(len(m.tasks)) {
		headerWidth += ansi.StringWidth(" (scroll)")
	}
	headerContent := "  " + headerText
	padding := contentWidth - headerWidth - 2
	if padding < 0 {
		// Content exceeds available width - truncate to fit
		headerContent = ansi.Truncate(headerContent, contentWidth, "")
		padding = 0
	}
	lines = append(lines, border+headerContent+strings.Repeat(" ", padding)+border)

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
		icon = IconComplete
		style = m.styles.TaskComplete
	case "in_progress":
		icon = IconInProgress
		style = m.styles.TaskInProgress
	default:
		icon = IconPending
		style = m.styles.TaskPending
	}

	contentWidth := m.layout.ContentWidth()
	border := m.styles.Border.Render(BoxVertical)

	content := task.Content
	maxLen := contentWidth - 6 // icon + spacing + borders
	if maxLen < 4 {
		maxLen = 4 // Minimum space for "..."
	}
	if ansi.StringWidth(content) > maxLen {
		// Use ANSI-aware truncation
		content = ansi.Truncate(content, maxLen-3, "...")
	}

	taskContent := style.Render("  " + icon + " " + content)
	taskWidth := ansi.StringWidth("  " + icon + " " + content)
	padding := contentWidth - taskWidth
	if padding < 0 {
		// Content exceeds available width - truncate to fit
		taskContent = ansi.Truncate(taskContent, contentWidth, "")
		padding = 0
	}

	return border + taskContent + strings.Repeat(" ", padding) + border
}

// renderProgressPanel renders the progress and metrics panel.
func (m Model) renderProgressPanel() string {
	p := m.progress
	contentWidth := m.layout.ContentWidth()
	border := m.styles.Border.Render(BoxVertical)

	// Line 1: Iteration progress bar and step info
	// Guard against division by zero when MaxIteration is 0
	var iterRatio float64
	if p.MaxIteration > 0 {
		iterRatio = float64(p.Iteration) / float64(p.MaxIteration)
	}
	iterBar := RenderProgressBar(iterRatio, BarWidth, m.styles.Value, m.styles.Warning)
	iterLabel := m.styles.Label.Render("Iteration ")
	iterValue := m.styles.Value.Render(formatFraction(p.Iteration, p.MaxIteration))
	if p.MaxIteration > 0 && iterRatio > 0.8 {
		iterValue = m.styles.Warning.Render(formatFraction(p.Iteration, p.MaxIteration))
	}

	stepStr := m.formatStep(p.StepName, p.StepPosition, p.StepTotal)
	gateStr := ""
	if p.GateRetries > 0 || p.MaxRetries > 0 {
		gateStr = m.formatGateRetries(p.GateRetries, p.MaxRetries)
	}
	timerStr := m.formatIterationTimer()

	line1Parts := []string{iterBar + " " + iterLabel + iterValue}
	if timerStr != "" {
		line1Parts = append(line1Parts, timerStr)
	}
	if stepStr != "" {
		line1Parts = append(line1Parts, stepStr)
	}
	if gateStr != "" {
		line1Parts = append(line1Parts, gateStr)
	}
	line1Content := " " + strings.Join(line1Parts, " "+InnerVertical+" ")
	line1Width := ansi.StringWidth(line1Content)
	line1Padding := contentWidth - line1Width
	if line1Padding < 0 {
		// Content exceeds available width - truncate to fit
		line1Content = ansi.Truncate(line1Content, contentWidth, "")
		line1Padding = 0
	}

	// Line 2: Budget progress bar, tokens and cost
	var costRatio float64
	if p.Budget > 0 {
		costRatio = p.Cost / p.Budget
	}
	budgetBar := RenderProgressBar(costRatio, BarWidth, m.styles.Value, m.styles.Warning)
	tokensStr := m.formatTokens(p.TokensIn, p.TokensOut)
	costStr := m.formatCost(p.Cost, p.Budget)
	line2Content := " " + budgetBar + " " + tokensStr + " " + InnerVertical + " " + costStr
	line2Width := ansi.StringWidth(line2Content)
	line2Padding := contentWidth - line2Width
	if line2Padding < 0 {
		// Content exceeds available width - truncate to fit
		line2Content = ansi.Truncate(line2Content, contentWidth, "")
		line2Padding = 0
	}

	// Line 3: Context window progress bar
	var contextRatio float64
	if p.ContextWindow > 0 {
		contextRatio = float64(p.TokensIn+p.TokensOut) / float64(p.ContextWindow)
	}
	contextBar := RenderProgressBar(contextRatio, BarWidth, m.styles.Value, m.styles.Warning)
	contextStr := m.formatContext(p.TokensIn+p.TokensOut, p.ContextWindow, contextRatio)
	line3Content := " " + contextBar + " " + contextStr
	line3Width := ansi.StringWidth(line3Content)
	line3Padding := contentWidth - line3Width
	if line3Padding < 0 {
		// Content exceeds available width - truncate to fit
		line3Content = ansi.Truncate(line3Content, contentWidth, "")
		line3Padding = 0
	}

	line1 := border + line1Content + strings.Repeat(" ", line1Padding) + border
	line2 := border + line2Content + strings.Repeat(" ", line2Padding) + border
	line3 := border + line3Content + strings.Repeat(" ", line3Padding) + border

	return line1 + "\n" + line2 + "\n" + line3
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
	inStr := m.styles.Value.Render(util.FormatNumber(in))
	outStr := m.styles.Value.Render(util.FormatNumber(out))
	return label + inStr + m.styles.Label.Render(" in / ") + outStr + m.styles.Label.Render(" out")
}

// formatCost formats cost with optional warning colour.
func (m Model) formatCost(cost, budget float64) string {
	label := m.styles.Label.Render("Cost: ")

	// Guard against division by zero: if budget is zero, don't show warning colour
	var costStr string
	if budget > 0 && cost/budget > 0.8 {
		costStr = m.styles.Warning.Render(formatCurrency(cost))
	} else {
		costStr = m.styles.Value.Render(formatCurrency(cost))
	}

	budgetStr := m.styles.Label.Render(" / ") + m.styles.Value.Render(formatCurrency(budget))
	return label + costStr + budgetStr
}

// formatContext formats context window usage with optional warning colour.
func (m Model) formatContext(used, window int, ratio float64) string {
	label := m.styles.Label.Render("Context: ")

	usedStr := util.FormatNumber(used)
	windowStr := util.FormatNumber(window)
	percentStr := util.IntToString(int(ratio*100)) + "%"

	// Apply warning colour if ratio exceeds 80%
	var valueStr string
	if ratio > 0.8 {
		valueStr = m.styles.Warning.Render(usedStr + "/" + windowStr + " (" + percentStr + ")")
	} else {
		valueStr = m.styles.Value.Render(usedStr + "/" + windowStr + " (" + percentStr + ")")
	}

	return label + valueStr
}

// formatIterationTimer formats the iteration countdown timer.
// Returns empty string if timer should be hidden (no iteration running or gate step).
func (m Model) formatIterationTimer() string {
	p := m.progress

	// Hide timer if no iteration is running (start time not set)
	if p.IterationStart.IsZero() {
		return ""
	}

	// Hide timer if timeout is not set
	if p.IterationTimeout <= 0 {
		return ""
	}

	// Hide timer during gate steps
	if p.IsGateStep {
		return ""
	}

	elapsed := time.Since(p.IterationStart)
	remaining := p.IterationTimeout - elapsed

	// Clamp to zero if negative
	if remaining < 0 {
		remaining = 0
	}

	// Format as "Xm Ys"
	mins := int(remaining.Minutes())
	secs := int(remaining.Seconds()) % 60
	timerStr := util.IntToString(mins) + "m " + util.IntToString(secs) + "s"

	// Apply warning colour if less than 1 minute remaining
	if remaining < time.Minute {
		return m.styles.Warning.Render(timerStr)
	}
	return m.styles.Label.Render(timerStr)
}

// renderSessionPanel renders the session info panel.
func (m Model) renderSessionPanel() string {
	s := m.session
	p := m.progress
	contentWidth := m.layout.ContentWidth()
	border := m.styles.Border.Render(BoxVertical)

	// Line 1: Spec file(s) and workflow name
	specStr := m.formatPaths("Spec", s.SpecFiles)
	line1Content := " " + specStr
	// Add workflow name if set
	if p.WorkflowName != "" {
		workflowStr := m.styles.Label.Render("Workflow: ") + m.styles.Value.Render(p.WorkflowName)
		line1Content += " " + InnerVertical + " " + workflowStr
	}
	line1Width := ansi.StringWidth(line1Content)
	line1Padding := contentWidth - line1Width
	if line1Padding < 0 {
		// Content exceeds available width - truncate to fit
		line1Content = ansi.Truncate(line1Content, contentWidth, "")
		line1Padding = 0
	}

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

	line2Content := " " + strings.Join(line2Parts, " "+InnerVertical+" ")
	line2Width := ansi.StringWidth(line2Content)
	line2Padding := contentWidth - line2Width
	if line2Padding < 0 {
		// Content exceeds available width - truncate to fit
		line2Content = ansi.Truncate(line2Content, contentWidth, "")
		line2Padding = 0
	}

	line1 := border + line1Content + strings.Repeat(" ", line1Padding) + border
	line2 := border + line2Content + strings.Repeat(" ", line2Padding) + border

	return line1 + "\n" + line2
}

// formatPath formats a single file path with truncation.
func (m Model) formatPath(label, path string) string {
	labelStr := m.styles.Label.Render(label + ": ")
	maxLen := 40
	if ansi.StringWidth(path) > maxLen {
		// Truncate from the start to show the filename
		// Find how many characters to remove from the start
		truncLen := maxLen - 3 // Reserve space for "..."
		if truncLen < 1 {
			truncLen = 1
		}
		// Walk backwards to find the right truncation point
		path = truncateFromStart(path, truncLen)
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
		if ansi.StringWidth(path) > maxLen {
			// Truncate from the start to show the filename
			truncLen := maxLen - 3
			if truncLen < 1 {
				truncLen = 1
			}
			path = truncateFromStart(path, truncLen)
		}
		return labelStr + m.styles.Value.Render(path)
	}
	return labelStr + m.styles.Value.Render(util.FormatNumber(len(paths))+" files")
}

// Helper functions for formatting

func formatFraction(a, b int) string {
	return util.IntToString(a) + "/" + util.IntToString(b)
}


func formatCurrency(amount float64) string {
	// Handle negative amounts by formatting absolute value and prepending minus
	if amount < 0 {
		return "-" + formatCurrency(-amount)
	}
	// Format as $X.XX with proper rounding using math.Round for precision
	totalCents := int(math.Round(amount * 100))
	whole := totalCents / 100
	cents := totalCents % 100
	return "$" + util.FormatNumber(whole) + "." + padLeft(util.IntToString(cents), 2, '0')
}


func padLeft(s string, length int, pad rune) string {
	for len(s) < length {
		s = string(pad) + s
	}
	return s
}

// truncateFromStart truncates a string from the beginning to fit within targetWidth,
// prepending "..." to indicate truncation. Uses ANSI-aware width measurement.
func truncateFromStart(s string, targetWidth int) string {
	if targetWidth <= 0 {
		return "..."
	}

	visWidth := ansi.StringWidth(s)
	if visWidth <= targetWidth {
		return s
	}

	// Walk through the string from the end to find where to start
	// We want to keep the last targetWidth characters (by visible width)
	runes := []rune(s)
	width := 0
	startIdx := len(runes)

	for i := len(runes) - 1; i >= 0; i-- {
		charWidth := ansi.StringWidth(string(runes[i]))
		if width+charWidth > targetWidth {
			break
		}
		width += charWidth
		startIdx = i
	}

	return "..." + string(runes[startIdx:])
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
		m.layout = CalculateLayout(m.layout.Width, m.layout.Height, len(tasks))
	}
}

// AppendOutput adds a line to the output buffer.
// When the buffer is full, the oldest line is evicted automatically.
// This also updates the viewport content and maintains tailing mode.
func (m *Model) AppendOutput(line string) {
	m.outputLines.Push(line)
	m.syncViewportContent()
}

// syncFileViewport creates or updates a viewport for a file tab.
// It sets the viewport dimensions and content from the cached file content.
func (m *Model) syncFileViewport(path string) {
	content, ok := m.fileContents[path]
	if !ok {
		return
	}

	// Get or create viewport for this file
	vp, exists := m.fileViewports[path]
	if !exists {
		vp = viewport.New(0, 0)
	}

	// Update dimensions - file content area excludes line numbers (6 chars: "NNNNN│")
	vp.Width = m.layout.ContentWidth() - 6
	if vp.Width < 1 {
		vp.Width = 1
	}
	vp.Height = m.layout.ScrollAreaHeight

	// Guard against zero-height viewport.
	// Save the viewport first to preserve scroll position even if dimensions are invalid.
	// This prevents losing scroll state during extreme resize sequences.
	if vp.Height <= 0 {
		m.fileViewports[path] = vp
		return
	}

	// Use lipgloss to wrap content to viewport width
	wrapStyle := lipgloss.NewStyle().Width(vp.Width)
	wrapped := wrapStyle.Render(content)
	vp.SetContent(wrapped)

	// If this is a new viewport, start at the top
	if !exists {
		vp.GotoTop()
	}

	m.fileViewports[path] = vp
}

// outputPaddingLeft is the left padding for output content in the viewport.
const outputPaddingLeft = 2

// syncViewportContent rebuilds viewport content from the ring buffer.
// If tailing is enabled, it scrolls to the bottom after content update.
func (m *Model) syncViewportContent() {
	// Guard against zero-dimension viewport (before WindowSizeMsg arrives).
	// Viewport operations on zero dimensions have undefined behaviour.
	if m.viewport.Width <= 0 || m.viewport.Height <= 0 {
		return
	}

	var lines []string
	m.outputLines.Iterate(func(_ int, line string) bool {
		lines = append(lines, line)
		return true
	})

	// Use lipgloss to wrap and pad content
	// Account for padding in the wrap width
	wrapWidth := m.viewport.Width - outputPaddingLeft
	if wrapWidth < 1 {
		wrapWidth = 1
	}
	contentStyle := lipgloss.NewStyle().Width(wrapWidth).PaddingLeft(outputPaddingLeft)
	wrapped := contentStyle.Render(strings.Join(lines, "\n"))
	m.viewport.SetContent(wrapped)

	// If tailing, scroll to bottom
	if m.outputTailing {
		m.viewport.GotoBottom()
	}
}

// ClearOutput clears the output buffer and viewport.
// It also resets tailing mode so new content will auto-scroll.
func (m *Model) ClearOutput() {
	m.outputLines.Clear()
	m.viewport.SetContent("")
	m.outputTailing = true
}
