# Notes: Bubbles Viewport Integration

## Session Start

**Date**: 2026-01-26
**Spec**: docs/specs/2026-01-26-224652-spec-bubbles-viewport-integration.md

## Task Selection

Chose Story 1 (Output Panel migration) as highest leverage because:
1. Unblocks Story 2 (file tabs can follow the same pattern)
2. Removes the bulk of manual scroll logic
3. Establishes the viewport integration pattern

## Observations

### Current Architecture

The current TUI uses:
- `outputLines *RingBuffer` for bounded memory output storage
- Manual scroll state: `outputScroll`, `outputTailing`
- `wrappedLinesCache` for performance optimization
- Custom functions: `wrapLine`, `findBreakPoint`, `scrollUp`, `scrollDown`, etc.

### Key Considerations

1. **Tailing Mode**: Viewport doesn't have built-in tailing. Will need to call `GotoBottom()` after content updates when tailing is enabled.

2. **Content Updates**: Use `viewport.SetContent(strings.Join(lines, "\n"))`. The viewport handles wrapping internally.

3. **Resize Handling**: Update `viewport.Width` and `viewport.Height` in `WindowSizeMsg` handler.

4. **RingBuffer**: The RingBuffer provides bounded memory. Need to decide whether to:
   - Keep RingBuffer and set viewport content from it
   - Let viewport handle all content (may use more memory)

Decision: Keep RingBuffer for memory bounds, set viewport content from buffer contents.

## Progress

### Iteration 1: Story 1 Implementation

**Completed:**
1. Added `github.com/charmbracelet/bubbles v0.21.0` dependency
2. Added `viewport.Model` to the Model struct alongside existing RingBuffer
3. Updated `AppendOutput()` to sync content to viewport and call `GotoBottom()` when tailing
4. Replaced `scrollUp/Down/PageUp/PageDown` with new handlers that use viewport methods:
   - `ScrollUp(1)`, `ScrollDown(1)` (non-deprecated API)
   - `HalfPageUp()`, `HalfPageDown()` (non-deprecated API)
5. Added `home` and `end` key handlers for viewport navigation
6. Updated `renderScrollArea()` to use `viewport.View()` and wrap with borders
7. Removed cache-related fields (`wrappedLinesCache`, `cacheWidth`, `cacheLineCount`) and functions
8. Removed `outputScroll` field (now managed by viewport)
9. Kept `wrapAllOutputLines()` for test compatibility
10. Updated all tests to use viewport API instead of direct field access
11. Removed `TestWrappedLinesCaching` and replaced with `TestViewportScrollPerformance`
12. Updated golden files

**Key Decisions:**
- Kept `RingBuffer` for bounded memory usage; viewport content is synced from buffer
- `outputTailing` flag remains for explicit tailing mode control
- Used `viewport.AtBottom()` to detect when to re-enable tailing after scroll down

**Remaining:**
- Manual verification of scrolling and tailing behaviour
- Story 2: File content tabs migration

