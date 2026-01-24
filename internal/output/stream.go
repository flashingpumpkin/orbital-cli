// Package output provides streaming output processing for orbit.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/flashingpumpkin/orbit-cli/internal/tasks"
)

// StreamProcessor processes Claude CLI stream-json output and formats it for display.
type StreamProcessor struct {
	writer        io.Writer
	parser        *Parser
	lastType      string
	textShown     bool
	currentTool   string
	showUnhandled bool
	todosOnly     bool
	tracker       *tasks.Tracker
}

// NewStreamProcessor creates a new StreamProcessor.
func NewStreamProcessor(w io.Writer) *StreamProcessor {
	return &StreamProcessor{
		writer:  w,
		parser:  NewParser(),
		tracker: tasks.NewTracker(),
	}
}

// SetTracker sets a custom task tracker (for sharing across iterations).
func (sp *StreamProcessor) SetTracker(tracker *tasks.Tracker) {
	sp.tracker = tracker
}

// GetTracker returns the task tracker for external access.
func (sp *StreamProcessor) GetTracker() *tasks.Tracker {
	return sp.tracker
}

// SetShowUnhandled enables output of raw JSON for unhandled event types.
func (sp *StreamProcessor) SetShowUnhandled(show bool) {
	sp.showUnhandled = show
}

// SetTodosOnly enables filtering to only show TodoWrite output.
func (sp *StreamProcessor) SetTodosOnly(show bool) {
	sp.todosOnly = show
}

// ProcessLine processes a single line of stream-json output.
func (sp *StreamProcessor) ProcessLine(line string) {
	event, err := sp.parser.ParseLine([]byte(line))
	if err != nil || event == nil {
		return
	}

	// In todosOnly mode, only process assistant events (which contain tool use)
	if sp.todosOnly {
		if event.Type == "assistant" {
			sp.printAssistant(event)
		}
		sp.lastType = event.Type
		return
	}

	switch event.Type {
	case "system":
		sp.printSystem(event.Content)

	case "content_block_start":
		sp.printContentBlockStart(event)

	case "content_block_delta":
		sp.printText(event.Content)

	case "content_block_stop":
		sp.printContentBlockStop()

	case "assistant":
		sp.printAssistant(event)

	case "user":
		sp.printToolResult(event)

	case "error":
		sp.printError(event.Content)

	case "result":
		sp.printResult(event.Content)

	default:
		// Unhandled event type
		if sp.showUnhandled && event.Type != "" {
			sp.printUnhandled(event.Type, line)
		}
	}

	sp.lastType = event.Type
}

// printUnhandled outputs raw JSON for unhandled event types.
func (sp *StreamProcessor) printUnhandled(eventType, rawJSON string) {
	// End any ongoing text block
	if sp.textShown {
		_, _ = fmt.Fprintln(sp.writer)
		sp.textShown = false
	}

	magenta := color.New(color.FgMagenta)
	dim := color.New(color.Faint)

	_, _ = magenta.Fprintf(sp.writer, "  ? [%s] ", eventType)
	_, _ = dim.Fprintln(sp.writer, rawJSON)
}

// printSystem prints a system message.
func (sp *StreamProcessor) printSystem(msg string) {
	if msg == "" {
		return
	}
	// End any ongoing text block
	if sp.textShown {
		_, _ = fmt.Fprintln(sp.writer)
		sp.textShown = false
	}

	dim := color.New(color.Faint)
	_, _ = dim.Fprintf(sp.writer, "⚙ %s\n", msg)
}

// printContentBlockStart handles the start of a new content block.
func (sp *StreamProcessor) printContentBlockStart(event *StreamEvent) {
	// End any ongoing text block
	if sp.textShown {
		_, _ = fmt.Fprintln(sp.writer)
		sp.textShown = false
	}

	if event.Content == "tool_use" && event.ToolName != "" {
		sp.currentTool = event.ToolName
		cyan := color.New(color.FgCyan)
		_, _ = cyan.Fprintf(sp.writer, "🔧 %s\n", event.ToolName)
	}
}

// printContentBlockStop handles the end of a content block.
func (sp *StreamProcessor) printContentBlockStop() {
	if sp.currentTool != "" {
		sp.currentTool = ""
	}
	if sp.textShown {
		_, _ = fmt.Fprintln(sp.writer)
		sp.textShown = false
	}
}

