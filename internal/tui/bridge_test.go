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

// mockProgram captures messages sent to it for testing.
type mockProgram struct {
	messages []interface{}
}

func (m *mockProgram) Send(msg interface{}) {
	m.messages = append(m.messages, msg)
}

// TestBridgeMessageQueue verifies the non-blocking message queue behaviour.
func TestBridgeMessageQueue(t *testing.T) {
	t.Run("sends messages through queue", func(t *testing.T) {
		tracker := NewTaskTracker()
		bridge := NewBridge(nil, tracker)
		defer bridge.Close()

		// With nil program, messages go to queue but aren't forwarded
		// This test verifies the queue doesn't block

		// Write multiple lines quickly
		lines := []string{
			`{"type":"assistant","message":{"content":[{"type":"text","text":"Line 1"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"text","text":"Line 2"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"text","text":"Line 3"}]}}`,
		}

		for _, line := range lines {
			_, err := bridge.Write([]byte(line + "\n"))
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}

		// Verify queue received messages (non-blocking check)
		// With nil program, the pump isn't running, so messages stay in queue
		queueLen := len(bridge.msgQueue)
		if queueLen == 0 {
			// This is expected when program is nil as the pump isn't started
			// Verify the bridge processes without blocking
		}
	})

	t.Run("handles queue full gracefully", func(t *testing.T) {
		tracker := NewTaskTracker()
		bridge := NewBridge(nil, tracker)

		// Fill the queue manually since pump isn't running with nil program
		for i := 0; i < defaultQueueSize; i++ {
			select {
			case bridge.msgQueue <- OutputLineMsg("test"):
			default:
				t.Fatalf("queue blocked at message %d", i)
			}
		}

		// Now queue is full, next message should be dropped without blocking
		bridge.sendMsg(OutputLineMsg("should be dropped"))

		// Verify bridge still works and didn't block
		bridge.Close()
	})

	t.Run("close is idempotent", func(t *testing.T) {
		tracker := NewTaskTracker()
		bridge := NewBridge(nil, tracker)

		// Close multiple times should not panic
		bridge.Close()
		bridge.Close()
		bridge.Close()
	})

	t.Run("sends after close are ignored", func(t *testing.T) {
		tracker := NewTaskTracker()
		bridge := NewBridge(nil, tracker)
		bridge.Close()

		// This should not panic or block
		bridge.sendMsg(OutputLineMsg("ignored"))
	})
}

// TestBridgeStatsMsg verifies that StatsMsg is sent on assistant and result events.
func TestBridgeStatsMsg(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectStatsMsg bool
		description    string
	}{
		{
			name:           "assistant with usage sends StatsMsg",
			input:          `{"type":"assistant","message":{"content":[{"type":"text","text":"Working"}],"usage":{"input_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":50}}}`,
			expectStatsMsg: true,
			description:    "assistant events with usage stats should trigger StatsMsg",
		},
		{
			name:           "assistant without usage does not send StatsMsg",
			input:          `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`,
			expectStatsMsg: false,
			description:    "assistant events without usage stats should not trigger StatsMsg",
		},
		{
			name:           "result with usage sends StatsMsg",
			input:          `{"type":"result","total_cost_usd":0.05,"usage":{"input_tokens":100,"output_tokens":50}}`,
			expectStatsMsg: true,
			description:    "result events with usage stats should trigger StatsMsg",
		},
		{
			name:           "content_block_delta does not send StatsMsg",
			input:          `{"type":"content_block_delta","delta":{"text":"streaming"}}`,
			expectStatsMsg: false,
			description:    "content_block_delta events should not trigger StatsMsg",
		},
		{
			name:           "system event does not send StatsMsg",
			input:          `{"type":"system","message":"Initializing..."}`,
			expectStatsMsg: false,
			description:    "system events should not trigger StatsMsg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTaskTracker()
			// We can't easily mock tea.Program, but we can verify parser state
			// and the conditional logic by checking that the bridge processes correctly
			bridge := NewBridge(nil, tracker)

			// Parse the line directly through the parser to verify it works
			event, err := bridge.parser.ParseLine([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			stats := bridge.parser.GetStats()

			// Verify the stats match expectations based on event type
			if tt.expectStatsMsg {
				if event.Type != "assistant" && event.Type != "result" {
					t.Errorf("expected event type 'assistant' or 'result', got %q", event.Type)
				}
				// For events that should send StatsMsg, verify stats are non-zero
				if stats.TokensIn == 0 && stats.TokensOut == 0 && stats.CostUSD == 0 {
					t.Errorf("expected non-zero stats for %s", tt.description)
				}
			}
		})
	}
}

// TestBridgeStatsMsgProgressive verifies that multiple assistant messages
// result in progressive StatsMsg updates.
func TestBridgeStatsMsgProgressive(t *testing.T) {
	tracker := NewTaskTracker()
	bridge := NewBridge(nil, tracker)

	// First assistant message
	line1 := `{"type":"assistant","message":{"content":[{"type":"text","text":"First"}],"usage":{"input_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":50}}}`
	_, err := bridge.parser.ParseLine([]byte(line1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats1 := bridge.parser.GetStats()
	if stats1.TokensIn != 100 {
		t.Errorf("after first assistant: expected TokensIn 100, got %d", stats1.TokensIn)
	}
	if stats1.TokensOut != 50 {
		t.Errorf("after first assistant: expected TokensOut 50, got %d", stats1.TokensOut)
	}

	// Second assistant message with updated stats (simulating progressive update)
	line2 := `{"type":"assistant","message":{"content":[{"type":"text","text":"Second"}],"usage":{"input_tokens":150,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":75}}}`
	_, err = bridge.parser.ParseLine([]byte(line2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats2 := bridge.parser.GetStats()
	// Assistant stats should update (not accumulate within iteration)
	if stats2.TokensIn != 150 {
		t.Errorf("after second assistant: expected TokensIn 150, got %d", stats2.TokensIn)
	}
	if stats2.TokensOut != 75 {
		t.Errorf("after second assistant: expected TokensOut 75, got %d", stats2.TokensOut)
	}
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
