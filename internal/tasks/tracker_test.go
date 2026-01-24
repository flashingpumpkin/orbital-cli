package tasks

import (
	"testing"
)

func TestNewTracker(t *testing.T) {
	tracker := NewTracker()

	tasks := tracker.GetTasks()
	if len(tasks) != 0 {
		t.Errorf("expected empty tasks, got %d", len(tasks))
	}
}

func TestTrackerProcessTaskCreate(t *testing.T) {
	tracker := NewTracker()

	input := `{"subject": "Set up auth middleware", "description": "Create middleware", "activeForm": "Setting up auth"}`
	tasks := tracker.ProcessToolUse("TaskCreate", input)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.ID != "1" {
		t.Errorf("expected ID '1', got %q", task.ID)
	}
	if task.Content != "Set up auth middleware" {
		t.Errorf("expected content 'Set up auth middleware', got %q", task.Content)
	}
	if task.Status != "pending" {
		t.Errorf("expected status 'pending', got %q", task.Status)
	}
	if task.ActiveForm != "Setting up auth" {
		t.Errorf("expected activeForm 'Setting up auth', got %q", task.ActiveForm)
	}
}

func TestTrackerProcessTaskUpdate(t *testing.T) {
	tracker := NewTracker()

	// Create a task first
	createInput := `{"subject": "Implement login", "description": "Add login endpoint"}`
	tracker.ProcessToolUse("TaskCreate", createInput)

	// Update the task
	updateInput := `{"taskId": "1", "status": "in_progress"}`
	tasks := tracker.ProcessToolUse("TaskUpdate", updateInput)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %q", tasks[0].Status)
	}
}

func TestTrackerProcessTaskUpdateComplete(t *testing.T) {
	tracker := NewTracker()

	// Create a task
	tracker.ProcessToolUse("TaskCreate", `{"subject": "Task 1"}`)

	// Complete the task
	updateInput := `{"taskId": "1", "status": "completed"}`
	tasks := tracker.ProcessToolUse("TaskUpdate", updateInput)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].Status != "completed" {
		t.Errorf("expected status 'completed', got %q", tasks[0].Status)
	}
}

func TestTrackerProcessUnknownTool(t *testing.T) {
	tracker := NewTracker()

	tasks := tracker.ProcessToolUse("Read", `{"file_path": "/some/file"}`)

	if tasks != nil {
		t.Errorf("expected nil for unknown tool, got %v", tasks)
	}
}

func TestTrackerProcessInvalidJSON(t *testing.T) {
	tracker := NewTracker()

	tasks := tracker.ProcessToolUse("TaskCreate", `{invalid json}`)

	if tasks != nil {
		t.Errorf("expected nil for invalid JSON, got %v", tasks)
	}
}

func TestTrackerProcessMissingSubject(t *testing.T) {
	tracker := NewTracker()

	tasks := tracker.ProcessToolUse("TaskCreate", `{"description": "No subject"}`)

	if tasks != nil {
		t.Errorf("expected nil for missing subject, got %v", tasks)
	}
}

func TestTrackerProcessUpdateNonexistent(t *testing.T) {
	tracker := NewTracker()

	tasks := tracker.ProcessToolUse("TaskUpdate", `{"taskId": "999", "status": "completed"}`)

	if tasks != nil {
		t.Errorf("expected nil for nonexistent task, got %v", tasks)
	}
}

func TestTrackerMultipleTasks(t *testing.T) {
	tracker := NewTracker()

	tracker.ProcessToolUse("TaskCreate", `{"subject": "Task 1"}`)
	tracker.ProcessToolUse("TaskCreate", `{"subject": "Task 2"}`)
	tasks := tracker.ProcessToolUse("TaskCreate", `{"subject": "Task 3"}`)

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Check order is preserved
	if tasks[0].Content != "Task 1" {
		t.Errorf("expected first task 'Task 1', got %q", tasks[0].Content)
	}
	if tasks[1].Content != "Task 2" {
		t.Errorf("expected second task 'Task 2', got %q", tasks[1].Content)
	}
	if tasks[2].Content != "Task 3" {
		t.Errorf("expected third task 'Task 3', got %q", tasks[2].Content)
	}
}

func TestTrackerClear(t *testing.T) {
	tracker := NewTracker()

	tracker.ProcessToolUse("TaskCreate", `{"subject": "Task 1"}`)
	tracker.ProcessToolUse("TaskCreate", `{"subject": "Task 2"}`)

	tracker.Clear()
	tasks := tracker.GetTasks()

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after clear, got %d", len(tasks))
	}
}

