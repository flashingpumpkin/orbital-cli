package tui

import (
	"github.com/flashingpumpkin/orbital/internal/tasks"
)

// TaskTracker is an alias to the shared tasks.Tracker type for TUI use.
type TaskTracker = tasks.Tracker

// NewTaskTracker creates a new TaskTracker using the shared tasks package.
func NewTaskTracker() *TaskTracker {
	return tasks.NewTracker()
}
