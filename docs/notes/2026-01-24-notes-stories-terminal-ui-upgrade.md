# Terminal UI Upgrade Notes

## 2026-01-24 - Story 1: Fixed Status Bar Layout

### Context
Starting implementation of the terminal UI upgrade using bubbletea and lipgloss.

### Current State
- Existing output system uses `internal/output/formatter.go` (spinner-based) and `internal/output/stream.go` (stream processing)
- Output is currently scrolling text without fixed panels
- Need to add bubbletea and lipgloss dependencies

### Decisions
- Will create new package `internal/tui` for bubbletea components
- Layout will have scrolling output area on top and fixed status panels at bottom
- Minimum terminal width: 80 characters

### Implementation Plan
1. Add bubbletea and lipgloss to go.mod
2. Create `internal/tui/model.go` with the main bubbletea model
3. Create `internal/tui/layout.go` for panel calculations
4. Create `internal/tui/layout_test.go` for unit tests

### Completed

**Story 1: Fixed Status Bar Layout** - COMPLETE

Files created:
- `internal/tui/layout.go` - Layout calculation with terminal size detection
- `internal/tui/layout_test.go` - Unit tests for layout calculations
- `internal/tui/model.go` - Main bubbletea model with View/Update/Init
- `internal/tui/model_test.go` - Unit tests for model

Features implemented:
- Terminal size detection via `tea.WindowSizeMsg`
- Resize handling that recalculates layout
- Minimum width (80) and height (24) enforcement with graceful messages
- Fixed panel layout: scrolling output (top), task panel (middle, collapsible), progress panel, session panel (bottom)
- Task list with status icons (✓ complete, → in progress, ○ pending)
- Progress display with iteration counter, step info, token counts, cost
- Session info with file path display
- Warning colours when cost or iteration exceeds 80% of limits
- Thousands separator for token counts
- Currency formatting for costs

All tests pass. Build successful.
