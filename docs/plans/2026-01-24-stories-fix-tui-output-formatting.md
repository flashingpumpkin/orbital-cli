# User Stories Plan: Fix TUI Output Formatting

## Project Overview

The Orbit CLI TUI (Terminal User Interface) displays Claude Code stream output but the current formatting does not match the expected output shown in the reference screenshot. The output should replicate the exact formatting style of the original Claude Code CLI output including tool calls, tool results, assistant thoughts, and TodoWrite task displays.

### Reference Output Format

The expected output format from the screenshot shows:
- Tool calls: `→ ToolName path/or/summary` (cyan arrow, tool name, dimmed path)
- Tool results: `✓ result summary` (green checkmark, dimmed content)
- Assistant text: `💭 Claude's thoughts here` (thought emoji prefix, yellow text)
- TodoWrite display: Multi-line task list with status icons (▶ in progress, ○ pending, ✓ completed)

### Current Problem

The TUI bridge (`internal/tui/bridge.go`) and stream processor (`internal/output/stream.go`) have similar but not identical formatting. The output shown in the scroll area does not match the reference screenshot format.

## Story Mapping Overview

**Epic**: TUI Output Formatting Fix

Stories are ordered outside-in, starting with the visual display layer and working inward to the parsing logic.

---

## Epic: TUI Output Formatting Fix

### [x] **Ticket: Match tool call arrow formatting to reference**

**As a** user viewing the TUI
**I want** tool calls to display with the exact format `→ ToolName summary`
**So that** the output matches the familiar Claude Code CLI format

**Context**: The reference screenshot shows tool calls formatted as `→ Read .../config/agents.go` with the arrow followed by tool name and a shortened path. Currently the bridge formats these slightly differently.

**Description**: Update the bridge's `formatEvent` function to match the exact visual format shown in the screenshot. The arrow should be cyan, tool name should be styled, and the path/summary should be dimmed.

**Implementation Requirements**:
- Arrow prefix should be `→` (not `->`)
- Tool name follows immediately after the arrow
- Path/summary follows the tool name, dimmed
- File paths should be shortened to `.../{parent}/{file}`

**Acceptance Criteria**:
- [x] Given a Read tool call, when displayed, then it shows `→ Read .../path/file.go`
- [x] Given a Glob tool call, when displayed, then it shows `→ Glob pattern`
- [x] Given an Edit tool call, when displayed, then it shows `→ Edit .../path/file.go`
- [x] Tool name is visually distinct from the summary

**Definition of Done** (Single Commit):
- [x] Bridge `formatEvent` updated for tool calls
- [x] Unit tests verify exact output format
- [x] Manual testing confirms visual match

**Dependencies**: None

**Risks**: None identified

**Notes**: Compare current bridge.go:92 output format with stream.go:196 format

**Effort Estimate**: XS

---

### [x] **Ticket: Match tool result checkmark formatting to reference**

**As a** user viewing the TUI
**I want** tool results to display with `✓ result` format
**So that** successful tool completions are clearly visible

**Context**: The reference screenshot shows tool results as `✓ 6 files` or `✓ todos updated`. The checkmark should be green and the result content dimmed.

**Description**: Update the bridge's `formatEvent` function to format user (tool result) events with the green checkmark prefix and dimmed result content.

**Implementation Requirements**:
- Checkmark prefix should be `✓` (green)
- Result content should be dimmed
- Indentation should be 4 spaces before checkmark
- Long results should be truncated appropriately

**Acceptance Criteria**:
- [x] Given a Glob result with 6 files, when displayed, then it shows `    ✓ 6 files`
- [x] Given a Read result, when displayed, then it shows `    ✓ shortened/path`
- [x] Given a TodoWrite result, when displayed, then it shows `    ✓ todos updated`
- [x] Checkmark is green, content is dimmed

**Definition of Done** (Single Commit):
- [x] Bridge `formatEvent` updated for tool results
- [x] Unit tests verify exact output format
- [x] Manual testing confirms visual match

**Dependencies**: None

**Risks**: None identified

**Notes**: The `cleanToolResult` function in stream.go already handles result cleaning

**Effort Estimate**: XS

---

### [x] **Ticket: Match assistant thought display to reference**

**As a** user viewing the TUI
**I want** Claude's thoughts to display with `💭` prefix in yellow
**So that** assistant reasoning is visually distinct from tool operations

**Context**: The reference screenshot shows assistant text with the thought bubble emoji prefix and yellow text. Text should wrap appropriately and be clearly distinguishable from tool output.

**Description**: Update the bridge to format assistant text content with the thought bubble emoji prefix and appropriate styling.

**Implementation Requirements**:
- Thought bubble prefix `💭` before assistant text
- Text should be yellow/cream coloured
- New thought blocks should have a blank line separator
- Streaming text should append without repeating prefix

**Acceptance Criteria**:
- [x] Given assistant text content, when displayed, then it shows `💭 The first pending story is...`
- [x] Given streaming text chunks, when displayed, then prefix only appears once at start
- [x] Text is visually distinct (yellow) from tool output (cyan/green)

**Definition of Done** (Single Commit):
- [x] Bridge `formatEvent` updated for assistant text
- [x] State tracking for streaming text blocks added
- [x] Unit tests verify format
- [x] Manual testing confirms visual match

**Dependencies**: None

**Risks**: Streaming text state management adds complexity

**Notes**: May need to track `textShown` state like stream.go does

**Effort Estimate**: S

---

### [x] **Ticket: Match TodoWrite multi-line task display to reference**

