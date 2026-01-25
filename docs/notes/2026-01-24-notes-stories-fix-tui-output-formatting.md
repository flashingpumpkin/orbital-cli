# Notes: Fix TUI Output Formatting

## 2026-01-24

### Changes Made

1. **Assistant thought display**: Fixed the `💭` emoji prefix to not be dimmed. Previously the emoji was wrapped in `dim.Sprint()` making it less visible. Now the emoji is rendered normally with only the text content in yellow.

2. **Tool call formatting**: The arrow and tool name are both rendered in cyan, with the path/summary dimmed. Format is `→ ToolName summary`.

3. **Added comprehensive unit tests**: Added tests for:
   - `cleanToolResult` function covering all result types
   - `formatTodoWriteInput` function with various task states
   - `formatToolSummaryTodoWrite` verifying multi-line output

### Implementation Details

- Tool calls: 2-space indent before `→` (line 104, 117)
- Tool results: 4-space indent before `✓` (line 149)
- Assistant thoughts: newline then 2-space indent before `💭` (line 124, 136)
- TodoWrite tasks: 6-space indent (line 261)

### Verification

- All tests pass: `go test ./...`
- Build succeeds: `go build ./...`
- Go vet passes: `go vet ./...`

### Files Modified

- `internal/tui/bridge.go`: Updated formatting for assistant thoughts and tool calls
- `internal/tui/bridge_test.go`: Added comprehensive tests for formatting functions

---

## Code Review: 2026-01-24

### Summary

Reviewed commit `e500a9b`: "fix(tui): Match output formatting to Claude Code CLI reference"

### Correctness

**Findings: No blocking issues**

1. **`formatEvent` state management**: The `textShown` flag is properly reset on all event types that mark the start of new content (system, tool calls, user results, errors, results). The streaming text logic correctly tracks whether we're mid-block.

2. **`formatTodoWriteInput` JSON parsing**: Uses proper JSON unmarshalling with struct tags. Handles empty todos, invalid JSON, and empty content fields gracefully.

3. **`cleanToolResult` logic**: The cascading checks correctly handle various result types. The file content detection using `1→` prefix is the actual format used by the Read tool output.

4. **`extractJSONField` edge cases**: Handles missing fields, empty JSON, and non-string values (returns empty). The simple string-based extraction is appropriate for performance-critical display code.

### Edge Cases

**Covered by tests:**

- Empty input/JSON
- Invalid JSON
- Long content truncation (60 chars for tasks)
- All task states (pending, in_progress, completed)
- Various result types (file counts, paths, skill launches, todos)
- File content detection patterns

**Minor note (not blocking):** The `extractJSONField` function would fail on JSON with escaped quotes in values (e.g., `"path": "foo\"bar"`). This is acceptable for the current use cases where tool inputs don't contain such values.

### Code Quality

1. **Consistent styling**: Uses `github.com/fatih/color` consistently for ANSI colours
2. **Clear indentation constants**: 2-space for tools, 4-space for results, 6-space for tasks
3. **Good separation**: `formatTodoWriteInput` extracted as a separate function
4. **Comprehensive tests**: 175 lines of table-driven tests added

### Test Coverage

Tests cover:
- `cleanToolResult`: 10 cases
- `formatTodoWriteInput`: 7 cases
- `formatToolSummaryTodoWrite`: verifies multi-line and indentation
- Existing tests for `extractJSONField`, `shortenPath`, `formatToolSummary`

### Action Items

None. Changes are acceptable.
