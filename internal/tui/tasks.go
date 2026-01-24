package tui

import (
	"encoding/json"
	"sync"
)

// TaskTracker maintains a map of tasks by ID and processes tool use events.
type TaskTracker struct {
	mu    sync.RWMutex
	tasks map[string]*Task
	order []string // Preserves insertion order
}

// NewTaskTracker creates a new TaskTracker.
func NewTaskTracker() *TaskTracker {
	return &TaskTracker{
		tasks: make(map[string]*Task),
		order: make([]string, 0),
	}
}

// taskCreateInput represents the JSON input for TaskCreate tool.
type taskCreateInput struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	ActiveForm  string `json:"activeForm"`
}

// taskUpdateInput represents the JSON input for TaskUpdate tool.
type taskUpdateInput struct {
	TaskID      string  `json:"taskId"`
	Status      *string `json:"status,omitempty"`
	Subject     *string `json:"subject,omitempty"`
	Description *string `json:"description,omitempty"`
	ActiveForm  *string `json:"activeForm,omitempty"`
}

// ProcessToolUse processes a tool use event and returns the updated task list if changed.
// Returns nil if the tool use is not task-related or if parsing fails.
func (t *TaskTracker) ProcessToolUse(toolName, input string) []Task {
	switch toolName {
	case "TaskCreate":
		return t.handleTaskCreate(input)
	case "TaskUpdate":
		return t.handleTaskUpdate(input)
	default:
		return nil
	}
}

// handleTaskCreate processes a TaskCreate tool use.
func (t *TaskTracker) handleTaskCreate(input string) []Task {
	var create taskCreateInput
	if err := json.Unmarshal([]byte(input), &create); err != nil {
		return nil
	}

	if create.Subject == "" {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Generate a simple ID based on order
	id := intToString(len(t.order) + 1)

	task := &Task{
		ID:         id,
		Content:    create.Subject,
		Status:     "pending",
		ActiveForm: create.ActiveForm,
	}

	t.tasks[id] = task
	t.order = append(t.order, id)

	return t.toSlice()
}

// handleTaskUpdate processes a TaskUpdate tool use.
func (t *TaskTracker) handleTaskUpdate(input string) []Task {
	var update taskUpdateInput
	if err := json.Unmarshal([]byte(input), &update); err != nil {
		return nil
	}

	if update.TaskID == "" {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	task, exists := t.tasks[update.TaskID]
	if !exists {
		return nil
	}

	if update.Status != nil {
		task.Status = *update.Status
	}
	if update.Subject != nil {
		task.Content = *update.Subject
	}
	if update.ActiveForm != nil {
		task.ActiveForm = *update.ActiveForm
	}

	return t.toSlice()
}

// toSlice returns the tasks as a slice in insertion order.
// Must be called with lock held.
func (t *TaskTracker) toSlice() []Task {
	result := make([]Task, 0, len(t.order))
	for _, id := range t.order {
		if task, exists := t.tasks[id]; exists {
			result = append(result, *task)
		}
	}
	return result
}

// GetTasks returns the current task list.
func (t *TaskTracker) GetTasks() []Task {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.toSlice()
}

// Clear removes all tasks.
func (t *TaskTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tasks = make(map[string]*Task)
	t.order = make([]string, 0)
}
