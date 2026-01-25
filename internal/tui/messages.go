package tui

// OutputLineMsg represents a new formatted output line to display.
type OutputLineMsg string

// TasksMsg represents an updated task list.
type TasksMsg []Task

// ProgressMsg represents updated progress and statistics.
type ProgressMsg ProgressInfo

// SessionMsg represents session information (typically set once at startup).
type SessionMsg SessionInfo
