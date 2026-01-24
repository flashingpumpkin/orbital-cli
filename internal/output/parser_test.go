package output

import (
	"testing"
	"time"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser() returned nil")
	}
}

func TestStreamEvent_Fields(t *testing.T) {
	now := time.Now()
	event := StreamEvent{
		Type:      "assistant",
		Content:   "Hello, world!",
		Timestamp: now,
	}

	if event.Type != "assistant" {
		t.Errorf("expected Type 'assistant', got %q", event.Type)
	}
	if event.Content != "Hello, world!" {
		t.Errorf("expected Content 'Hello, world!', got %q", event.Content)
	}
	if event.Timestamp != now {
		t.Errorf("expected Timestamp %v, got %v", now, event.Timestamp)
	}
}

func TestOutputStats_Fields(t *testing.T) {
	stats := OutputStats{
		TokensIn:  100,
		TokensOut: 50,
		CostUSD:   0.05,
		Duration:  5 * time.Second,
	}

	if stats.TokensIn != 100 {
		t.Errorf("expected TokensIn 100, got %d", stats.TokensIn)
	}
	if stats.TokensOut != 50 {
		t.Errorf("expected TokensOut 50, got %d", stats.TokensOut)
	}
	if stats.CostUSD != 0.05 {
		t.Errorf("expected CostUSD 0.05, got %f", stats.CostUSD)
	}
	if stats.Duration != 5*time.Second {
		t.Errorf("expected Duration 5s, got %v", stats.Duration)
	}
}

func TestParseLine_AssistantMessage(t *testing.T) {
	p := NewParser()
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != "assistant" {
		t.Errorf("expected Type 'assistant', got %q", event.Type)
	}
	if event.Content != "Hello" {
		t.Errorf("expected Content 'Hello', got %q", event.Content)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set")
	}
}

func TestParseLine_AssistantMessageMultipleContent(t *testing.T) {
	p := NewParser()
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello "},{"type":"text","text":"World"}]}}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Content != "Hello World" {
		t.Errorf("expected Content 'Hello World', got %q", event.Content)
	}
}

func TestParseLine_ResultMessage(t *testing.T) {
	p := NewParser()
	// Use actual Claude Code format: total_cost_usd and usage object
	line := []byte(`{"type":"result","total_cost_usd":0.05,"usage":{"input_tokens":100,"output_tokens":50}}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != "result" {
		t.Errorf("expected Type 'result', got %q", event.Type)
	}

	// Result should accumulate stats
	stats := p.GetStats()
	if stats.TokensIn != 100 {
		t.Errorf("expected TokensIn 100, got %d", stats.TokensIn)
	}
	if stats.TokensOut != 50 {
		t.Errorf("expected TokensOut 50, got %d", stats.TokensOut)
	}
	if stats.CostUSD != 0.05 {
		t.Errorf("expected CostUSD 0.05, got %f", stats.CostUSD)
	}
}

func TestParseLine_ErrorMessage(t *testing.T) {
	p := NewParser()
	line := []byte(`{"type":"error","error":{"message":"Something went wrong"}}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != "error" {
		t.Errorf("expected Type 'error', got %q", event.Type)
	}
	if event.Content != "Something went wrong" {
		t.Errorf("expected Content 'Something went wrong', got %q", event.Content)
	}
}

func TestParseLine_MalformedJSON(t *testing.T) {
	p := NewParser()
	line := []byte(`{invalid json`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Errorf("expected nil error for malformed JSON, got %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for malformed JSON, got %+v", event)
	}
}

func TestParseLine_EmptyLine(t *testing.T) {
	p := NewParser()
	line := []byte(``)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Errorf("expected nil error for empty line, got %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for empty line, got %+v", event)
	}
}