// printAssistant handles assistant messages with text or tool use.
func (sp *StreamProcessor) printAssistant(event *StreamEvent) {
	// End any ongoing text block
	if sp.textShown {
		_, _ = fmt.Fprintln(sp.writer)
		sp.textShown = false
	}

	// Show tool use if present
	if event.ToolName != "" {
		sp.currentTool = event.ToolName

		// Process task-related tools through the tracker
		if tasks.IsTaskTool(event.ToolName) {
			if taskList := sp.tracker.ProcessToolUse(event.ToolName, event.ToolInput); taskList != nil {
				sp.printTaskUpdate(taskList)
			}
			// In todosOnly mode, we've printed the task update, nothing more to do
			if sp.todosOnly {
				return
			}
		}

		// If todosOnly mode, skip non-task tools entirely
		if sp.todosOnly {
			return
		}

		cyan := color.New(color.FgCyan)
		dim := color.New(color.Faint)

		// Format tool input nicely based on tool type
		inputSummary := sp.formatToolInput(event.ToolName, event.ToolInput)
		if inputSummary != "" {
			_, _ = cyan.Fprintf(sp.writer, "  → %s ", event.ToolName)
			_, _ = dim.Fprintln(sp.writer, inputSummary)
		} else {
			_, _ = cyan.Fprintf(sp.writer, "  → %s\n", event.ToolName)
		}
	}

	// Show text content if present and no streaming has occurred (skip in todosOnly mode)
	if event.Content != "" && event.ToolName == "" && !sp.todosOnly {
		sp.printText(event.Content)
	}
}

// printTaskUpdate prints task changes in a compact format.
func (sp *StreamProcessor) printTaskUpdate(taskList []tasks.Task) {
	for _, task := range taskList {
		var icon string
		var col *color.Color

		switch task.Status {
		case "completed":
			icon = "✓"
			col = color.New(color.FgGreen)
		case "in_progress":
			icon = "→"
			col = color.New(color.FgYellow)
		default:
			icon = "○"
			col = color.New(color.Faint)
		}

		content := task.Content
		if len(content) > 60 {
			content = content[:60] + "..."
		}

		_, _ = col.Fprintf(sp.writer, "  [Task] %s %s\n", icon, content)
	}
}

// formatToolInput extracts a brief summary from tool input JSON.
func (sp *StreamProcessor) formatToolInput(toolName, input string) string {
	if input == "" {
		return ""
	}

	// Parse JSON to extract key info
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return ""
	}

	switch toolName {
	case "Read":
		if path, ok := data["file_path"].(string); ok {
			return shortenPath(path)
		}
	case "Glob":
		if pattern, ok := data["pattern"].(string); ok {
			return pattern
		}
	case "Grep":
		if pattern, ok := data["pattern"].(string); ok {
			return pattern
		}
	case "Write", "Edit":
		if path, ok := data["file_path"].(string); ok {
			return shortenPath(path)
		}
	case "Bash":
		if cmd, ok := data["command"].(string); ok {
			if len(cmd) > 60 {
				cmd = cmd[:60] + "..."
			}
			return cmd
		}
	case "Skill":
		if skill, ok := data["skill"].(string); ok {
			return skill
		}
	case "TodoWrite":
		return formatTodoInput(data)
	}

	return ""
}

// shortenPath returns the last 2 path components.
func shortenPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}

// formatTodoInput formats TodoWrite input showing each task with status.
func formatTodoInput(data map[string]any) string {
	todos, ok := data["todos"].([]any)
	if !ok || len(todos) == 0 {
		return ""
	}

	var lines []string
	for _, t := range todos {
		todo, ok := t.(map[string]any)
		if !ok {
			continue
		}
		content, _ := todo["content"].(string)
		status, _ := todo["status"].(string)

		if content == "" {
			continue
		}

		// Truncate long content
		if len(content) > 60 {
			content = content[:60] + "..."
		}

		// Status indicator
		var indicator string
		switch status {
		case "completed":
			indicator = "✓"
		case "in_progress":
			indicator = "▶"
		case "pending":
			indicator = "○"
		default:
			indicator = "?"
		}

		lines = append(lines, fmt.Sprintf("%s %s", indicator, content))
	}

	if len(lines) == 0 {
		return ""
	}

	return "\n      " + strings.Join(lines, "\n      ")
}

// printToolResult handles user messages containing tool results.
func (sp *StreamProcessor) printToolResult(event *StreamEvent) {
	if event.Content == "" {
		return
	}

	// Clean up the content - extract just the useful part
	content := cleanToolResult(event.Content)
	if content == "" {
		return
	}

	dim := color.New(color.Faint)
	green := color.New(color.FgGreen)

	_, _ = green.Fprint(sp.writer, "    ✓ ")
	_, _ = dim.Fprintln(sp.writer, content)
}

