// Package tui provides the terminal user interface for orbit using bubbletea.
package tui

// MinTerminalWidth is the minimum supported terminal width.
const MinTerminalWidth = 80

// MinTerminalHeight is the minimum supported terminal height.
const MinTerminalHeight = 24

// Panel heights (number of lines)
const (
	// HeaderPanelHeight is the height of the header panel (brand + metrics).
	HeaderPanelHeight = 1

	// ProgressPanelHeight is the height of the progress bar panel (iteration, budget, context).
	ProgressPanelHeight = 3

	// SessionPanelHeight is the height of the session info panel (spec, notes, state paths).
	SessionPanelHeight = 2

	// TaskPanelMaxHeight is the maximum height for the task list panel.
	TaskPanelMaxHeight = 6

	// TabBarHeight is the height of the tab bar at the top.
	TabBarHeight = 1

	// HelpBarHeight is the height of the help bar at the bottom (outside main frame).
	HelpBarHeight = 1

	// BorderHeight is the total height used by horizontal borders between panels.
	// Includes: top border, after header, after tab bar, before tasks, after tasks,
	// before progress, after progress, before session, bottom border.
	BorderHeight = 6
)

// Layout represents the calculated dimensions for each UI region.
type Layout struct {
	// Total terminal dimensions
	Width  int
	Height int

	// HeaderPanel is the brand + metrics header
	HeaderPanelHeight int

	// TabBar is the tab bar region at the top
	TabBarHeight int

	// ScrollArea is the output region at the top
	ScrollAreaHeight int

	// TaskPanel is the task list region (variable height, max 6)
	TaskPanelHeight int

	// ProgressPanel is the metrics region
	ProgressPanelHeight int

	// SessionPanel is the file paths region
	SessionPanelHeight int

	// HelpBar is the help text region at the bottom (outside main frame)
	HelpBarHeight int

	// TooSmall indicates the terminal is below minimum size
	TooSmall bool

	// TooSmallMessage is shown when terminal is too small
	TooSmallMessage string
}

// CalculateLayout computes the layout based on terminal dimensions and task count.
func CalculateLayout(width, height, taskCount int) Layout {
	layout := Layout{
		Width:               width,
		Height:              height,
		HeaderPanelHeight:   HeaderPanelHeight,
		TabBarHeight:        TabBarHeight,
		ProgressPanelHeight: ProgressPanelHeight,
		SessionPanelHeight:  SessionPanelHeight,
		HelpBarHeight:       HelpBarHeight,
	}

	// Check minimum width
	if width < MinTerminalWidth {
		layout.TooSmall = true
		layout.TooSmallMessage = "Terminal too narrow. Minimum width: 80 columns."
		return layout
	}

	// Check minimum height
	if height < MinTerminalHeight {
		layout.TooSmall = true
		layout.TooSmallMessage = "Terminal too short. Minimum height: 24 rows."
		return layout
	}

	// Calculate task panel height (variable, 0 to max)
	if taskCount == 0 {
		layout.TaskPanelHeight = 0
	} else if taskCount <= TaskPanelMaxHeight {
		layout.TaskPanelHeight = taskCount + 1 // +1 for header
	} else {
		layout.TaskPanelHeight = TaskPanelMaxHeight + 1 // +1 for header with scroll indicator
	}

	// Calculate fixed panel total
	borderCount := BorderHeight
	// Add extra border if task panel is visible
	if layout.TaskPanelHeight > 0 {
		borderCount++
	}
	fixedHeight := layout.HeaderPanelHeight + layout.TabBarHeight + layout.TaskPanelHeight + layout.ProgressPanelHeight + layout.SessionPanelHeight + layout.HelpBarHeight + borderCount

	// Remaining space goes to scroll area
	layout.ScrollAreaHeight = height - fixedHeight

	// If scroll area would be too small, collapse task panel
	if layout.ScrollAreaHeight < 4 && layout.TaskPanelHeight > 0 {
		layout.TaskPanelHeight = 0
		borderCount-- // Remove task panel border
		fixedHeight = layout.HeaderPanelHeight + layout.TabBarHeight + layout.ProgressPanelHeight + layout.SessionPanelHeight + layout.HelpBarHeight + borderCount
		layout.ScrollAreaHeight = height - fixedHeight
	}

	// Final check: if still too cramped, mark as too small
	if layout.ScrollAreaHeight < 2 {
		layout.TooSmall = true
		layout.TooSmallMessage = "Terminal too short to display UI."
		return layout
	}

	return layout
}

// ContentWidth returns the usable width inside panels (accounting for borders).
func (l Layout) ContentWidth() int {
	// Account for left and right borders
	return l.Width - 2
}

// TasksVisible returns the number of tasks that can be displayed.
func (l Layout) TasksVisible() int {
	if l.TaskPanelHeight <= 1 {
		return 0
	}
	return l.TaskPanelHeight - 1 // -1 for header
}

// HasTaskOverflow returns true if there are more tasks than can be displayed.
func (l Layout) HasTaskOverflow(taskCount int) bool {
	return taskCount > l.TasksVisible()
}