func TestParseLine_UnknownType(t *testing.T) {
	p := NewParser()
	line := []byte(`{"type":"unknown","data":"something"}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != "unknown" {
		t.Errorf("expected Type 'unknown', got %q", event.Type)
	}
}

func TestGetStats_Initial(t *testing.T) {
	p := NewParser()
	stats := p.GetStats()

	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}
	if stats.TokensIn != 0 {
		t.Errorf("expected initial TokensIn 0, got %d", stats.TokensIn)
	}
	if stats.TokensOut != 0 {
		t.Errorf("expected initial TokensOut 0, got %d", stats.TokensOut)
	}
	if stats.CostUSD != 0 {
		t.Errorf("expected initial CostUSD 0, got %f", stats.CostUSD)
	}
}

func TestGetStats_Accumulates(t *testing.T) {
	p := NewParser()

	// Parse first result - use actual Claude Code format
	line1 := []byte(`{"type":"result","total_cost_usd":0.05,"usage":{"input_tokens":100,"output_tokens":50}}`)
	_, err := p.ParseLine(line1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse second result
	line2 := []byte(`{"type":"result","total_cost_usd":0.03,"usage":{"input_tokens":60,"output_tokens":30}}`)
	_, err = p.ParseLine(line2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := p.GetStats()
	if stats.TokensIn != 160 {
		t.Errorf("expected accumulated TokensIn 160, got %d", stats.TokensIn)
	}
	if stats.TokensOut != 80 {
		t.Errorf("expected accumulated TokensOut 80, got %d", stats.TokensOut)
	}
	if stats.CostUSD != 0.08 {
		t.Errorf("expected accumulated CostUSD 0.08, got %f", stats.CostUSD)
	}
}

func TestParseLine_ContentDelta(t *testing.T) {
	p := NewParser()
	// Claude CLI may also send content_block_delta events
	line := []byte(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"streaming text"}}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != "content_block_delta" {
		t.Errorf("expected Type 'content_block_delta', got %q", event.Type)
	}
	if event.Content != "streaming text" {
		t.Errorf("expected Content 'streaming text', got %q", event.Content)
	}
}

func TestParseLine_SystemMessage(t *testing.T) {
	p := NewParser()
	line := []byte(`{"type":"system","message":"Initializing..."}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != "system" {
		t.Errorf("expected Type 'system', got %q", event.Type)
	}
	if event.Content != "Initializing..." {
		t.Errorf("expected Content 'Initializing...', got %q", event.Content)
	}
}

func TestParseLine_ResultWithDuration(t *testing.T) {
	p := NewParser()
	// Result messages include duration_ms (milliseconds) - use actual Claude Code format
	line := []byte(`{"type":"result","total_cost_usd":0.05,"duration_ms":2500,"usage":{"input_tokens":100,"output_tokens":50}}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}

	stats := p.GetStats()
	expectedDuration := 2500 * time.Millisecond
	if stats.Duration != expectedDuration {
		t.Errorf("expected Duration %v, got %v", expectedDuration, stats.Duration)
	}
}

func TestParseLine_WhitespaceOnly(t *testing.T) {
	p := NewParser()
	line := []byte(`   `)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Errorf("expected nil error for whitespace line, got %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for whitespace line, got %+v", event)
	}
}

func TestParseLine_AssistantWithNoContent(t *testing.T) {
	p := NewParser()
	line := []byte(`{"type":"assistant","message":{"content":[]}}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != "assistant" {
		t.Errorf("expected Type 'assistant', got %q", event.Type)
	}
	if event.Content != "" {
		t.Errorf("expected empty Content, got %q", event.Content)
	}
}

func TestParseLine_AssistantWithUsage(t *testing.T) {
	p := NewParser()
	// Real format from Claude CLI stream-json output
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":8442,"cache_read_input_tokens":42399,"output_tokens":150}}}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != "assistant" {
		t.Errorf("expected Type 'assistant', got %q", event.Type)
	}
	if event.Content != "Hello" {
		t.Errorf("expected Content 'Hello', got %q", event.Content)
	}

	stats := p.GetStats()
	expectedTokensIn := 1 + 8442 + 42399
	if stats.TokensIn != expectedTokensIn {
		t.Errorf("expected TokensIn %d, got %d", expectedTokensIn, stats.TokensIn)
	}
	if stats.TokensOut != 150 {
		t.Errorf("expected TokensOut 150, got %d", stats.TokensOut)
	}
}

func TestParseLine_AssistantUsageUpdatesWithLatest(t *testing.T) {
	p := NewParser()

	// First assistant message
	line1 := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"First"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":100,"cache_read_input_tokens":200,"output_tokens":50}}}`)
	_, _ = p.ParseLine(line1)

	// Second assistant message with updated usage
	line2 := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Second"}],"usage":{"input_tokens":2,"cache_creation_input_tokens":150,"cache_read_input_tokens":300,"output_tokens":100}}}`)
	_, _ = p.ParseLine(line2)

	stats := p.GetStats()
	// Should have latest values
	expectedTokensIn := 2 + 150 + 300
	if stats.TokensIn != expectedTokensIn {
		t.Errorf("expected TokensIn %d, got %d", expectedTokensIn, stats.TokensIn)
	}
	if stats.TokensOut != 100 {
		t.Errorf("expected TokensOut 100, got %d", stats.TokensOut)
	}
}

func TestParseLine_ResultMessageActualFormat(t *testing.T) {
	// This is the ACTUAL format from Claude Code CLI stream-json output
	// as captured from: claude -p --verbose --output-format stream-json "say hello"
	p := NewParser()
	line := []byte(`{"type":"result","subtype":"success","is_error":false,"duration_ms":2638,"duration_api_ms":2564,"num_turns":1,"result":"Hello!","session_id":"test","total_cost_usd":0.07306125,"usage":{"input_tokens":3,"cache_creation_input_tokens":10507,"cache_read_input_tokens":14155,"output_tokens":12}}`)

	event, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != "result" {
		t.Errorf("expected Type 'result', got %q", event.Type)
	}

	stats := p.GetStats()

	// Should extract total_cost_usd
	if stats.CostUSD != 0.07306125 {
		t.Errorf("expected CostUSD 0.07306125, got %f", stats.CostUSD)
	}

	// Should extract usage.input_tokens + cache tokens
	expectedTokensIn := 3 + 10507 + 14155
	if stats.TokensIn != expectedTokensIn {
		t.Errorf("expected TokensIn %d, got %d", expectedTokensIn, stats.TokensIn)
	}

	// Should extract usage.output_tokens
	if stats.TokensOut != 12 {
		t.Errorf("expected TokensOut 12, got %d", stats.TokensOut)
	}

	// Should extract duration_ms and convert to time.Duration
	expectedDuration := 2638 * time.Millisecond
	if stats.Duration != expectedDuration {
		t.Errorf("expected Duration %v, got %v", expectedDuration, stats.Duration)
	}
}

func TestParseLine_AssistantThenResult_NoDoubleCount(t *testing.T) {
	// This test verifies that when an assistant message is followed by a result message
	// in the same iteration, the tokens are not double-counted.
	p := NewParser()

	// Assistant message with intermediate cumulative stats
	assistant := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Working..."}],"usage":{"input_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":50}}}`)
	_, err := p.ParseLine(assistant)
	if err != nil {
		t.Fatalf("unexpected error parsing assistant: %v", err)
	}

	// Check intermediate state
	stats := p.GetStats()
	if stats.TokensIn != 100 {
		t.Errorf("after assistant: expected TokensIn 100, got %d", stats.TokensIn)
	}
	if stats.TokensOut != 50 {
		t.Errorf("after assistant: expected TokensOut 50, got %d", stats.TokensOut)
	}

	// Result message with final stats for the iteration
	// These should REPLACE the assistant stats, not add to them
	result := []byte(`{"type":"result","total_cost_usd":0.05,"usage":{"input_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":50}}`)
	_, err = p.ParseLine(result)
	if err != nil {
		t.Fatalf("unexpected error parsing result: %v", err)
	}

	// Final state should be the result values, not assistant + result
	stats = p.GetStats()
	if stats.TokensIn != 100 {
		t.Errorf("after result: expected TokensIn 100 (no double count), got %d", stats.TokensIn)
	}
	if stats.TokensOut != 50 {
		t.Errorf("after result: expected TokensOut 50 (no double count), got %d", stats.TokensOut)
	}
	if stats.CostUSD != 0.05 {
		t.Errorf("expected CostUSD 0.05, got %f", stats.CostUSD)
	}
}

func TestParseLine_MultipleIterations_AccumulatesCorrectly(t *testing.T) {
	// This test verifies that across multiple iterations (assistant + result sequences),
	// the token counts accumulate correctly without double-counting within each iteration.
	p := NewParser()

	// Iteration 1: assistant then result
	assistant1 := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Iter 1"}],"usage":{"input_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":50}}}`)
	_, _ = p.ParseLine(assistant1)
	result1 := []byte(`{"type":"result","total_cost_usd":0.05,"usage":{"input_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":50}}`)
	_, _ = p.ParseLine(result1)

	stats := p.GetStats()
	if stats.TokensIn != 100 {
		t.Errorf("after iter 1: expected TokensIn 100, got %d", stats.TokensIn)
	}
	if stats.TokensOut != 50 {
		t.Errorf("after iter 1: expected TokensOut 50, got %d", stats.TokensOut)
	}

	// Iteration 2: assistant then result
	assistant2 := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Iter 2"}],"usage":{"input_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":100}}}`)
	_, _ = p.ParseLine(assistant2)

	// During iteration 2, stats should show iter1 result + iter2 assistant
	stats = p.GetStats()
	if stats.TokensIn != 300 {
		t.Errorf("during iter 2: expected TokensIn 300 (100 from iter1 + 200 from assistant2), got %d", stats.TokensIn)
	}
	if stats.TokensOut != 150 {
		t.Errorf("during iter 2: expected TokensOut 150 (50 from iter1 + 100 from assistant2), got %d", stats.TokensOut)
	}

	result2 := []byte(`{"type":"result","total_cost_usd":0.03,"usage":{"input_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":100}}`)
	_, _ = p.ParseLine(result2)

	// After iteration 2, should have accumulated results from both iterations
	stats = p.GetStats()
	if stats.TokensIn != 300 {
		t.Errorf("after iter 2: expected TokensIn 300 (100+200), got %d", stats.TokensIn)
	}
	if stats.TokensOut != 150 {
		t.Errorf("after iter 2: expected TokensOut 150 (50+100), got %d", stats.TokensOut)
	}
	if stats.CostUSD != 0.08 {
		t.Errorf("expected CostUSD 0.08 (0.05+0.03), got %f", stats.CostUSD)
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "plain text (non-JSON)",
			input:    "Hello world",
			expected: "",
		},
		{
			name:     "single content_block_delta",
			input:    `{"type":"content_block_delta","delta":{"text":"Hello"}}`,
			expected: "Hello\n",
		},
		{
			name: "multiple content_block_delta lines preserves newlines between events",
			input: `{"type":"content_block_delta","delta":{"text":"Hello "}}
{"type":"content_block_delta","delta":{"text":"World"}}`,
			expected: "Hello \nWorld\n",
		},
		{
			name:     "assistant message with text content",
			input:    `{"type":"assistant","message":{"content":[{"type":"text","text":"Assistant says hello"}]}}`,
			expected: "Assistant says hello\n",
		},
		{
			name: "mixed event types extracts only text",
			input: `{"type":"system","message":"Initializing..."}
{"type":"content_block_delta","delta":{"text":"WORKTREE_PATH: .orbital/worktrees/test"}}
{"type":"content_block_delta","delta":{"text":"\nBRANCH_NAME: orbit/test"}}
{"type":"result","total_cost_usd":0.05}`,
			expected: "Initializing...\nWORKTREE_PATH: .orbital/worktrees/test\n\nBRANCH_NAME: orbit/test\n",
		},
		{
			name: "stream-json with markers embedded in JSON",
			input: `{"type":"content_block_delta","delta":{"text":"Setting up worktree...\n"}}
{"type":"content_block_delta","delta":{"text":"WORKTREE_PATH: .orbital/worktrees/fix-bug\n"}}
{"type":"content_block_delta","delta":{"text":"BRANCH_NAME: orbit/fix-bug"}}`,
			expected: "Setting up worktree...\nWORKTREE_PATH: .orbital/worktrees/fix-bug\nBRANCH_NAME: orbit/fix-bug\n",
		},
		{
			name: "marker followed by success text in separate events",
			input: `{"type":"content_block_delta","delta":{"text":"BRANCH_NAME: orbit/fix-bug"}}
{"type":"content_block_delta","delta":{"text":"success"}}`,
			expected: "BRANCH_NAME: orbit/fix-bug\nsuccess\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractText(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractText() = %q; want %q", got, tt.expected)
			}
		})
	}
}
