package tui

import (
	"testing"
)

// Tests for the TUI's TaskTracker wrapper.
// The actual task tracking logic is tested in internal/tasks/tracker_test.go.
// These tests verify that the TUI wrapper works correctly.

func TestNewTaskTracker(t *testing.T) {
	tracker := NewTaskTracker()

	tasks := tracker.GetTasks()
	if len(tasks) != 0 {
		t.Errorf("expected empty tasks, got %d", len(tasks))
	}
}

func TestTaskTrackerProcessTaskCreate(t *testing.T) {
	tracker := NewTaskTracker()

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

func TestTaskTrackerProcessTaskUpdate(t *testing.T) {
	tracker := NewTaskTracker()

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

func TestTaskTrackerProcessTaskUpdateComplete(t *testing.T) {
	tracker := NewTaskTracker()

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

func TestTaskTrackerProcessUnknownTool(t *testing.T) {
	tracker := NewTaskTracker()

	tasks := tracker.ProcessToolUse("Read", `{"file_path": "/some/file"}`)

	if tasks != nil {
		t.Errorf("expected nil for unknown tool, got %v", tasks)
	}
}

func TestTaskTrackerProcessInvalidJSON(t *testing.T) {
	tracker := NewTaskTracker()

	tasks := tracker.ProcessToolUse("TaskCreate", `{invalid json}`)

	if tasks != nil {
		t.Errorf("expected nil for invalid JSON, got %v", tasks)
	}
}

func TestTaskTrackerProcessMissingSubject(t *testing.T) {
	tracker := NewTaskTracker()

	tasks := tracker.ProcessToolUse("TaskCreate", `{"description": "No subject"}`)

	if tasks != nil {
		t.Errorf("expected nil for missing subject, got %v", tasks)
	}
}

func TestTaskTrackerProcessUpdateNonexistent(t *testing.T) {
	tracker := NewTaskTracker()

	tasks := tracker.ProcessToolUse("TaskUpdate", `{"taskId": "999", "status": "completed"}`)

	if tasks != nil {
		t.Errorf("expected nil for nonexistent task, got %v", tasks)
	}
}

func TestTaskTrackerMultipleTasks(t *testing.T) {
	tracker := NewTaskTracker()

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

func TestTaskTrackerClear(t *testing.T) {
	tracker := NewTaskTracker()

	tracker.ProcessToolUse("TaskCreate", `{"subject": "Task 1"}`)
	tracker.ProcessToolUse("TaskCreate", `{"subject": "Task 2"}`)

	tracker.Clear()
	tasks := tracker.GetTasks()

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after clear, got %d", len(tasks))
	}
}

func TestTaskTrackerUpdateSubject(t *testing.T) {
	tracker := NewTaskTracker()

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
