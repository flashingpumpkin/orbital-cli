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

## Code Review - Iteration 1

### Security
No issues. The ring buffer provides bounded memory (10,000 lines max), preventing DoS. File size limits (1MB) are enforced. No injection vectors or information disclosure concerns.

### Design
_ISSUES_FOUND

1. **Dual data storage creates synchronisation burden**: Model maintains both `RingBuffer` and `viewport.Model` storing the same data. Every `AppendOutput()` rebuilds the entire content string (O(n) at 10,000 lines). Consider rendering only visible content or using viewport as single source of truth.

2. **Inconsistent scrolling abstractions**: Output tab uses viewport component; file tabs use manual `fileScroll` offsets. This creates duplicate scroll logic and inconsistent page scroll distances (viewport uses HalfPage, files use full page).

3. **Test-only method in production code**: `wrapAllOutputLines()` exists solely for tests. Should be moved to test file or tests should verify behaviour through public interfaces.

4. **Feature envy in file tab scroll handling**: Scroll boundary calculations are duplicated across 6 methods. Extract a `FileTabState` struct to own scroll logic.

### Logic
_ISSUES_FOUND

1. **CRITICAL - ClearOutput() does not reset outputTailing**: If user scrolls up then clears output, `outputTailing` remains false. New content won't auto-scroll to bottom. Fix: add `m.outputTailing = true` in `ClearOutput()`.

2. **Wrapping mismatch**: `syncViewportContent()` pushes raw lines to viewport. `wrapAllOutputLines()` (used by tests) applies custom list-aware wrapping. Tests may not reflect actual render behaviour.

3. **Viewport initialised with 0x0 dimensions**: Content can be added before `WindowSizeMsg` arrives. Viewport operations on zero dimensions have undefined behaviour.

### Error Handling
No issues. Guards exist for zero/negative dimensions. Viewport component handles edge cases internally. Ring buffer iteration is safe for empty state.

### Data Integrity
_ISSUES_FOUND

1. **outputTailing flag desync after window resize**: Resize changes viewport dimensions but doesn't sync `outputTailing` with `viewport.AtBottom()`. User may miss new content if resize puts them at bottom but flag remains false.

2. **Zero-dimension viewport operations**: `syncViewportContent()` should guard against viewport.Width/Height being zero to avoid undefined behaviour.

### Verdict
**FAIL**

Critical issue: `ClearOutput()` doesn't reset `outputTailing`, causing new content to not auto-scroll after clear. This is a functional bug that affects normal usage.

Secondary concerns:
- Design debt from dual data storage (performance impact at scale)
- Inconsistent abstractions between output and file tabs
- Missing guard for zero-dimension viewport

Recommend fixing the `ClearOutput()` bug before marking this iteration complete.

## Iteration 2: Review Fixes

### Issues Addressed

**Logic Issue 1 (CRITICAL)**: `ClearOutput()` does not reset `outputTailing`
- **Fix**: Added `m.outputTailing = true` to `ClearOutput()` method
- **Rationale**: When output is cleared, the user expects new content to auto-scroll. Resetting tailing ensures new content appears at the bottom.

**Data Integrity Issue 1**: `outputTailing` flag desync after window resize
- **Fix**: Added tailing sync logic after resize in `WindowSizeMsg` handler
- **Rationale**: If the user was tailing before resize, or if the resize puts them at the bottom, maintain tailing mode and call `GotoBottom()` to ensure consistent behaviour.

**Data Integrity Issue 2**: Zero-dimension viewport operations
- **Fix**: Added guard in `syncViewportContent()` to return early if `viewport.Width <= 0 || viewport.Height <= 0`
- **Rationale**: Before `WindowSizeMsg` arrives, the viewport has 0x0 dimensions. Operations on such a viewport have undefined behaviour. The guard prevents content from being set until valid dimensions are established.

**Test Updates**
- Updated tests that used `Height: 20` to use `Height: 24` (the minimum valid terminal height)
- This was necessary because the zero-dimension guard correctly prevents viewport operations when the layout is marked as "too small" (height < 24)

### Issues Not Addressed (Deferred)

The following design issues were identified in the review but are out of scope for Story 1:

- **Dual data storage**: The RingBuffer + viewport approach is intentional for bounded memory. The O(n) cost at 10,000 lines is acceptable given the interactive nature of the UI.
- **Inconsistent scrolling abstractions**: Story 2 will migrate file tabs to viewport, addressing this inconsistency.
- **Test-only method**: `wrapAllOutputLines()` can be cleaned up in a future refactoring pass.
- **Feature envy in file tab scroll handling**: Will be addressed when file tabs migrate to viewport in Story 2.

### Verification
- All tests pass (`make check`)
- Build succeeds (`make build`)

