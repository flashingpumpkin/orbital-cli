# User Stories: Fix TUI Display Issues

## Project Overview

Orbital CLI has two display issues affecting readability:

1. **Unicode escape sequences displayed literally**: Shell redirections like `2>&1` appear as `2\u003e\u00261`
2. **Long lines truncated**: Claude's thoughts and other content are cut off with `...` instead of being fully visible

Both are display-only bugs; functionality is unaffected.

**Target Users**: Developers using Orbital CLI to run autonomous Claude Code iterations.

**Business Value**: Improved readability and professional appearance of TUI output, reducing confusion when monitoring Orbital sessions.

## Story Mapping Overview

**Epic**: TUI Display Quality

Three tickets address the display issues:
- Ticket 1: Fix Unicode escape decoding
- Ticket 2: Add tests for Unicode escape handling
- Ticket 3: Wrap long lines instead of truncating

## Epic: TUI Display Quality

### [x] **Ticket 1: Replace manual JSON extraction with proper unmarshalling**

**As a** developer using Orbital CLI
**I want** shell commands to display correctly in the TUI
**So that** I can easily read and understand what commands Claude is executing

**Context**: The `extractJSONField` function in `internal/tui/bridge.go` uses manual string manipulation to extract JSON field values. This approach, while performant, fails to decode Unicode escape sequences. When Claude CLI outputs commands containing `>`, `<`, or `&` characters, the JSON encoding escapes them as `\u003e`, `\u003c`, and `\u0026`. The manual extraction returns these escapes verbatim instead of decoding them to their original characters.

**Description**: Replace the manual string extraction in `extractJSONField` with proper `json.Unmarshal` to ensure all Unicode escape sequences are decoded correctly. The function currently searches for `"field":` patterns and extracts the value through string slicing. The replacement should unmarshal the entire JSON object and return the typed field value.

**Implementation Requirements**:
- Replace string manipulation logic with `json.Unmarshal` call
- Handle error cases by returning empty string (matching current behaviour)
- Support string field extraction (current use case)
- Maintain function signature: `func extractJSONField(jsonStr, field string) string`

**Acceptance Criteria**:
- [x] Given a JSON string `{"command":"ls 2\u003e\u00261"}`, when extracting the "command" field, then the result is `ls 2>&1`
- [x] Given a JSON string `{"command":"cmd1 \u0026\u0026 cmd2"}`, when extracting the "command" field, then the result is `cmd1 && cmd2`
- [x] Given a JSON string `{"command":"cat \u003c file"}`, when extracting the "command" field, then the result is `cat < file`
- [x] Given invalid JSON, when extracting any field, then an empty string is returned
- [x] Given valid JSON without the requested field, when extracting that field, then an empty string is returned
- [x] Given valid JSON with a non-string field value, when extracting that field, then an empty string is returned

**Definition of Done** (Single Commit):
- [x] `extractJSONField` function updated to use `json.Unmarshal`
- [x] All existing tests pass
- [x] New test cases added for Unicode escape handling
- [x] Manual verification with a real Orbital run shows correct display

**Dependencies**:
- None (self-contained change)

**Risks**:
- Minimal risk; the function is used only for display formatting
- Performance impact is negligible for TUI use case

**Notes**: The tech proposal also mentions Option B (fixing at the parser encoding point), but Option A is preferred as it keeps the fix localised to the display layer.

**Effort Estimate**: XS (under 2 hours)

---

### [x] **Ticket 2: Add comprehensive test coverage for JSON field extraction**

**As a** developer maintaining Orbital CLI
**I want** test coverage for Unicode escape handling in JSON extraction
**So that** future changes do not reintroduce display bugs

**Context**: The `extractJSONField` function currently lacks test coverage for Unicode escape sequences. Adding dedicated tests ensures the fix is verified and prevents regression.

**Description**: Add table-driven tests covering various Unicode escape scenarios including shell redirections, logical operators, and input redirections. Tests should verify both the happy path (correct decoding) and edge cases (invalid JSON, missing fields, non-string values).

**Implementation Requirements**:
- Add test function `TestExtractJSONField_UnicodeEscapes` in `bridge_test.go`
- Use table-driven test pattern consistent with codebase style
- Cover shell redirect (`2>&1`), ampersand (`&&`), less-than (`<`) cases
- Cover error handling cases

**Acceptance Criteria**:
- [x] Test case exists for shell redirect: `\u003e` decodes to `>`
- [x] Test case exists for ampersand: `\u0026` decodes to `&`
- [x] Test case exists for less-than: `\u003c` decodes to `<`
- [x] Test case exists for mixed characters in single command
- [x] Test case exists for invalid JSON input
- [x] Test case exists for missing field
- [x] All tests pass with the updated implementation

