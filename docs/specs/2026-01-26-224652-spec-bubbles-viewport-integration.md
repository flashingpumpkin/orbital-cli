# Bubbles Viewport Integration Spec

## Goal

Replace custom scroll logic in the TUI with the `bubbles/viewport` component. This removes 200+ lines of manual scroll handling and leverages a well-tested upstream component.

## Background

The current implementation in `internal/tui/model.go` contains manual scroll logic:
- `wrapAllOutputLines()`, `wrapLine()`, `findBreakPoint()`
- `scrollUp()`, `scrollDown()`, `scrollPageUp()`, `scrollPageDown()`
- `renderScrollArea()`, `renderFileContent()`
- Wrapped lines cache management

The Bubbles `viewport` component provides all this functionality with built-in keyboard navigation, mouse wheel support, and proper edge case handling.

## Stories

### Story 1: Migrate Output Panel to Viewport

**As a** developer
**I want** the output panel to use `bubbles/viewport`
**So that** scroll logic is handled by a tested upstream component

**Acceptance Criteria**
- [x] Add `github.com/charmbracelet/bubbles` dependency
- [x] Replace `outputLines *RingBuffer` with `viewport.Model` in Model struct
- [x] Update `AppendOutput` to set viewport content and call `GotoBottom()` for tailing
- [x] Forward keyboard events (up/down/pgup/pgdown/home/end) to viewport
- [x] Replace `renderScrollArea()` with `viewport.View()`
- [x] Remove unused scroll helper functions (`wrapLine`, `findBreakPoint`, `scrollUp`, `scrollDown`, etc.)
- [x] Existing golden file tests pass or are updated to reflect new rendering
- [ ] Manual verification: output scrolls correctly, tailing works on new content

**Definition of Done**
- Output panel uses viewport component
- No manual scroll logic remains for output panel
- All tests pass

### Story 2: Migrate File Content Tabs to Viewport

**As a** developer
**I want** file content tabs to use individual `viewport.Model` instances
**So that** file scrolling behaviour matches output panel

**Acceptance Criteria**
- [x] Replace file content scroll state with `map[string]viewport.Model`
- [x] Create viewport per file tab on first load
- [x] Forward keyboard events to active file viewport
- [x] Replace `renderFileContent()` with viewport.View()
- [x] Remove remaining scroll-related helper functions
- [x] Existing golden file tests pass or are updated
- [ ] Manual verification: file tabs scroll independently

**Definition of Done**
- File content tabs use viewport components
- Each tab maintains independent scroll position
- All tests pass

## Out of Scope

- Progress bar migration (current implementation is simpler)
- List component for tasks (overkill for 6-item panel)
- Help component (current static help bar is sufficient)
- Animated spinners (external dependency works fine)

## Technical Notes

**Tailing mode**: Viewport does not have built-in tailing. Implement by calling `viewport.GotoBottom()` after content updates when tailing is enabled.

**Resize handling**: Viewport handles resize via `viewport.Width` and `viewport.Height` setters. Update these in the `tea.WindowSizeMsg` handler.

**Content updates**: Use `viewport.SetContent(strings.Join(lines, "\n"))` to update content. The viewport handles wrapping internally.

## References

- docs/research/2026-01-26-001750-bubbletea-testing-patterns.md
- https://pkg.go.dev/github.com/charmbracelet/bubbles/viewport
