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
	outputLines *RingBuffer // Ring buffer for bounded memory usage
	tasks       []Task
	progress    ProgressInfo
	session     SessionInfo
	worktree    WorktreeInfo

	// Tabs
	tabs         []Tab             // List of tabs
	activeTab    int               // Index of active tab
	fileContents map[string]string // Cached file contents by path
	fileScroll   map[string]int    // Scroll offset per file

	// Output scrolling
	outputScroll  int  // Line offset from the top of the wrapped output buffer
	outputTailing bool // Whether the output window is locked to the bottom (auto-scrolling)

	// Wrapped lines cache for scroll performance
	wrappedLinesCache []string // Cache of wrapped output lines
	cacheWidth        int      // Width used for current cache (invalidate on change)
	cacheLineCount    int      // Number of raw lines when cache was built

	// Styles
	styles Styles

	// State
	ready bool
}

// NewModel creates a new TUI model.
func NewModel() Model {
	return Model{
		outputLines:   NewRingBuffer(DefaultMaxOutputLines),
		tasks:         make([]Task, 0),
		tabs:          []Tab{{Name: "Output", Type: TabOutput}},
		activeTab:     0,
		fileContents:  make(map[string]string),
		fileScroll:    make(map[string]int),
		outputScroll:  0,
		outputTailing: true,
		styles:        defaultStyles(),
		progress: ProgressInfo{
			Iteration:    1,
			MaxIteration: 50,
		},
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
		return intToString(int(size)) + " B"
	}
	if size < 1024*1024 {
		return intToString(int(size/1024)) + " KB"
	}
	return intToString(int(size/(1024*1024))) + " MB"
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = CalculateLayout(msg.Width, msg.Height, len(m.tasks), m.worktree.Path != "")
		m.ready = true

		// Invalidate wrapped lines cache since width may have changed
		m.invalidateWrappedLinesCache()
		m.updateWrappedLinesCache()

		// Clamp output scroll position if not tailing
		if !m.outputTailing {
			wrappedLines := m.wrapAllOutputLines()
			height := m.layout.ScrollAreaHeight
			maxOffset := len(wrappedLines) - height
			if maxOffset < 0 {
				maxOffset = 0
			}
			// Clamp scroll to valid range
			if m.outputScroll > maxOffset {
				m.outputScroll = maxOffset
			}
			// If output now fits in viewport, optionally resume tailing
			if maxOffset == 0 {
				m.outputTailing = true
				m.outputScroll = 0
			}
		}
		return m, nil

	case StatsMsg:
		m.progress.TokensIn = msg.TokensIn
		m.progress.TokensOut = msg.TokensOut
		m.progress.Cost = msg.Cost
		return m, nil

	case OutputLineMsg:
		m.outputLines.Push(string(msg))
		m.appendLineToCache(string(msg))
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
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			return m.scrollUp()
		case tea.MouseButtonWheelDown:
			return m.scrollDown()
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
			name = intToString(i+1) + ":" + name
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

// scrollUp scrolls the current tab up.
func (m Model) scrollUp() (tea.Model, tea.Cmd) {
	// Handle output tab (tab 0)
	if m.activeTab == 0 {
		wrappedLines := m.wrapAllOutputLines()
		height := m.layout.ScrollAreaHeight

		// Calculate max scroll offset
		maxOffset := len(wrappedLines) - height
		if maxOffset < 0 {
			maxOffset = 0
		}

		// Nothing to scroll if content fits in viewport
		if maxOffset == 0 {
			return m, nil
		}

		if m.outputTailing {
			// Unlock tail mode and position one line up from bottom
			m.outputTailing = false
			m.outputScroll = maxOffset - 1
			if m.outputScroll < 0 {
				m.outputScroll = 0
			}
		} else {
			// Already scrolling, move up one line
			if m.outputScroll > 0 {
				m.outputScroll--
			}
		}
		return m, nil
	}

	// Handle file tabs
	if len(m.tabs) <= m.activeTab {
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

// scrollDown scrolls the current tab down.
func (m Model) scrollDown() (tea.Model, tea.Cmd) {
	// Handle output tab (tab 0)
	if m.activeTab == 0 {
		// If already tailing, nothing to do (already at bottom)
		if m.outputTailing {
			return m, nil
		}

		wrappedLines := m.wrapAllOutputLines()
		height := m.layout.ScrollAreaHeight

		// Calculate max scroll offset
		maxOffset := len(wrappedLines) - height
		if maxOffset < 0 {
			maxOffset = 0
		}

		// Increment scroll offset
		m.outputScroll++

		// If we've reached or exceeded max offset, re-lock to tail mode
		if m.outputScroll >= maxOffset {
			m.outputScroll = maxOffset
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

// scrollPageUp scrolls the current tab up by a page.
func (m Model) scrollPageUp() (tea.Model, tea.Cmd) {
	// Handle output tab (tab 0)
	if m.activeTab == 0 {
		wrappedLines := m.wrapAllOutputLines()
		height := m.layout.ScrollAreaHeight

		// Calculate max scroll offset
		maxOffset := len(wrappedLines) - height
		if maxOffset < 0 {
			maxOffset = 0
		}

		// Nothing to scroll if content fits in viewport
		if maxOffset == 0 {
			return m, nil
		}

		if m.outputTailing {
			// Unlock tail mode and jump up one page from bottom
			m.outputTailing = false
			m.outputScroll = maxOffset - height
			if m.outputScroll < 0 {
				m.outputScroll = 0
			}
		} else {
			// Already scrolling, move up one page
			m.outputScroll -= height
			if m.outputScroll < 0 {
				m.outputScroll = 0
			}
		}
		return m, nil
	}

	// Handle file tabs
	if len(m.tabs) <= m.activeTab {
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

// scrollPageDown scrolls the current tab down by a page.
func (m Model) scrollPageDown() (tea.Model, tea.Cmd) {
	// Handle output tab (tab 0)
	if m.activeTab == 0 {
		// If already tailing, nothing to do (already at bottom)
		if m.outputTailing {
			return m, nil
		}

		wrappedLines := m.wrapAllOutputLines()
		height := m.layout.ScrollAreaHeight

		// Calculate max scroll offset
		maxOffset := len(wrappedLines) - height
		if maxOffset < 0 {
			maxOffset = 0
		}

		// Jump down by one page
		m.outputScroll += height

		// If we've reached or exceeded max offset, re-lock to tail mode
		if m.outputScroll >= maxOffset {
			m.outputScroll = maxOffset
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

	// Worktree panel (if worktree mode is active)
	if m.layout.WorktreePanelHeight > 0 {
		sections = append(sections, RenderDoubleBorder(m.layout.Width, m.styles.Border))
		sections = append(sections, m.renderWorktreePanel())
	}

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
	iterRatio := float64(p.Iteration) / float64(p.MaxIteration)
	costRatio := p.Cost / p.Budget

	var iterStyled, costStyled string
	if iterRatio > 0.8 {
		iterStyled = m.styles.Warning.Render(iterStr)
	} else {
		iterStyled = m.styles.Value.Render(iterStr)
	}
	if costRatio > 0.8 {
		costStyled = m.styles.Warning.Render(costStr)
	} else {
		costStyled = m.styles.Value.Render(costStr)
	}

	metrics := m.styles.Label.Render("Iteration ") + iterStyled +
		m.styles.Label.Render("  " + InnerVertical + "  ") +
		costStyled

	// Calculate padding between brand and metrics
	brandWidth := ansi.StringWidth(IconBrand + " ORBITAL")
	metricsWidth := ansi.StringWidth("Iteration " + iterStr + "  " + InnerVertical + "  " + costStr)
	padding := width - brandWidth - metricsWidth
	if padding < 1 {
		padding = 1
	}

	return m.styles.Border.Render(BoxVertical) + " " + brand + strings.Repeat(" ", padding) + metrics + " " + m.styles.Border.Render(BoxVertical)
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
			name = intToString(i+1) + ":" + name
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

// renderFileContent renders the content of a file.
func (m Model) renderFileContent(path string) string {
	height := m.layout.ScrollAreaHeight
	contentWidth := m.layout.ContentWidth()
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
			emptyLine := border + strings.Repeat(" ", contentWidth) + border
			lines = append(lines, emptyLine)
			continue
		}

		line := fileLines[lineIdx]
		lineNum := lineIdx + 1

		// Format line number (right-aligned, 5 chars)
		numStr := intToString(lineNum)
		for len(numStr) < 5 {
			numStr = " " + numStr
		}
		numStr = m.styles.Label.Render(numStr + InnerVertical)

		// Truncate long lines (ANSI-aware)
		visibleWidth := contentWidth - 6 // Account for line number column
		if ansi.StringWidth(line) > visibleWidth {
			line = ansi.Truncate(line, visibleWidth-3, "...")
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

// wrapAllOutputLines returns the cached wrapped output lines.
// The cache is maintained by updateWrappedLinesCache() which is called
// during Update() when content or width changes.
func (m Model) wrapAllOutputLines() []string {
	// If cache exists and is valid, use it
	if m.wrappedLinesCache != nil {
		return m.wrappedLinesCache
	}

	// Fallback: rebuild if cache is nil (shouldn't happen in normal operation)
	width := m.layout.ContentWidth()
	var wrappedLines []string
	m.outputLines.Iterate(func(_ int, line string) bool {
		wrapped := wrapLine(line, width)
		wrappedLines = append(wrappedLines, wrapped...)
		return true
	})
	return wrappedLines
}

// updateWrappedLinesCache rebuilds or updates the wrapped lines cache.
// Call this when content changes (new line) or width changes (resize).
func (m *Model) updateWrappedLinesCache() {
	width := m.layout.ContentWidth()
	lineCount := m.outputLines.Len()

	// Check if cache is still valid
	if m.wrappedLinesCache != nil && m.cacheWidth == width && m.cacheLineCount == lineCount {
		return
	}

	// Rebuild the cache
	var wrappedLines []string
	m.outputLines.Iterate(func(_ int, line string) bool {
		wrapped := wrapLine(line, width)
		wrappedLines = append(wrappedLines, wrapped...)
		return true
	})

	m.wrappedLinesCache = wrappedLines
	m.cacheWidth = width
	m.cacheLineCount = lineCount
}

// invalidateWrappedLinesCache clears the cache, forcing a rebuild on next access.
func (m *Model) invalidateWrappedLinesCache() {
	m.wrappedLinesCache = nil
	m.cacheWidth = 0
	m.cacheLineCount = 0
}

// renderScrollArea renders the scrolling output region.
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

	wrappedLines := m.wrapAllOutputLines()
	border := m.styles.Border.Render(BoxVertical)
	emptyLine := border + strings.Repeat(" ", contentWidth) + border

	// Empty state: show waiting message
	if len(wrappedLines) == 0 {
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

	// Determine start index based on scroll state
	var startIdx int
	if m.outputTailing {
		// Tailing: show most recent lines
		startIdx = 0
		if len(wrappedLines) > height {
			startIdx = len(wrappedLines) - height
		}
	} else {
		// Scrolling: use the scroll offset
		startIdx = m.outputScroll
		// Clamp to valid range
		maxOffset := len(wrappedLines) - height
		if maxOffset < 0 {
			maxOffset = 0
		}
		if startIdx > maxOffset {
			startIdx = maxOffset
		}
		if startIdx < 0 {
			startIdx = 0
		}
	}

	var lines []string
	for i := startIdx; i < len(wrappedLines) && len(lines) < height; i++ {
		line := wrappedLines[i]
		// Pad line to content width
		lineWidth := ansi.StringWidth(line)
		padding := contentWidth - lineWidth
		if padding < 0 {
			padding = 0
		}
		lines = append(lines, border+line+strings.Repeat(" ", padding)+border)
	}

	// Pad with empty lines if needed
	for len(lines) < height {
		lines = append(lines, emptyLine)
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
	padding := contentWidth - headerWidth - 2
	if padding < 0 {
		padding = 0
	}
	lines = append(lines, border+"  "+headerText+strings.Repeat(" ", padding)+border)

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
	if len(content) > maxLen {
		content = content[:maxLen-3] + "..."
	}

	taskContent := style.Render("  " + icon + " " + content)
	taskWidth := ansi.StringWidth("  " + icon + " " + content)
	padding := contentWidth - taskWidth
	if padding < 0 {
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
	iterRatio := float64(p.Iteration) / float64(p.MaxIteration)
	iterBar := RenderProgressBar(iterRatio, BarWidth, m.styles.Value, m.styles.Warning)
	iterLabel := m.styles.Label.Render("Iteration ")
	iterValue := m.styles.Value.Render(formatFraction(p.Iteration, p.MaxIteration))
	if iterRatio > 0.8 {
		iterValue = m.styles.Warning.Render(formatFraction(p.Iteration, p.MaxIteration))
	}

	stepStr := m.formatStep(p.StepName, p.StepPosition, p.StepTotal)
	gateStr := ""
	if p.GateRetries > 0 || p.MaxRetries > 0 {
		gateStr = m.formatGateRetries(p.GateRetries, p.MaxRetries)
	}

	line1Parts := []string{iterBar + " " + iterLabel + iterValue}
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
		line1Padding = 0
	}

	// Line 2: Budget progress bar, tokens and cost
	costRatio := p.Cost / p.Budget
	if p.Budget == 0 {
		costRatio = 0
	}
	budgetBar := RenderProgressBar(costRatio, BarWidth, m.styles.Value, m.styles.Warning)
	tokensStr := m.formatTokens(p.TokensIn, p.TokensOut)
	costStr := m.formatCost(p.Cost, p.Budget)
	line2Content := " " + budgetBar + " " + tokensStr + " " + InnerVertical + " " + costStr
	line2Width := ansi.StringWidth(line2Content)
	line2Padding := contentWidth - line2Width
	if line2Padding < 0 {
		line2Padding = 0
	}

	line1 := border + line1Content + strings.Repeat(" ", line1Padding) + border
	line2 := border + line2Content + strings.Repeat(" ", line2Padding) + border

	return line1 + "\n" + line2
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
	contentWidth := m.layout.ContentWidth()
	border := m.styles.Border.Render(BoxVertical)

	// Line 1: Spec file(s)
	specStr := m.formatPaths("Spec", s.SpecFiles)
	line1Content := " " + specStr
	line1Width := ansi.StringWidth(line1Content)
	line1Padding := contentWidth - line1Width
	if line1Padding < 0 {
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
		line2Padding = 0
	}

	line1 := border + line1Content + strings.Repeat(" ", line1Padding) + border
	line2 := border + line2Content + strings.Repeat(" ", line2Padding) + border

	return line1 + "\n" + line2
}

// renderWorktreePanel renders the worktree info panel.
func (m Model) renderWorktreePanel() string {
	w := m.worktree
	contentWidth := m.layout.ContentWidth()
	border := m.styles.Border.Render(BoxVertical)

	// Icon and label
	icon := m.styles.WorktreeLabel.Render(IconWorktree)

	// If name is available, show it prominently
	var nameStr string
	var nameWidth int
	if w.Name != "" {
		nameStr = m.styles.WorktreeLabel.Render(" Worktree: ") + m.styles.WorktreeValue.Render(w.Name)
		nameWidth = len(" Worktree: ") + len(w.Name)
	} else {
		// Fallback to path if no name
		path := w.Path
		maxPathLen := 40
		if len(path) > maxPathLen {
			path = "..." + path[len(path)-maxPathLen+3:]
		}
		nameStr = m.styles.WorktreeLabel.Render(" Worktree: ") + m.styles.WorktreeValue.Render(path)
		nameWidth = len(" Worktree: ") + len(path)
	}

	// Branch
	branchLabel := m.styles.Label.Render(" " + InnerVertical + " Branch: ")
	branchStr := m.styles.WorktreeValue.Render(w.Branch)

	lineContent := " " + icon + nameStr + branchLabel + branchStr
	lineWidth := 1 + 1 + nameWidth + len(" "+InnerVertical+" Branch: ") + len(w.Branch)
	padding := contentWidth - lineWidth
	if padding < 0 {
		padding = 0
	}

	return border + lineContent + strings.Repeat(" ", padding) + border
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
// When the buffer is full, the oldest line is evicted automatically.
// This also updates the wrapped lines cache incrementally when possible.
func (m *Model) AppendOutput(line string) {
	m.outputLines.Push(line)
	m.appendLineToCache(line)
}

// appendLineToCache updates the wrapped lines cache after a new line is pushed.
// It attempts incremental update if possible, otherwise rebuilds the cache.
func (m *Model) appendLineToCache(line string) {
	if m.wrappedLinesCache != nil && m.cacheWidth == m.layout.ContentWidth() {
		// Check if ring buffer wrapped (evicted old lines)
		if m.outputLines.Len() == m.cacheLineCount+1 {
			// Normal case: just append the new wrapped lines
			wrapped := wrapLine(line, m.cacheWidth)
			m.wrappedLinesCache = append(m.wrappedLinesCache, wrapped...)
			m.cacheLineCount = m.outputLines.Len()
		} else {
			// Ring buffer wrapped, need full rebuild
			m.invalidateWrappedLinesCache()
			m.updateWrappedLinesCache()
		}
	} else if m.ready {
		// No valid cache, build it
		m.updateWrappedLinesCache()
	}
}

// ClearOutput clears the output buffer and cache.
func (m *Model) ClearOutput() {
	m.outputLines.Clear()
	m.invalidateWrappedLinesCache()
}
