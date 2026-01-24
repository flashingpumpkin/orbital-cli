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
