package tui

// OutputLineMsg represents a new formatted output line to display.
type OutputLineMsg string

// TasksMsg represents an updated task list.
type TasksMsg []Task

// ProgressMsg represents updated progress and statistics.
type ProgressMsg ProgressInfo

// SessionMsg represents session information (typically set once at startup).
type SessionMsg SessionInfo

// WorktreeInfo contains information about the active worktree.
type WorktreeInfo struct {
	Name   string
	Path   string
	Branch string
}

// WorktreeMsg represents worktree information update.
type WorktreeMsg WorktreeInfo
