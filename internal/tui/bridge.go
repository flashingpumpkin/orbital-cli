package tui

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/flashingpumpkin/orbital/internal/output"
	"github.com/flashingpumpkin/orbital/internal/tasks"
)

// Default message queue size. Large enough to handle bursts without blocking,
// small enough to limit memory usage.
const defaultQueueSize = 100

// Bridge connects the Claude CLI stream output to the bubbletea TUI.
// It implements io.Writer and sends messages to the tea.Program.
// Messages are sent through a buffered channel to avoid blocking stream
// processing when TUI rendering is slow.
type Bridge struct {
	program *tea.Program
	tracker *tasks.Tracker
	parser  *output.Parser

	mu        sync.Mutex
	textShown bool // tracks if we're in a streaming text block

	// Message queue for non-blocking sends to TUI
	msgQueue chan tea.Msg
	closed   atomic.Bool
	wg       sync.WaitGroup // tracks the message pump goroutine
}

// NewBridge creates a new Bridge with the given program and tracker.
// It starts a background goroutine that pumps messages to the TUI program.
func NewBridge(program *tea.Program, tracker *tasks.Tracker) *Bridge {
	b := &Bridge{
		program:  program,
		tracker:  tracker,
		parser:   output.NewParser(),
		msgQueue: make(chan tea.Msg, defaultQueueSize),
	}

	// Start the message pump goroutine if program is provided
	if program != nil {
		b.wg.Add(1)
		go b.messagePump()
	}

	return b
}

// messagePump reads messages from the queue and sends them to the TUI program.
// It runs until the bridge is closed.
func (b *Bridge) messagePump() {
	defer b.wg.Done()

	for msg := range b.msgQueue {
		// Send to the TUI program (this may block briefly but won't block writers)
		b.program.Send(msg)
	}
}

// sendMsg sends a message to the TUI through the buffered queue.
// If the queue is full, the message is dropped to avoid blocking stream processing.
func (b *Bridge) sendMsg(msg tea.Msg) {
	if b.closed.Load() {
		return
	}

	// Non-blocking send to the queue
	select {
	case b.msgQueue <- msg:
		// Message queued successfully
	default:
		// Queue is full, drop the message to avoid blocking
		// This is acceptable for real-time updates as newer messages will follow
	}
}

// Close shuts down the bridge and stops the message pump goroutine.
// It should be called when the bridge is no longer needed.
func (b *Bridge) Close() {
	if b.closed.Swap(true) {
		return // Already closed
	}

	close(b.msgQueue)
	b.wg.Wait()
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
			b.sendMsg(TasksMsg(tasks))
		}
	}

	// Format and send output line based on event type
	formatted := b.formatEvent(event)
	if formatted != "" {
		b.sendMsg(OutputLineMsg(formatted))
	}

	// Send progress updates for stats-bearing events
	// Assistant messages contain intermediate cumulative stats during streaming
	// Result messages contain final stats for the iteration
	if event.Type == "assistant" || event.Type == "result" {
		stats := b.parser.GetStats()
		// Only send if we have meaningful stats (non-zero values)
		if stats.TokensIn > 0 || stats.TokensOut > 0 || stats.CostUSD > 0 {
			b.sendMsg(StatsMsg{
				TokensIn:  stats.TokensIn,
				TokensOut: stats.TokensOut,
				Cost:      stats.CostUSD,
			})
		}
	}
}

