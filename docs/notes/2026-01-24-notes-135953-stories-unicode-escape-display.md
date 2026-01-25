# Notes: TUI Display Issues Fix

## 2026-01-24

### Implementation Summary

Completed all three tickets in the TUI Display Quality epic:

**Ticket 1: Unicode Escape Decoding**
- Replaced manual string extraction in `extractJSONField` with `json.Unmarshal`
- The function now properly decodes Unicode escape sequences like `\u003e` to `>`
- Maintains the same function signature and error handling behaviour

**Ticket 2: Test Coverage**
- Added test cases to `TestExtractJSONField` for:
  - Shell redirect: `\u003e` decodes to `>`
  - Ampersand: `\u0026` decodes to `&`
  - Less-than: `\u003c` decodes to `<`
  - Mixed characters in single command
  - Invalid JSON input
  - Non-string field values
  - Null field values

**Ticket 3: Line Wrapping**
- Replaced truncation logic in `renderScrollArea` with word-aware line wrapping
- Added helper functions: `wrapLine`, `findBreakPoint`, `truncateToWidth`
- Uses `github.com/charmbracelet/x/ansi` for ANSI-aware width measurement
- Continuation lines are indented with 4 spaces for visual distinction
- Added tests for wrapping behaviour including ANSI code preservation

### Technical Decisions

- Used `map[string]interface{}` for JSON unmarshalling rather than a typed struct to maintain flexibility
- Implemented custom wrapping logic rather than using external packages to avoid new dependencies
- Used `ansi.StringWidth` for accurate visible width measurement that excludes escape sequences
- Kept wrapping logic in model.go rather than creating a separate utility package since it's TUI-specific

### All Tests Pass

```
go test ./...
ok  	github.com/flashingpumpkin/orbital/cmd/orbital
ok  	github.com/flashingpumpkin/orbital/internal/completion
ok  	github.com/flashingpumpkin/orbital/internal/config
ok  	github.com/flashingpumpkin/orbital/internal/executor
ok  	github.com/flashingpumpkin/orbital/internal/loop
ok  	github.com/flashingpumpkin/orbital/internal/output
ok  	github.com/flashingpumpkin/orbital/internal/spec
ok  	github.com/flashingpumpkin/orbital/internal/state
ok  	github.com/flashingpumpkin/orbital/internal/tasks
ok  	github.com/flashingpumpkin/orbital/internal/tui
ok  	github.com/flashingpumpkin/orbital/internal/workflow
ok  	github.com/flashingpumpkin/orbital/internal/worktree
```

## Code Review (Review Gate)

### Review Summary

Reviewed commit `fc810b3` implementing all three tickets in the TUI Display Quality epic.

### Correctness Analysis

**Ticket 1 (Unicode Escape Decoding)**:
- `extractJSONField` correctly uses `json.Unmarshal` with `map[string]interface{}` to decode JSON
- The type assertion to string is correct and returns empty string for non-string values
- Error handling matches the documented behaviour (return empty string on any failure)
- Implementation is sound and handles all edge cases properly

**Ticket 2 (Test Coverage)**:
- Comprehensive test cases covering all acceptance criteria
- Tests include: shell redirect, ampersand, less-than, mixed characters, invalid JSON, non-string fields, null values
- Table-driven test pattern follows codebase conventions

**Ticket 3 (Line Wrapping)**:
- `wrapLine` function correctly handles ANSI escape sequences using `ansi.StringWidth`
- Word-aware breaking at spaces via `findBreakPoint`
- Falls back to character-based breaking for long words/paths via `truncateToWidth`
- Continuation lines correctly indented with 4 spaces
- Edge cases handled: zero width, narrow terminals, very long words

### Edge Cases Reviewed

- Zero width handling returns original line (correct)
- Narrow terminal (width <= 10) has fallback behaviour
- ANSI escape sequence tracking handles start/end correctly
- Empty string field extraction returns empty string
- Null JSON values return empty string

### Code Quality

- Clean, readable implementation
- Functions are well-documented with clear purpose
- No unnecessary complexity
- Uses standard library JSON unmarshalling (idiomatic Go)
- ANSI handling is thorough with proper escape sequence state tracking

### Test Coverage

- All new functionality has corresponding tests
- Tests verify both happy path and error cases
- Content preservation is verified in wrapping tests
- ANSI code preservation is explicitly tested

### Potential Concerns (Non-Blocking)

- The ANSI escape tracking in `findBreakPoint` assumes all ANSI sequences end with a letter (A-Z or a-z). This is correct for standard CSI sequences but might not handle all edge cases (e.g., OSC sequences). However, for the TUI display use case, this is acceptable.

- Performance: `json.Unmarshal` on every field extraction is slightly slower than manual parsing, but the notes correctly identify this as negligible for TUI display purposes.

### Verdict

No blocking issues found. The implementation is correct, well-tested, and follows codebase conventions.

<gate>PASS</gate>