// cleanToolResult extracts useful info from tool result content.
func cleanToolResult(content string) string {
	// If it starts with a line number prefix, it's file content - skip it
	if strings.HasPrefix(strings.TrimSpace(content), "1→") ||
		strings.HasPrefix(strings.TrimSpace(content), "     1→") {
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

// printText prints streaming text content (Claude's thoughts).
func (sp *StreamProcessor) printText(text string) {
	if text == "" {
		return
	}

	// Add visual distinction for thoughts
	if !sp.textShown {
		// Starting a new thought block
		_, _ = fmt.Fprintln(sp.writer)
		dim := color.New(color.Faint)
		_, _ = dim.Fprint(sp.writer, "  💭 ")
	}

	sp.textShown = true
	yellow := color.New(color.FgYellow)
	_, _ = yellow.Fprint(sp.writer, text)
}

// printError prints an error message.
func (sp *StreamProcessor) printError(msg string) {
	if msg == "" {
		return
	}
	// End any ongoing text block
	if sp.textShown {
		_, _ = fmt.Fprintln(sp.writer)
		sp.textShown = false
	}

	red := color.New(color.FgRed, color.Bold)
	_, _ = red.Fprintf(sp.writer, "✗ Error: %s\n", msg)
}

// printResult prints the result stats.
func (sp *StreamProcessor) printResult(subtype string) {
	// End any ongoing text block
	if sp.textShown {
		_, _ = fmt.Fprintln(sp.writer)
		sp.textShown = false
	}

	stats := sp.parser.GetStats()
	dim := color.New(color.Faint)

	// Show different messages based on result subtype
	switch subtype {
	case "end_turn":
		_, _ = dim.Fprintf(sp.writer, "\n─── turn complete | tokens: %d in, %d out | cost: $%.4f ───\n",
			stats.TokensIn, stats.TokensOut, stats.CostUSD)
	case "tool_use":
		// Tool use results are intermediate, just show a brief indicator
		_, _ = dim.Fprintf(sp.writer, "   ↳ awaiting tool result\n")
	default:
		_, _ = dim.Fprintf(sp.writer, "\n─── tokens: %d in, %d out | cost: $%.4f ───\n",
			stats.TokensIn, stats.TokensOut, stats.CostUSD)
	}
}

// GetStats returns the accumulated statistics.
func (sp *StreamProcessor) GetStats() *OutputStats {
	return sp.parser.GetStats()
}

// Write implements io.Writer for use with executor streaming.
func (sp *StreamProcessor) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			sp.ProcessLine(line)
		}
	}
	return len(p), nil
}

// PrintTaskSummary prints a summary of all tasks at the end of execution.
func (sp *StreamProcessor) PrintTaskSummary() {
	summary := sp.tracker.GetSummary()
	if summary.Total == 0 {
		return
	}

	_, _ = fmt.Fprintln(sp.writer)
	dim := color.New(color.Faint)
	_, _ = dim.Fprintln(sp.writer, "────────────────────────────────────────────")
	_, _ = dim.Fprintf(sp.writer, "Tasks: %d/%d completed", summary.Completed, summary.Total)
	if summary.InProgress > 0 {
		_, _ = dim.Fprintf(sp.writer, " (%d in progress)", summary.InProgress)
	}
	_, _ = fmt.Fprintln(sp.writer)

	// Group tasks by status
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	grey := color.New(color.Faint)

	// Print in-progress first (most important)
	for _, task := range summary.Tasks {
		if task.Status == "in_progress" {
			_, _ = yellow.Fprintf(sp.writer, "  → %s\n", truncateContent(task.Content, 60))
		}
	}

	// Print pending
	for _, task := range summary.Tasks {
		if task.Status == "pending" {
			_, _ = grey.Fprintf(sp.writer, "  ○ %s\n", truncateContent(task.Content, 60))
		}
	}

	// Print completed
	for _, task := range summary.Tasks {
		if task.Status == "completed" {
			_, _ = green.Fprintf(sp.writer, "  ✓ %s\n", truncateContent(task.Content, 60))
		}
	}
}

// truncateContent truncates content to maxLen characters.
func truncateContent(content string, maxLen int) string {
	if len(content) > maxLen {
		return content[:maxLen] + "..."
	}
	return content
}
