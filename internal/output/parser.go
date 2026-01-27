// Package output provides parsing utilities for Claude CLI stream-json output.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// StreamEvent represents a parsed event from the Claude CLI output stream.
type StreamEvent struct {
	Type      string
	Content   string
	Timestamp time.Time
	// Tool-related fields
	ToolName  string
	ToolID    string
	ToolInput string
}

// OutputStats contains accumulated statistics from parsing Claude CLI output.
type OutputStats struct {
	TokensIn  int
	TokensOut int
	CostUSD   float64
	Duration  time.Duration
}

// knownEventTypes lists all event types recognised by this parser version.
// If Claude CLI adds new types, they will trigger warnings until the parser is updated.
var knownEventTypes = map[string]bool{
	"assistant":           true,
	"user":                true,
	"result":              true,
	"error":               true,
	"content_block_delta": true,
	"content_block_start": true,
	"content_block_stop":  true,
	"system":              true,
}

// Parser accumulates statistics while parsing Claude CLI stream-json output.
type Parser struct {
	stats OutputStats
	// assistantTokensIn/Out track tokens from assistant messages within the current iteration.
	// These are overwritten by each assistant message (cumulative within iteration).
	// Result events provide the authoritative final counts and are accumulated separately.
	assistantTokensIn  int
	assistantTokensOut int
	// resultTokensIn/Out accumulate tokens across result events (iterations).
	resultTokensIn  int
	resultTokensOut int
	// Event tracking for validation
	knownEventCount   int            // Count of recognised event types parsed
	unknownEventCount int            // Count of unrecognised event types parsed
	unknownTypes      map[string]int // Map of unknown type -> count (for warning deduplication)
	// Warning output (defaults to nil = no warnings)
	warnWriter io.Writer
}

// NewParser creates a new Parser instance.
func NewParser() *Parser {
	return &Parser{
		unknownTypes: make(map[string]int),
	}
}

// SetWarningWriter sets the writer for format warnings.
// If set, warnings about unrecognised event types are written to it.
// Pass nil to disable warnings (the default).
func (p *Parser) SetWarningWriter(w io.Writer) {
	p.warnWriter = w
}

type messageContent struct {
	Content []contentBlock `json:"content"`
	Usage   *usageStats    `json:"usage,omitempty"`
}

type usageStats struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type contentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type toolUseResult struct {
	Filenames  []string `json:"filenames,omitempty"`
	DurationMs int      `json:"durationMs,omitempty"`
	NumFiles   int      `json:"numFiles,omitempty"`
	Truncated  bool     `json:"truncated,omitempty"`
}

type errorContent struct {
	Message string `json:"message"`
}

type deltaContent struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json,omitempty"`
}

type toolBlock struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

// ParseLine parses a single JSON line from Claude CLI stream-json output.
// Returns nil event (not error) for malformed JSON or empty lines.
func (p *Parser) ParseLine(line []byte) (*StreamEvent, error) {
	// Handle empty or whitespace-only lines
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// Parse JSON - return nil for malformed JSON (not an error per requirements)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return nil, nil
	}

	// Extract type
	var eventType string
	if typeRaw, ok := raw["type"]; ok {
		if err := json.Unmarshal(typeRaw, &eventType); err != nil {
			return nil, nil
		}
	}

	event := &StreamEvent{
		Type:      eventType,
		Timestamp: time.Now(),
	}

	// Track event types for validation
	if eventType != "" {
		if knownEventTypes[eventType] {
			p.knownEventCount++
		} else {
			p.unknownEventCount++
			p.unknownTypes[eventType]++
			// Log warning on first occurrence of each unknown type
			if p.warnWriter != nil && p.unknownTypes[eventType] == 1 {
				_, _ = fmt.Fprintf(p.warnWriter, "warning: unrecognised event type %q in Claude CLI output; "+
					"this may indicate a Claude CLI version incompatibility\n", eventType)
			}
		}
	}

	// Extract content based on type
	switch eventType {
	case "assistant":
		p.parseAssistantMessage(raw, event)

	case "user":
		p.parseUserMessage(raw, event)

	case "result":
		p.parseResultStats(raw)
		event.Content = p.parseResultSubtype(raw)

	case "error":
		event.Content = p.parseErrorContent(raw)

	case "content_block_delta":
		event.Content = p.parseDeltaContent(raw)

	case "content_block_start":
		p.parseContentBlockStart(raw, event)

	case "content_block_stop":
		// Just mark the event type, no additional parsing needed

	case "system":
		event.Content = p.parseSystemContent(raw)
	}

	return event, nil
}