// formatEvent formats a stream event into a display string.
func (b *Bridge) formatEvent(event *output.StreamEvent) string {
	cyan := color.New(color.FgCyan)
	dim := color.New(color.Faint)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed, color.Bold)

	switch event.Type {
	case "system":
		if event.Content != "" {
			b.textShown = false
			return dim.Sprint("⚙ " + event.Content)
		}

	case "content_block_start":
		if event.Content == "tool_use" && event.ToolName != "" {
			b.textShown = false
			summary := formatToolSummary(event.ToolName, event.ToolInput)
			return cyan.Sprint("  → ") + cyan.Sprint(event.ToolName) + dim.Sprint(summary)
		}

	case "content_block_stop":
		if b.textShown {
			b.textShown = false
			return "" // End of text block, no additional output needed
		}

	case "assistant":
		if event.ToolName != "" {
			b.textShown = false
			summary := formatToolSummary(event.ToolName, event.ToolInput)
			return cyan.Sprint("  → ") + cyan.Sprint(event.ToolName) + dim.Sprint(summary)
		}
		if event.Content != "" {
			// Format as assistant thought with 💭 prefix
			var result string
			if !b.textShown {
				// Starting a new thought block - emoji visible, not dimmed
				result = "\n  💭 "
				b.textShown = true
			}
			return result + yellow.Sprint(event.Content)
		}

	case "content_block_delta":
		// Streaming text content
		if event.Content != "" {
			var result string
			if !b.textShown {
				// Starting a new thought block - emoji visible, not dimmed
				result = "\n  💭 "
				b.textShown = true
			}
			return result + yellow.Sprint(event.Content)
		}

	case "user":
		if event.Content != "" {
			b.textShown = false
			content := cleanToolResult(event.Content)
			if content == "" {
				return ""
			}
			return green.Sprint("    ✓ ") + dim.Sprint(content)
		}

	case "error":
		if event.Content != "" {
			b.textShown = false
			return red.Sprint("✗ Error: " + event.Content)
		}

	case "result":
		b.textShown = false
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
	case "Skill":
		if skill := extractJSONField(input, "skill"); skill != "" {
			return " " + skill
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
	case "TodoWrite":
		return formatTodoWriteInput(input)
	}

	return ""
}

// formatTodoWriteInput formats TodoWrite tool input as multi-line task list.
func formatTodoWriteInput(input string) string {
	var data struct {
		Todos []struct {
			Content string `json:"content"`
			Status  string `json:"status"`
		} `json:"todos"`
	}

	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return ""
	}

	if len(data.Todos) == 0 {
		return ""
	}

	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	dim := color.New(color.Faint)

	var lines []string
	for _, todo := range data.Todos {
		if todo.Content == "" {
			continue
		}

		content := todo.Content
		if len(content) > 60 {
			content = content[:60] + "..."
		}

		var line string
		switch todo.Status {
		case "completed":
			line = green.Sprint("✓") + " " + content
		case "in_progress":
			line = yellow.Sprint("▶") + " " + content
		default:
			line = dim.Sprint("○") + " " + content
		}
		lines = append(lines, "      "+line)
	}

	if len(lines) == 0 {
		return ""
	}

	return "\n" + strings.Join(lines, "\n")
}

// extractJSONField extracts a string field from JSON using proper unmarshalling.
// This ensures Unicode escape sequences (like \u003e for >) are decoded correctly.
// Returns empty string on any error (invalid JSON, missing field, non-string field).
func extractJSONField(jsonStr, field string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return ""
	}

	value, ok := data[field]
	if !ok {
		return ""
	}

	str, ok := value.(string)
	if !ok {
		return ""
	}

	return str
}

// shortenPath returns the last 2 path components.
func shortenPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 3 {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}

// cleanToolResult extracts useful info from tool result content.
func cleanToolResult(content string) string {
	// If it starts with a line number prefix, it's file content - skip it
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "1→") ||
		strings.HasPrefix(trimmed, "     1→") {
		return ""
	}

	// If it's a path, shorten it
	if strings.HasPrefix(content, "/") && !strings.Contains(content, "\n") {
		return shortenPath(content)
	}

	// If it contains "files", it's a count - keep it
	if strings.Contains(content, " files") || strings.Contains(content, "No files") {
		return content
	}

	// If it's a skill launch message, keep it
	if strings.HasPrefix(content, "Launching skill:") {
		return content
	}

	// If it's a todo confirmation, shorten it
	if strings.HasPrefix(content, "Todos have been") {
		return "todos updated"
	}

	// For short content, show it
	if len(content) < 80 && !strings.Contains(content, "\n") {
		return content
	}

	return ""
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