func TestTrackerUpdateSubject(t *testing.T) {
	tracker := NewTracker()

	tracker.ProcessToolUse("TaskCreate", `{"subject": "Original subject"}`)

	updateInput := `{"taskId": "1", "subject": "Updated subject"}`
	tasks := tracker.ProcessToolUse("TaskUpdate", updateInput)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].Content != "Updated subject" {
		t.Errorf("expected content 'Updated subject', got %q", tasks[0].Content)
	}
}

func TestTrackerTodoWrite(t *testing.T) {
	tracker := NewTracker()

	input := `{"todos": [
		{"content": "Task 1", "status": "completed", "activeForm": "Doing task 1"},
		{"content": "Task 2", "status": "in_progress", "activeForm": "Doing task 2"},
		{"content": "Task 3", "status": "pending", "activeForm": "Doing task 3"}
	]}`
	tasks := tracker.ProcessToolUse("TodoWrite", input)

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	if tasks[0].Content != "Task 1" {
		t.Errorf("expected first task 'Task 1', got %q", tasks[0].Content)
	}
	if tasks[0].Status != "completed" {
		t.Errorf("expected first task status 'completed', got %q", tasks[0].Status)
	}

	if tasks[1].Content != "Task 2" {
		t.Errorf("expected second task 'Task 2', got %q", tasks[1].Content)
	}
	if tasks[1].Status != "in_progress" {
		t.Errorf("expected second task status 'in_progress', got %q", tasks[1].Status)
	}

	if tasks[2].Content != "Task 3" {
		t.Errorf("expected third task 'Task 3', got %q", tasks[2].Content)
	}
	if tasks[2].Status != "pending" {
		t.Errorf("expected third task status 'pending', got %q", tasks[2].Status)
	}
}

func TestTrackerTodoWriteReplacesExisting(t *testing.T) {
	tracker := NewTracker()

	// Create some tasks first
	tracker.ProcessToolUse("TaskCreate", `{"subject": "Old task 1"}`)
	tracker.ProcessToolUse("TaskCreate", `{"subject": "Old task 2"}`)

	// TodoWrite should replace all tasks
	input := `{"todos": [{"content": "New task 1", "status": "pending"}]}`
	tasks := tracker.ProcessToolUse("TodoWrite", input)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after TodoWrite, got %d", len(tasks))
	}

	if tasks[0].Content != "New task 1" {
		t.Errorf("expected 'New task 1', got %q", tasks[0].Content)
	}
}

func TestTrackerTodoWriteEmptyTodos(t *testing.T) {
	tracker := NewTracker()

	tasks := tracker.ProcessToolUse("TodoWrite", `{"todos": []}`)

	if tasks != nil {
		t.Errorf("expected nil for empty todos, got %v", tasks)
	}
}

func TestTrackerGetSummary(t *testing.T) {
	tracker := NewTracker()

	input := `{"todos": [
		{"content": "Task 1", "status": "completed"},
		{"content": "Task 2", "status": "completed"},
		{"content": "Task 3", "status": "in_progress"},
		{"content": "Task 4", "status": "pending"},
		{"content": "Task 5", "status": "pending"}
	]}`
	tracker.ProcessToolUse("TodoWrite", input)

	summary := tracker.GetSummary()

	if summary.Total != 5 {
		t.Errorf("expected total 5, got %d", summary.Total)
	}
	if summary.Completed != 2 {
		t.Errorf("expected completed 2, got %d", summary.Completed)
	}
	if summary.InProgress != 1 {
		t.Errorf("expected in_progress 1, got %d", summary.InProgress)
	}
	if summary.Pending != 2 {
		t.Errorf("expected pending 2, got %d", summary.Pending)
	}
	if len(summary.Tasks) != 5 {
		t.Errorf("expected 5 tasks in summary, got %d", len(summary.Tasks))
	}
}

func TestIsTaskTool(t *testing.T) {
	tests := []struct {
		toolName string
		want     bool
	}{
		{"TaskCreate", true},
		{"TaskUpdate", true},
		{"TodoWrite", true},
		{"Read", false},
		{"Write", false},
		{"Bash", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			got := IsTaskTool(tt.toolName)
			if got != tt.want {
				t.Errorf("IsTaskTool(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestTrackerTodoWriteDefaultStatus(t *testing.T) {
	tracker := NewTracker()

	// TodoWrite with no status should default to pending
	input := `{"todos": [{"content": "Task with no status"}]}`
	tasks := tracker.ProcessToolUse("TodoWrite", input)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].Status != "pending" {
		t.Errorf("expected status 'pending', got %q", tasks[0].Status)
	}
}