**Definition of Done** (Single Commit):
- [x] Test file updated with new test function
- [x] All tests pass
- [x] Test coverage includes all acceptance criteria scenarios

**Dependencies**:
- Ticket 1 (implementation must exist before tests can pass)

**Risks**:
- None

**Notes**: This ticket can be combined with Ticket 1 into a single commit if preferred, as tests and implementation naturally belong together.

**Effort Estimate**: XS (under 1 hour)

---

### [x] **Ticket 3: Wrap long output lines instead of truncating**

**As a** developer using Orbital CLI
**I want** Claude's thoughts and other long text to be fully visible
**So that** I can read the complete reasoning without missing important context

**Context**: The TUI output panel truncates lines that exceed the panel width, appending `...` to cut-off content. This is implemented in `internal/tui/model.go:237-238`:

```go
if len(line) > width {
    line = line[:width-3] + "..."
}
```

Claude's thoughts often contain valuable reasoning that helps users understand what the agent is doing. Truncating this content forces users to guess at the full meaning or miss critical information entirely.

**Description**: Replace line truncation with word-aware line wrapping in the `renderOutputPanel` function. Long lines should wrap to subsequent lines, preserving all content. The wrapping should be word-aware where possible (break at spaces rather than mid-word) but fall back to character-based wrapping for very long words or paths.

**Implementation Requirements**:
- Replace truncation logic with line wrapping in `renderOutputPanel`
- Use `lipgloss` or `wordwrap` package for word-aware wrapping (lipgloss is already a dependency)
- Preserve ANSI colour codes across wrapped lines
- Indent continuation lines to visually distinguish them from new output lines
- Handle edge cases: very long words, file paths, URLs

**Acceptance Criteria**:
- [x] Given a line longer than panel width, when rendered, then the full content is visible across multiple lines
- [x] Given a line with words, when wrapped, then breaks occur at word boundaries where possible
- [x] Given a line with ANSI colour codes, when wrapped, then colours are preserved on continuation lines
- [x] Given a continuation line, when rendered, then it is visually distinct from a new output line (indented or marked)
- [x] Given a very long word (path or URL), when wrapped, then it breaks at character boundary

**Definition of Done** (Single Commit):
- [x] Truncation replaced with wrapping in `renderOutputPanel`
- [x] All existing tests pass
- [x] New tests cover wrapping behaviour
- [x] Manual verification shows full Claude thoughts visible

**Dependencies**:
- None (self-contained change)

**Risks**:
- Wrapped lines consume more vertical space, reducing visible history
- Very long content may dominate the output panel
- ANSI code handling may have edge cases

**Mitigation**: The trade-off of more vertical space for complete content is acceptable. Users can scroll to see history. Complete information is more valuable than compact display.

**Notes**: Consider using `lipgloss.NewStyle().Width(width).Render(line)` which handles wrapping automatically, or the `github.com/muesli/reflow/wordwrap` package for more control.

**Effort Estimate**: S (2-4 hours)

---

## Backlog Prioritisation

**Must Have:**
- Ticket 1: Replace manual JSON extraction with proper unmarshalling
- Ticket 2: Add comprehensive test coverage for JSON field extraction
- Ticket 3: Wrap long output lines instead of truncating

**Should Have:**
- None

**Could Have:**
- None

**Won't Have:**
- Option B (fixing Unicode at parser encoding point) is explicitly excluded per tech proposal recommendation

## Technical Considerations

**Unicode escapes (Tickets 1-2)**: The fix is isolated to `internal/tui/bridge.go`. The change replaces approximately 20 lines of manual string manipulation with 10 lines using standard library JSON unmarshalling. The performance difference is negligible for TUI display purposes.

**Line wrapping (Ticket 3)**: The change is isolated to `internal/tui/model.go` in the `renderOutputPanel` function. The `lipgloss` library (already a dependency) provides word-aware wrapping via `lipgloss.NewStyle().Width(n).Render(s)`. Alternatively, `github.com/muesli/reflow/wordwrap` offers finer control. ANSI code preservation needs testing with coloured output.

## Success Metrics

- Shell redirections (`2>&1`) display correctly in TUI
- Ampersands (`&&`) display correctly in TUI
- Less-than signs (`<`) display correctly in TUI
- Claude's thoughts are fully visible without truncation
- Long Bash commands are fully visible
- No regression in existing functionality
- All tests pass