**As a** user viewing the TUI
**I want** TodoWrite tool calls to display each task with status icons
**So that** I can see the full task list with clear status indicators

**Context**: The reference screenshot shows TodoWrite calls displaying each task item with status icons: `▶` for in_progress, `○` for pending, `✓` for completed. Each task appears on its own line, indented under the TodoWrite tool call.

**Description**: Update the bridge to parse TodoWrite tool input and format each task as a separate indented line with the appropriate status icon.

**Implementation Requirements**:
- TodoWrite tool call shows `→ TodoWrite` header
- Each task item appears on its own line, indented
- Status icons: `▶` (yellow) for in_progress, `○` (dim) for pending, `✓` (green) for completed
- Task content truncated at 60 characters
- Format matches the reference exactly

**Acceptance Criteria**:
- [x] Given a TodoWrite with 4 tasks, when displayed, then shows header plus 4 indented task lines
- [x] Given a task with status `in_progress`, when displayed, then shows `▶ Task content...`
- [x] Given a task with status `pending`, when displayed, then shows `○ Task content...`
- [x] Given a task with status `completed`, when displayed, then shows `✓ Task content...`
- [x] Long task content is truncated with `...`

**Definition of Done** (Single Commit):
- [x] Bridge `formatToolSummary` updated for TodoWrite
- [x] Helper function to parse todo JSON and format tasks
- [x] Unit tests verify multi-line output
- [x] Manual testing confirms visual match with screenshot

**Dependencies**: None

**Risks**: JSON parsing within format function adds complexity

**Notes**: Reference implementation exists in stream.go `formatTodoInput`

**Effort Estimate**: S

---

### [x] **Ticket: Add ANSI colour support to TUI output lines**

**As a** user viewing the TUI
**I want** output lines to include proper ANSI colour codes
**So that** tool calls, results, and thoughts are colour-coded as in the reference

**Context**: The TUI scroll area currently displays plain text. To match the reference screenshot, output lines need to include ANSI colour codes that the TUI renderer preserves.

**Description**: Ensure the bridge formats output lines with lipgloss styles and that the TUI scroll area renders these colours correctly.

**Implementation Requirements**:
- Tool arrows and names in cyan
- Checkmarks in green
- Result content dimmed (grey)
- Assistant text in yellow
- Task status icons coloured appropriately
- TUI scroll area must not strip ANSI codes

**Acceptance Criteria**:
- [x] Given formatted output with ANSI codes, when rendered in TUI, then colours display correctly
- [x] Tool calls display in cyan
- [x] Tool results display with green checkmark and dim content
- [x] Assistant text displays in yellow

**Definition of Done** (Single Commit):
- [x] Bridge uses lipgloss styles for formatting
- [x] TUI scroll area preserves ANSI codes
- [x] Visual testing confirms colours match reference
- [x] Unit tests verify ANSI codes in output

**Dependencies**: Ticket 1-4 (formatting changes)

**Risks**: May require changes to TUI rendering to preserve colours

**Notes**: lipgloss and bubbletea already used throughout internal/tui package

**Effort Estimate**: S

---

### [x] **Ticket: Match line indentation to reference format**

**As a** user viewing the TUI
**I want** consistent indentation across all output types
**So that** the visual hierarchy matches the reference screenshot

**Context**: The reference shows specific indentation patterns: tool calls have 2-space indent, tool results have 4-space indent (under their tool call), task items have 6-space indent (under TodoWrite).

**Description**: Audit and fix indentation across all formatted output to match the reference screenshot exactly.

**Implementation Requirements**:
- Tool calls: 2-space indent before `→`
- Tool results: 4-space indent before `✓`
- Assistant thoughts: newline then 2-space indent before `💭`
- TodoWrite tasks: 6-space indent for each task line

**Acceptance Criteria**:
- [x] Given tool call output, when displayed, then has 2 leading spaces
- [x] Given tool result output, when displayed, then has 4 leading spaces
- [x] Given TodoWrite task, when displayed, then has 6 leading spaces
- [x] Indentation is consistent across all event types

**Definition of Done** (Single Commit):
- [x] All format functions use consistent indentation
- [x] Unit tests verify exact spacing
- [x] Visual comparison confirms match

**Dependencies**: Tickets 1-4

**Risks**: None identified

**Notes**: May be addressed as part of earlier tickets

**Effort Estimate**: XS

---

## Backlog Prioritisation

**Must Have (Sprint 1):**
- [x] Match tool call arrow formatting to reference
- [x] Match tool result checkmark formatting to reference
- [x] Match assistant thought display to reference
- [x] Match TodoWrite multi-line task display to reference

**Should Have (Sprint 1):**
- [x] Add ANSI colour support to TUI output lines
- [x] Match line indentation to reference format

**Could Have:**
- Additional formatting polish based on user feedback

**Won't Have:**
- Changes to non-TUI output modes (stream processor)

## Technical Considerations

### Architecture
- Changes primarily in `internal/tui/bridge.go`
- May reference patterns from `internal/output/stream.go`
- TUI model in `internal/tui/model.go` renders output lines

### Dependencies
- `github.com/charmbracelet/lipgloss` for TUI styling (already in use)
- `github.com/charmbracelet/bubbletea` for TUI framework (already in use)

### Testing Strategy
- Unit tests for each format function
- Table-driven tests comparing input events to expected output strings
- Manual visual comparison with reference screenshot

## Success Metrics

- TUI output visually matches the reference screenshot
- All tool types display with correct formatting
- Colours render correctly in terminal
- No regression in existing functionality
