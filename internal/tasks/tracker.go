// Package tasks provides task tracking functionality for orbit.
// This package can be used by both TUI and non-TUI modes.
package tasks

import (
	"encoding/json"
	"sync"
)

// Task represents a single task item.
type Task struct {
	ID         string
	Content    string
	Status     string // "pending", "in_progress", "completed"
	ActiveForm string
}

// Tracker maintains a map of tasks by ID and processes tool use events.
type Tracker struct {
	mu    sync.RWMutex
	tasks map[string]*Task
	order []string // Preserves insertion order
}

// NewTracker creates a new Tracker.
func NewTracker() *Tracker {
	return &Tracker{
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

// todoWriteInput represents the JSON input for TodoWrite tool.
type todoWriteInput struct {
	Todos []todoItem `json:"todos"`
}

// todoItem represents a single todo item in TodoWrite.
type todoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

// IsTaskTool returns true if the tool name is a task-related tool.
func IsTaskTool(toolName string) bool {
	switch toolName {
	case "TaskCreate", "TaskUpdate", "TodoWrite":
		return true
	default:
		return false
	}
}

// ProcessToolUse processes a tool use event and returns the updated task list if changed.
// Returns nil if the tool use is not task-related or if parsing fails.
func (t *Tracker) ProcessToolUse(toolName, input string) []Task {
	switch toolName {
	case "TaskCreate":
		return t.handleTaskCreate(input)
	case "TaskUpdate":
		return t.handleTaskUpdate(input)
	case "TodoWrite":
		return t.handleTodoWrite(input)
	default:
		return nil
	}
}

// handleTaskCreate processes a TaskCreate tool use.
func (t *Tracker) handleTaskCreate(input string) []Task {
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
func (t *Tracker) handleTaskUpdate(input string) []Task {
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

// handleTodoWrite processes a TodoWrite tool use (replaces all tasks).
func (t *Tracker) handleTodoWrite(input string) []Task {
	var write todoWriteInput
	if err := json.Unmarshal([]byte(input), &write); err != nil {
		return nil
	}

	if len(write.Todos) == 0 {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Clear existing tasks and replace with new ones
	t.tasks = make(map[string]*Task)
	t.order = make([]string, 0, len(write.Todos))

	for i, todo := range write.Todos {
		if todo.Content == "" {
			continue
		}

		id := intToString(i + 1)
		task := &Task{
			ID:         id,
			Content:    todo.Content,
			Status:     todo.Status,
			ActiveForm: todo.ActiveForm,
		}
		if task.Status == "" {
			task.Status = "pending"
		}

		t.tasks[id] = task
		t.order = append(t.order, id)
	}

	return t.toSlice()
}

// toSlice returns the tasks as a slice in insertion order.
// Must be called with lock held.
func (t *Tracker) toSlice() []Task {
	result := make([]Task, 0, len(t.order))
	for _, id := range t.order {
		if task, exists := t.tasks[id]; exists {
			result = append(result, *task)
		}
	}
	return result
}

// GetTasks returns the current task list.
func (t *Tracker) GetTasks() []Task {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.toSlice()
}

// GetSummary returns a summary of task counts by status.
func (t *Tracker) GetSummary() Summary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	summary := Summary{
		Tasks: t.toSlice(),
	}

	for _, task := range t.tasks {
		summary.Total++
		switch task.Status {
		case "completed":
			summary.Completed++
		case "in_progress":
			summary.InProgress++
		default:
			summary.Pending++
		}
	}

	return summary
}

// Summary contains task count statistics.
type Summary struct {
	Total      int
	Completed  int
	InProgress int
	Pending    int
	Tasks      []Task
}

// Clear removes all tasks.
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tasks = make(map[string]*Task)
	t.order = make([]string, 0)
}

// intToString converts an integer to a string without fmt dependency.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}
	var result []byte
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	return string(result)
}
