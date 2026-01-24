package tui

import (
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flashingpumpkin/orbit-cli/internal/output"
	"github.com/flashingpumpkin/orbit-cli/internal/tasks"
)

// Bridge connects the Claude CLI stream output to the bubbletea TUI.
// It implements io.Writer and sends messages to the tea.Program.
type Bridge struct {
	program *tea.Program
	tracker *tasks.Tracker
	parser  *output.Parser

	mu sync.Mutex
}

// NewBridge creates a new Bridge with the given program and tracker.
func NewBridge(program *tea.Program, tracker *tasks.Tracker) *Bridge {
	return &Bridge{
		program: program,
		tracker: tracker,
		parser:  output.NewParser(),
	}
}

// Write implements io.Writer. It processes each line of stream-json output
// and sends appropriate messages to the TUI program.
func (b *Bridge) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		b.processLine(line)
	}

	return len(p), nil
}

// processLine processes a single line of stream-json output.
func (b *Bridge) processLine(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	event, err := b.parser.ParseLine([]byte(line))
	if err != nil || event == nil {
		return
	}

	// Check for task-related tool uses
	if event.ToolName != "" && event.ToolInput != "" {
		if tasks := b.tracker.ProcessToolUse(event.ToolName, event.ToolInput); tasks != nil {
			b.program.Send(TasksMsg(tasks))
		}
	}

	// Format and send output line based on event type
	formatted := b.formatEvent(event)
	if formatted != "" {
		b.program.Send(OutputLineMsg(formatted))
	}

	// Send progress updates from result events
	if event.Type == "result" {
		stats := b.parser.GetStats()
		b.program.Send(StatsMsg{
			TokensIn:  stats.TokensIn,
			TokensOut: stats.TokensOut,
			Cost:      stats.CostUSD,
		})
	}
}

// formatEvent formats a stream event into a display string.
func (b *Bridge) formatEvent(event *output.StreamEvent) string {
	switch event.Type {
	case "system":
		if event.Content != "" {
			return "  " + event.Content
		}

	case "content_block_start":
		if event.Content == "tool_use" && event.ToolName != "" {
			return "  -> " + event.ToolName + formatToolSummary(event.ToolName, event.ToolInput)
		}

	case "assistant":
		if event.ToolName != "" {
			return "  -> " + event.ToolName + formatToolSummary(event.ToolName, event.ToolInput)
		}
		if event.Content != "" {
			// Truncate long content
			content := event.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			return "  " + content
		}

	case "user":
		if event.Content != "" {
			content := event.Content
			if len(content) > 80 {
				content = content[:80] + "..."
			}
			return "    <- " + content
		}

	case "error":
		if event.Content != "" {
			return "  ERROR: " + event.Content
		}

	case "result":
		stats := b.parser.GetStats()
		return formatResultLine(stats)
	}

	return ""
}

// formatToolSummary formats a brief summary of tool input.
func formatToolSummary(toolName, input string) string {
	if input == "" {
		return ""
	}

	// Simple extraction without full JSON parsing for common tools
	switch toolName {
	case "Read":
		if path := extractJSONField(input, "file_path"); path != "" {
			return " " + shortenPath(path)
		}
	case "Write", "Edit":
		if path := extractJSONField(input, "file_path"); path != "" {
			return " " + shortenPath(path)
		}
	case "Glob":
		if pattern := extractJSONField(input, "pattern"); pattern != "" {
			return " " + pattern
		}
	case "Grep":
		if pattern := extractJSONField(input, "pattern"); pattern != "" {
			return " " + pattern
		}
	case "Bash":
		if cmd := extractJSONField(input, "command"); cmd != "" {
			if len(cmd) > 50 {
				cmd = cmd[:50] + "..."
			}
			return " " + cmd
		}
	case "TaskCreate":
		if subject := extractJSONField(input, "subject"); subject != "" {
			return " " + subject
		}
	case "TaskUpdate":
		if taskID := extractJSONField(input, "taskId"); taskID != "" {
			if status := extractJSONField(input, "status"); status != "" {
				return " #" + taskID + " -> " + status
			}
			return " #" + taskID
		}
	}

	return ""
}

// extractJSONField extracts a string field from JSON without full parsing.
// This is a simple approach for performance; returns empty string on any error.
func extractJSONField(jsonStr, field string) string {
	// Look for "field":"value" pattern
	key := `"` + field + `":`
	idx := strings.Index(jsonStr, key)
	if idx == -1 {
		return ""
	}

	rest := jsonStr[idx+len(key):]
	rest = strings.TrimSpace(rest)

	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}

	rest = rest[1:] // Skip opening quote
	endIdx := strings.Index(rest, `"`)
	if endIdx == -1 {
		return ""
	}

	return rest[:endIdx]
}

// shortenPath returns the last 2 path components.
func shortenPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 3 {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}

// formatResultLine formats the result statistics line.
func formatResultLine(stats *output.OutputStats) string {
	return "  --- tokens: " + formatInt(stats.TokensIn) + " in, " + formatInt(stats.TokensOut) + " out | cost: $" + formatFloat(stats.CostUSD) + " ---"
}

// formatInt formats an integer with thousands separator.
func formatInt(n int) string {
	if n < 1000 {
		return intToString(n)
	}
	return formatNumber(n)
}

// formatFloat formats a float with 4 decimal places.
func formatFloat(f float64) string {
	// Simple formatting without fmt
	whole := int(f)
	frac := int((f - float64(whole)) * 10000)
	if frac < 0 {
		frac = -frac
	}
	return intToString(whole) + "." + padLeft(intToString(frac), 4, '0')
}

// GetParser returns the parser for external access to stats.
func (b *Bridge) GetParser() *output.Parser {
	return b.parser
}

// GetTracker returns the task tracker.
func (b *Bridge) GetTracker() *tasks.Tracker {
	return b.tracker
}
