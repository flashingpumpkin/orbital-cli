package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestStreamProcessor_ProcessLine(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		wantText string
	}{
		{
			name: "content_block_delta shows text",
			lines: []string{
				`{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello "}}`,
				`{"type":"content_block_delta","delta":{"type":"text_delta","text":"World"}}`,
			},
			wantText: "Hello World",
		},
		{
			name: "system message shows with icon",
			lines: []string{
				`{"type":"system","message":"Starting session"}`,
			},
			wantText: "⚙ Starting session",
		},
		{
			name: "error shows with icon",
			lines: []string{
				`{"type":"error","error":{"message":"Something went wrong"}}`,
			},
			wantText: "✗ Error: Something went wrong",
		},
		{
			name: "result shows stats",
			lines: []string{
				`{"type":"result","total_cost_usd":0.05,"usage":{"input_tokens":100,"output_tokens":50}}`,
			},
			wantText: "tokens: 100 in, 50 out | cost: $0.0500",
		},
		{
			name: "ignores malformed json",
			lines: []string{
				`not json at all`,
				`{"type":"content_block_delta","delta":{"type":"text_delta","text":"Valid"}}`,
			},
			wantText: "Valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			sp := NewStreamProcessor(&buf)

			for _, line := range tt.lines {
				sp.ProcessLine(line)
			}

			got := buf.String()
			if !strings.Contains(got, tt.wantText) {
				t.Errorf("ProcessLine() output = %q, want to contain %q", got, tt.wantText)
			}
		})
	}
}

func TestStreamProcessor_Write(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf)

	// Simulate what executor does - writing multiple lines
	input := `{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}
{"type":"content_block_delta","delta":{"type":"text_delta","text":" World"}}
`
	n, err := sp.Write([]byte(input))

	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(input) {
		t.Errorf("Write() returned %d, want %d", n, len(input))
	}
	if !strings.Contains(buf.String(), "Hello World") {
		t.Errorf("Write() output = %q, want to contain 'Hello World'", buf.String())
	}
}

func TestStreamProcessor_GetStats(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf)

	// Use actual Claude Code format
	sp.ProcessLine(`{"type":"result","total_cost_usd":0.05,"usage":{"input_tokens":100,"output_tokens":50}}`)
	sp.ProcessLine(`{"type":"result","total_cost_usd":0.03,"usage":{"input_tokens":80,"output_tokens":40}}`)

	stats := sp.GetStats()

	if stats.CostUSD != 0.08 {
		t.Errorf("GetStats().CostUSD = %v, want 0.08", stats.CostUSD)
	}
	if stats.TokensIn != 180 {
		t.Errorf("GetStats().TokensIn = %v, want 180", stats.TokensIn)
	}
	if stats.TokensOut != 90 {
		t.Errorf("GetStats().TokensOut = %v, want 90", stats.TokensOut)
	}
}

func TestStreamProcessor_TaskTracking(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf)

	// Simulate assistant message with TodoWrite tool
	todoWriteEvent := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"TodoWrite","input":{"todos":[{"content":"Task 1","status":"pending","activeForm":"Working on task 1"},{"content":"Task 2","status":"in_progress","activeForm":"Working on task 2"}]}}]}}`
	sp.ProcessLine(todoWriteEvent)

	// Check that tasks were tracked
	tracker := sp.GetTracker()
	tasks := tracker.GetTasks()

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	if tasks[0].Content != "Task 1" {
		t.Errorf("expected first task 'Task 1', got %q", tasks[0].Content)
	}
	if tasks[0].Status != "pending" {
		t.Errorf("expected first task status 'pending', got %q", tasks[0].Status)
	}

	if tasks[1].Content != "Task 2" {
		t.Errorf("expected second task 'Task 2', got %q", tasks[1].Content)
	}
	if tasks[1].Status != "in_progress" {
		t.Errorf("expected second task status 'in_progress', got %q", tasks[1].Status)
	}

	// Verify output contains task indicators
	output := buf.String()
	if !strings.Contains(output, "[Task]") {
		t.Errorf("output should contain [Task] indicator, got: %q", output)
	}
}

func TestStreamProcessor_TodosOnlyMode(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf)
	sp.SetTodosOnly(true)

	// Simulate non-task tool (should be suppressed)
	readEvent := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/some/file"}}]}}`
	sp.ProcessLine(readEvent)

	// Simulate task tool (should be shown)
	todoWriteEvent := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"TodoWrite","input":{"todos":[{"content":"Task 1","status":"pending"}]}}]}}`
	sp.ProcessLine(todoWriteEvent)

	output := buf.String()

	// Read tool should not appear
	if strings.Contains(output, "Read") {
		t.Errorf("todos-only mode should suppress Read tool, got: %q", output)
	}

	// Task should appear
	if !strings.Contains(output, "[Task]") {
		t.Errorf("todos-only mode should show task, got: %q", output)
	}
}

func TestStreamProcessor_PrintTaskSummary(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf)

	// Add some tasks via tracker
	tracker := sp.GetTracker()
	tracker.ProcessToolUse("TodoWrite", `{"todos":[
		{"content":"Completed task","status":"completed"},
		{"content":"In progress task","status":"in_progress"},
		{"content":"Pending task","status":"pending"}
	]}`)

	// Clear buffer before summary
	buf.Reset()

	// Print summary
	sp.PrintTaskSummary()

	output := buf.String()

	// Should show counts
	if !strings.Contains(output, "1/3 completed") {
		t.Errorf("summary should show '1/3 completed', got: %q", output)
	}

	// Should show tasks with status indicators
	if !strings.Contains(output, "✓") {
		t.Errorf("summary should contain completed indicator '✓', got: %q", output)
	}
	if !strings.Contains(output, "→") {
		t.Errorf("summary should contain in_progress indicator '→', got: %q", output)
	}
	if !strings.Contains(output, "○") {
		t.Errorf("summary should contain pending indicator '○', got: %q", output)
	}
}
