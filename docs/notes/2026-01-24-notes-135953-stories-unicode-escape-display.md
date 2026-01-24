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