// parseAssistantMessage extracts content and usage stats from assistant message.
func (p *Parser) parseAssistantMessage(raw map[string]json.RawMessage, event *StreamEvent) {
	msgRaw, ok := raw["message"]
	if !ok {
		return
	}

	var msg messageContent
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return
	}

	// Use strings.Builder to avoid O(n²) string concatenation
	var contentBuilder strings.Builder
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			contentBuilder.WriteString(block.Text)
		case "tool_use":
			event.ToolName = block.Name
			event.ToolID = block.ID
			if block.Input != nil {
				if inputBytes, err := json.Marshal(block.Input); err == nil {
					event.ToolInput = string(inputBytes)
				}
			}
		}
	}
	event.Content = contentBuilder.String()

	// Extract usage stats if present
	// Assistant messages contain cumulative tokens within the current API call.
	// These are intermediate values that get replaced by each subsequent assistant message.
	if msg.Usage != nil {
		p.assistantTokensIn = msg.Usage.InputTokens + msg.Usage.CacheCreationInputTokens + msg.Usage.CacheReadInputTokens
		p.assistantTokensOut = msg.Usage.OutputTokens
		// Update stats to reflect current state (assistant values + accumulated result values)
		p.stats.TokensIn = p.resultTokensIn + p.assistantTokensIn
		p.stats.TokensOut = p.resultTokensOut + p.assistantTokensOut
	}
}

// parseUserMessage extracts tool result info from user message.
func (p *Parser) parseUserMessage(raw map[string]json.RawMessage, event *StreamEvent) {
	// Parse tool_use_result if present
	if resultRaw, ok := raw["tool_use_result"]; ok {
		var result toolUseResult
		if err := json.Unmarshal(resultRaw, &result); err == nil {
			if len(result.Filenames) > 0 {
				event.Content = result.Filenames[0]
				if len(result.Filenames) > 1 {
					event.Content = fmt.Sprintf("%d files", len(result.Filenames))
				}
			}
		}
	}

	// Parse message content for tool_result type
	msgRaw, ok := raw["message"]
	if !ok {
		return
	}

	var msg messageContent
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return
	}

	for _, block := range msg.Content {
		if block.Type == "tool_result" {
			event.ToolID = block.ToolUseID
			if event.Content == "" && block.Content != "" {
				// Truncate long content
				content := block.Content
				if len(content) > 100 {
					content = content[:100] + "..."
				}
				event.Content = content
			}
		}
	}
}

// parseResultStats extracts and accumulates statistics from result message.
// The result event format from Claude Code CLI is:
//
//	{"type":"result","total_cost_usd":0.07,"duration_ms":2638,"usage":{"input_tokens":3,"cache_creation_input_tokens":10507,"cache_read_input_tokens":14155,"output_tokens":12}}
//
// Stat accumulation strategy:
// - Cost: accumulates across result events (for budget tracking across iterations)
// - Duration: accumulates across result events
// - Tokens: result events contain the authoritative final counts for the API call,
//   so they REPLACE any intermediate values from assistant messages. Token counts
//   accumulate across multiple result events (iterations).
func (p *Parser) parseResultStats(raw map[string]json.RawMessage) {
	// Extract total_cost_usd (note: field is "total_cost_usd" not "cost_usd")
	// Cost accumulates across iterations for budget tracking
	if costRaw, ok := raw["total_cost_usd"]; ok {
		var cost float64
		if err := json.Unmarshal(costRaw, &cost); err == nil {
			p.stats.CostUSD += cost
		}
	}

	// Extract duration_ms and convert to time.Duration (note: field is "duration_ms" not "duration_seconds")
	// Duration accumulates across iterations
	if durationRaw, ok := raw["duration_ms"]; ok {
		var durationMs int64
		if err := json.Unmarshal(durationRaw, &durationMs); err == nil {
			p.stats.Duration += time.Duration(durationMs) * time.Millisecond
		}
	}

	// Extract token stats from nested usage object
	// Result events contain the authoritative final token counts for this API call.
	// These accumulate across iterations (result events).
	// When a result arrives, it supersedes any assistant tokens from the same iteration,
	// so we reset the assistant counters and add result tokens to the running total.
	if usageRaw, ok := raw["usage"]; ok {
		var usage usageStats
		if err := json.Unmarshal(usageRaw, &usage); err == nil {
			tokensIn := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
			tokensOut := usage.OutputTokens
			// Accumulate result tokens across iterations
			p.resultTokensIn += tokensIn
			p.resultTokensOut += tokensOut
			// Reset assistant tokens (result supersedes them for this iteration)
			p.assistantTokensIn = 0
			p.assistantTokensOut = 0
			// Update stats to reflect the accumulated result totals
			p.stats.TokensIn = p.resultTokensIn
			p.stats.TokensOut = p.resultTokensOut
		}
	}
}

