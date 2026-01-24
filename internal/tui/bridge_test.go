package tui

import (
	"testing"
)

func TestExtractJSONField(t *testing.T) {
	tests := []struct {
		name     string
		jsonStr  string
		field    string
		expected string
	}{
		{
			name:     "simple field",
			jsonStr:  `{"file_path": "/some/path"}`,
			field:    "file_path",
			expected: "/some/path",
		},
		{
			name:     "field with spaces",
			jsonStr:  `{"subject": "Set up auth middleware"}`,
			field:    "subject",
			expected: "Set up auth middleware",
		},
		{
			name:     "field not found",
			jsonStr:  `{"other": "value"}`,
			field:    "file_path",
			expected: "",
		},
		{
			name:     "empty json",
			jsonStr:  `{}`,
			field:    "file_path",
			expected: "",
		},
		{
			name:     "multiple fields",
			jsonStr:  `{"taskId": "1", "status": "completed"}`,
			field:    "status",
			expected: "completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONField(tt.jsonStr, tt.field)
			if got != tt.expected {
				t.Errorf("extractJSONField(%q, %q) = %q, want %q", tt.jsonStr, tt.field, got, tt.expected)
			}
		})
	}
}

func TestShortenPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/a/b", "/a/b"},
		{"file.go", "file.go"},
		{"/a/b/c", ".../b/c"},
		{"/Users/test/Projects/project/internal/tui/model.go", ".../tui/model.go"},
	}

	for _, tt := range tests {
		got := shortenPath(tt.path)
		if got != tt.expected {
			t.Errorf("shortenPath(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestFormatToolSummary(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    string
		expected string
	}{
		{
			name:     "Read tool",
			toolName: "Read",
			input:    `{"file_path": "/some/dir/file.go"}`,
			expected: " .../dir/file.go",
		},
		{
			name:     "Glob tool",
			toolName: "Glob",
			input:    `{"pattern": "**/*.go"}`,
			expected: " **/*.go",
		},
		{
			name:     "Grep tool",
			toolName: "Grep",
			input:    `{"pattern": "func Test"}`,
			expected: " func Test",
		},
		{
			name:     "TaskCreate",
			toolName: "TaskCreate",
			input:    `{"subject": "Implement feature"}`,
			expected: " Implement feature",
		},
		{
			name:     "TaskUpdate",
			toolName: "TaskUpdate",
			input:    `{"taskId": "1", "status": "completed"}`,
			expected: " #1 -> completed",
		},
		{
			name:     "Unknown tool",
			toolName: "Unknown",
			input:    `{"foo": "bar"}`,
			expected: "",
		},
		{
			name:     "Empty input",
			toolName: "Read",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatToolSummary(tt.toolName, tt.input)
			if got != tt.expected {
				t.Errorf("formatToolSummary(%q, %q) = %q, want %q", tt.toolName, tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
	}

	for _, tt := range tests {
		got := formatInt(tt.n)
		if got != tt.expected {
			t.Errorf("formatInt(%d) = %q, want %q", tt.n, got, tt.expected)
		}
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		f        float64
		expected string
	}{
		{0.0, "0.0000"},
		{1.5, "1.5000"},
		{0.1234, "0.1234"},
		{10.05, "10.0500"},
	}

	for _, tt := range tests {
		got := formatFloat(tt.f)
		if got != tt.expected {
			t.Errorf("formatFloat(%f) = %q, want %q", tt.f, got, tt.expected)
		}
	}
}

func TestNewBridge(t *testing.T) {
	tracker := NewTaskTracker()
	// We can't easily test with a real tea.Program, so just test creation
	bridge := NewBridge(nil, tracker)

	if bridge.tracker != tracker {
		t.Error("expected tracker to be set")
	}

	if bridge.parser == nil {
		t.Error("expected parser to be created")
	}
}

func TestBridgeGetTracker(t *testing.T) {
	tracker := NewTaskTracker()
	bridge := NewBridge(nil, tracker)

	if bridge.GetTracker() != tracker {
		t.Error("GetTracker should return the tracker")
	}
}

func TestBridgeGetParser(t *testing.T) {
	tracker := NewTaskTracker()
	bridge := NewBridge(nil, tracker)

	if bridge.GetParser() == nil {
		t.Error("GetParser should return a non-nil parser")
	}
}

func TestCleanToolResult(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "file count",
			content:  "6 files",
			expected: "6 files",
		},
		{
			name:     "no files",
			content:  "No files matched",
			expected: "No files matched",
		},
		{
			name:     "path shortening",
			content:  "/Users/test/Projects/project/internal/tui/model.go",
			expected: ".../tui/model.go",
		},
		{
			name:     "skill launch",
			content:  "Launching skill: commit",
			expected: "Launching skill: commit",
		},
		{
			name:     "todos confirmation",
			content:  "Todos have been updated",
			expected: "todos updated",
		},
		{
			name:     "file content - skip",
			content:  "1→package main",
			expected: "",
		},
		{
			name:     "file content with spaces - skip",
			content:  "     1→package main",
			expected: "",
		},
		{
			name:     "long content - skip",
			content:  "This is a very long piece of content that exceeds the 80 character limit and should be skipped entirely",
			expected: "",
		},
		{
			name:     "short content",
			content:  "Success",
			expected: "Success",
		},
		{
			name:     "multiline content - skip",
			content:  "line1\nline2",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanToolResult(tt.content)
			if got != tt.expected {
				t.Errorf("cleanToolResult(%q) = %q, want %q", tt.content, got, tt.expected)
			}
		})
	}
}

func TestFormatTodoWriteInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name: "single pending task",
			input: `{"todos":[{"content":"Run tests","status":"pending"}]}`,
			contains: []string{"○", "Run tests"},
		},
		{
			name: "single in_progress task",
			input: `{"todos":[{"content":"Fix bug","status":"in_progress"}]}`,
			contains: []string{"▶", "Fix bug"},
		},
		{
			name: "single completed task",
			input: `{"todos":[{"content":"Write code","status":"completed"}]}`,
			contains: []string{"✓", "Write code"},
		},
		{
			name: "multiple tasks",
			input: `{"todos":[{"content":"Task 1","status":"completed"},{"content":"Task 2","status":"in_progress"},{"content":"Task 3","status":"pending"}]}`,
			contains: []string{"✓", "Task 1", "▶", "Task 2", "○", "Task 3"},
		},
		{
			name: "long content truncation",
			input: `{"todos":[{"content":"This is a very long task content that should be truncated at sixty characters","status":"pending"}]}`,
			contains: []string{"...", "This is a very long task content that should be truncated at"},
		},
		{
			name: "empty todos",
			input: `{"todos":[]}`,
			contains: nil, // expects empty result
		},
		{
			name: "invalid json",
			input: `not json`,
			contains: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTodoWriteInput(tt.input)
			if tt.contains == nil {
				if got != "" {
					t.Errorf("formatTodoWriteInput() = %q, want empty string", got)
				}
				return
			}
			for _, want := range tt.contains {
				if !containsString(got, want) {
					t.Errorf("formatTodoWriteInput() = %q, missing %q", got, want)
				}
			}
		})
	}
}

// containsString checks if s contains substr (helper for ANSI-containing strings)
func containsString(s, substr string) bool {
	// Simple substring check - works with ANSI codes present
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFormatToolSummaryTodoWrite(t *testing.T) {
	input := `{"todos":[{"content":"Read spec file","status":"in_progress"},{"content":"Implement feature","status":"pending"}]}`
	got := formatToolSummary("TodoWrite", input)

	// Should contain newlines for multi-line output
	if !containsString(got, "\n") {
		t.Error("TodoWrite summary should be multi-line")
	}

	// Should contain task indicators
	expectedIndicators := []string{"▶", "○"}
	for _, indicator := range expectedIndicators {
		if !containsString(got, indicator) {
			t.Errorf("TodoWrite summary missing indicator %q", indicator)
		}
	}

	// Should contain task content
	if !containsString(got, "Read spec file") {
		t.Error("TodoWrite summary missing first task content")
	}
	if !containsString(got, "Implement feature") {
		t.Error("TodoWrite summary missing second task content")
	}

	// Should have proper indentation (6 spaces for task lines)
	if !containsString(got, "      ") {
		t.Error("TodoWrite summary missing 6-space indentation")
	}
}