// parseErrorContent extracts error message.
func (p *Parser) parseErrorContent(raw map[string]json.RawMessage) string {
	errRaw, ok := raw["error"]
	if !ok {
		return ""
	}

	var errContent errorContent
	if err := json.Unmarshal(errRaw, &errContent); err != nil {
		return ""
	}
	return errContent.Message
}

// parseDeltaContent extracts text from content_block_delta.
func (p *Parser) parseDeltaContent(raw map[string]json.RawMessage) string {
	deltaRaw, ok := raw["delta"]
	if !ok {
		return ""
	}

	var delta deltaContent
	if err := json.Unmarshal(deltaRaw, &delta); err != nil {
		return ""
	}
	return delta.Text
}

// parseSystemContent extracts message from system event.
func (p *Parser) parseSystemContent(raw map[string]json.RawMessage) string {
	msgRaw, ok := raw["message"]
	if !ok {
		return ""
	}

	var msg string
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return ""
	}
	return msg
}

// parseContentBlockStart extracts tool information from content_block_start.
func (p *Parser) parseContentBlockStart(raw map[string]json.RawMessage, event *StreamEvent) {
	cbRaw, ok := raw["content_block"]
	if !ok {
		return
	}

	var cb toolBlock
	if err := json.Unmarshal(cbRaw, &cb); err != nil {
		return
	}

	event.Content = cb.Type
	if cb.Type == "tool_use" {
		event.ToolName = cb.Name
		event.ToolID = cb.ID
		if cb.Input != nil {
			if inputBytes, err := json.Marshal(cb.Input); err == nil {
				event.ToolInput = string(inputBytes)
			}
		}
	}
}

// parseResultSubtype extracts the subtype from result events.
func (p *Parser) parseResultSubtype(raw map[string]json.RawMessage) string {
	subtypeRaw, ok := raw["subtype"]
	if !ok {
		return ""
	}

	var subtype string
	if err := json.Unmarshal(subtypeRaw, &subtype); err != nil {
		return ""
	}
	return subtype
}

// GetStats returns the accumulated statistics from parsed output.
func (p *Parser) GetStats() *OutputStats {
	return &OutputStats{
		TokensIn:  p.stats.TokensIn,
		TokensOut: p.stats.TokensOut,
		CostUSD:   p.stats.CostUSD,
		Duration:  p.stats.Duration,
	}
}

// ParseStats contains statistics about the parsing process itself.
type ParseStats struct {
	KnownEventCount   int            // Number of recognised events parsed
	UnknownEventCount int            // Number of unrecognised events parsed
	UnknownTypes      map[string]int // Map of unknown type -> occurrence count
}

// GetParseStats returns statistics about the parsing process.
func (p *Parser) GetParseStats() *ParseStats {
	// Return a copy of the map to prevent external modification
	unknownCopy := make(map[string]int, len(p.unknownTypes))
	for k, v := range p.unknownTypes {
		unknownCopy[k] = v
	}
	return &ParseStats{
		KnownEventCount:   p.knownEventCount,
		UnknownEventCount: p.unknownEventCount,
		UnknownTypes:      unknownCopy,
	}
}

// Validate checks if the parser processed any valid events.
// Returns an error if no recognised events were parsed, which may indicate
// a Claude CLI format change or version incompatibility.
func (p *Parser) Validate() error {
	if p.knownEventCount == 0 {
		if p.unknownEventCount > 0 {
			// We got events but none were recognised
			var types []string
			for t := range p.unknownTypes {
				types = append(types, t)
			}
			return fmt.Errorf("no recognised events parsed; found %d unknown event types (%s); "+
				"this may indicate a Claude CLI version incompatibility - please update Orbital",
				len(types), strings.Join(types, ", "))
		}
		// No events at all
		return fmt.Errorf("no events parsed from Claude CLI output; "+
			"check that Claude CLI is producing stream-json output correctly")
	}
	return nil
}

// ExtractText extracts all text content from raw stream-json output.
// It parses each line as JSON and concatenates text from content_block_delta
// and assistant message events. This is useful for searching for markers in
// output without being affected by JSON encoding.
//
// To preserve line boundaries between events (important for marker extraction),
// a newline is appended after content that doesn't already end with one.
func ExtractText(rawOutput string) string {
	parser := NewParser()
	var text strings.Builder
	for _, line := range strings.Split(rawOutput, "\n") {
		event, _ := parser.ParseLine([]byte(line))
		if event != nil && event.Content != "" {
			text.WriteString(event.Content)
			// Preserve line boundaries between events
			if !strings.HasSuffix(event.Content, "\n") {
				text.WriteString("\n")
			}
		}
	}
	return text.String()
}
