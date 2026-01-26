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

## Code Review - Iteration 2

### Security
No issues. Changes are pure TUI state management (scroll position, viewport dimensions, tailing mode). No user input processing, file operations, network access, command execution, or authentication logic. No injection vectors.

### Design
No issues. Changes follow existing patterns:
- Guard clause in `syncViewportContent()` is proper defensive programming
- Tailing state sync in resize handler maintains state coherence
- `ClearOutput()` reset is documented and intentional
- Single responsibility maintained in each change

### Logic
No issues. After thorough analysis:
- Zero-dimension guard correctly prevents undefined viewport operations
- Lines stored in RingBuffer are preserved and processed when dimensions become valid
- Resize tailing logic handles both cases: already tailing (stay tailing) and resize puts user at bottom (enable tailing)
- `ClearOutput()` tailing reset is sensible UX for "start fresh" operation

### Error Handling
No issues. The changes actually improve error handling:
- Zero-dimension guard prevents potential panics or undefined behaviour
- Viewport methods (`AtBottom()`, `GotoBottom()`, `SetContent()`) don't return errors
- No swallowed errors, silent failures, or resource leaks
- Early return in guard is appropriate for transient zero-dimension state

### Data Integrity
No issues. Previous concerns addressed:
- `ClearOutput()` now resets tailing (fixes critical bug from Iteration 1)
- Zero-dimension guard prevents undefined viewport operations
- Resize handler syncs tailing state with viewport position

The concern about resize overwriting user scroll intent was analysed: the condition `m.outputTailing || m.viewport.AtBottom()` is intentional design. If resize coincidentally puts user at bottom, enabling tailing is reasonable UX (the comment documents this intent).

### Verdict
**PASS**

All critical issues from Iteration 1 have been addressed:
1. `ClearOutput()` now resets `outputTailing = true`
2. Zero-dimension viewport guard added to `syncViewportContent()`
3. Resize handler syncs tailing state with viewport position

The code is production-ready. Changes are defensive, well-documented, and maintain proper state invariants.

## Iteration 3: Story 2 Implementation

### Task Selection

Story 2 (File Content Tabs migration) was selected as the highest leverage task because:
1. Story 1 is complete except for manual verification (requires human interaction)
2. Story 2 follows the same pattern established in Story 1
3. Completes the viewport migration for the entire TUI

### Changes Made

1. **Replaced `fileScroll map[string]int` with `fileViewports map[string]viewport.Model`**
   - Each file tab now has its own viewport instance
   - Scroll state is managed by the viewport component

2. **Added `syncFileViewport()` helper function**
   - Creates or updates viewport for a file when content is loaded
   - Sets viewport dimensions (width excludes 6-char line number column)
   - Guards against zero-dimension viewport operations

3. **Updated scroll handlers for file tabs**
   - `handleScrollUp()`: Uses `vp.ScrollUp(1)` instead of decrementing offset
   - `handleScrollDown()`: Uses `vp.ScrollDown(1)` instead of incrementing offset
   - `handleScrollPageUp()`: Uses `vp.HalfPageUp()` for consistent page scrolling
   - `handleScrollPageDown()`: Uses `vp.HalfPageDown()` for consistent page scrolling
   - `handleScrollHome()`: Uses `vp.GotoTop()`
   - `handleScrollEnd()`: Uses `vp.GotoBottom()`

4. **Updated `renderFileContent()` to use viewport scroll position**
   - Gets scroll offset from `vp.YOffset` instead of `fileScroll` map
   - Still renders line numbers manually (viewport doesn't support this)
   - Maintains ANSI-aware truncation for long lines

5. **Updated `WindowSizeMsg` handler**
   - Resizes all file viewports when terminal dimensions change

6. **Updated `reloadCurrentFile()`**
   - Clears both `fileContents` and `fileViewports` for the file being reloaded

7. **Updated tests**
   - Tests now use `syncFileViewport()` and `vp.SetYOffset()` instead of `fileScroll`
   - Assertions check `vp.YOffset` instead of `fileScroll[path]`

### Design Decisions

- **Kept line numbers in file rendering**: The viewport component doesn't support line numbers. Rather than remove this useful feature, I kept manual line number rendering while using the viewport for scroll state management. This is a hybrid approach that preserves functionality.

- **Used `viewport.YOffset` for scroll position**: Rather than calling `viewport.View()` and parsing the output, I directly use the `YOffset` property to slice the file content. This is more efficient and maintains control over line number formatting.

### Verification

All tests pass (`make check`).

### Remaining Work

Both Story 1 and Story 2 have only "Manual verification" items remaining. These require human interaction to verify:
- Story 1: Output scrolls correctly, tailing works on new content
- Story 2: File tabs scroll independently

## Code Review - Iteration 3

### Security
No issues. The viewport migration is purely internal TUI state management. No new external input handling, network operations, or authentication changes. File paths originate from the parent orchestration layer. Map accesses use comma-ok idiom for safety. The bubbletea architecture is inherently single-threaded for state updates.

### Design
_ISSUES_FOUND

1. **Inconsistent viewport usage pattern**: Output tab uses `viewport.View()` for rendering; file tabs only use `viewport.YOffset` and do manual rendering. The file viewports store content via `SetContent()` that is never used for rendering, creating a leaky abstraction.

2. **Code duplication in scroll handlers**: Six methods (handleScrollUp/Down/PageUp/PageDown/Home/End) share 80% identical structure. Could extract a helper method for viewport selection and the get-modify-store pattern.

3. **Value type verbose pattern**: Storing `viewport.Model` (value type) in a map requires verbose get-modify-store on every scroll operation. Could use `*viewport.Model` pointers instead.

### Logic
_ISSUES_FOUND

1. **Early return in syncFileViewport loses modifications**: If `vp.Height <= 0` (during resize to small terminal), the function returns without saving the viewport back to the map. This loses any previous scroll position. When terminal becomes larger, a new viewport is created at position 0.

2. **Dead code in width guard**: After clamping `vp.Width` to minimum 1 (line 1508-1510), the subsequent check `if vp.Width <= 0` (line 1515) can never be true.

3. **Viewport content never used**: `syncFileViewport` calls `vp.SetContent(content)` but `renderFileContent` reads raw `fileContents[path]` and only uses `vp.YOffset`. The content storage is wasteful.

### Error Handling
_ISSUES_FOUND

1. **WindowSizeMsg iterates over wrong map**: The resize handler iterates over `fileViewports` (line 187) but should iterate over `fileContents`. Files loaded before `WindowSizeMsg` (when dimensions are invalid) never get viewports created, causing permanent scroll failure.

2. **Silent scroll failure**: When viewport is missing for a file tab, scroll operations silently do nothing. No feedback to user, no attempt to create the missing viewport.

### Data Integrity
_ISSUES_FOUND

1. **State inconsistency between maps**: Files loaded when dimensions are invalid have content in `fileContents` but no viewport in `fileViewports`. The resize handler only syncs existing viewports, leaving these files permanently without scroll capability.

2. **Same root cause as error handling issue**: The fix is to iterate over `fileContents` in the resize handler, not `fileViewports`.

### Verdict
**FAIL**

Primary issue: The `WindowSizeMsg` handler iterates over `fileViewports` but should iterate over `fileContents`. This causes files loaded before valid dimensions to permanently lack viewport support.

**Required fix** (line 187-189):
```go
// Before:
for path := range m.fileViewports {

// After:
for path := range m.fileContents {
```

Secondary issue: The early return in `syncFileViewport` (line 1516) discards viewport modifications without saving, potentially losing scroll position on extreme resize sequences.

The design issues (inconsistent viewport usage, code duplication, value type verbosity) are acceptable technical debt that doesn't block functionality.

## Iteration 4: Review Fixes for Story 2

### Issues Addressed

**Error Handling Issue 1 (PRIMARY)**: WindowSizeMsg iterates over wrong map
- **Fix**: Changed `for path := range m.fileViewports` to `for path := range m.fileContents` in `WindowSizeMsg` handler (line 187-189)
- **Rationale**: Files loaded before `WindowSizeMsg` arrives (when dimensions are invalid) have content in `fileContents` but no viewport in `fileViewports`. By iterating over `fileContents`, we ensure all files get their viewports created or updated when valid dimensions become available. This prevents the permanent scroll failure bug.

**Logic Issue 1**: Early return in syncFileViewport loses modifications
- **Fix**: Added `m.fileViewports[path] = vp` before the early return when height is invalid
- **Rationale**: During extreme resize sequences (e.g., very small terminal), the viewport dimensions may become invalid but we should still preserve scroll position. Saving the viewport before returning ensures scroll state is not lost.

**Logic Issue 2**: Dead code in width guard
- **Fix**: Removed the `vp.Width <= 0` check since width is already clamped to minimum 1 on line 1508-1510
- **Rationale**: The width clamp ensures `vp.Width >= 1`, so `vp.Width <= 0` can never be true. Only the height check is needed.

### Verification

All tests pass (`make check`).

## Iteration 5: Final State Assessment

### Task Selection

Assessed the remaining work to determine highest leverage action.

### Remaining Items

Both stories have only "Manual verification" items remaining:
- Story 1: `[ ] Manual verification: output scrolls correctly, tailing works on new content`
- Story 2: `[ ] Manual verification: file tabs scroll independently`

### Blocker

These items require human interaction with a running instance of the TUI. They cannot be completed through automated means. The autonomous loop cannot:
- Launch the TUI interactively
- Scroll through output
- Switch between file tabs
- Verify visual behaviour

### Recommendation

A human operator should:
1. Run `orbital --tui` with a spec file that produces substantial output
2. Verify output panel scrolls with arrow keys, Page Up/Down, Home/End
3. Verify new output auto-scrolls to bottom (tailing)
4. Verify scrolling up disables tailing, scrolling to bottom re-enables it
5. Open multiple file tabs
6. Verify each tab scrolls independently
7. Mark the acceptance criteria as complete in the spec file

### Verification Status

All automated checks pass (`make check`). All code changes are complete and reviewed. Only manual verification remains.

## Code Review - Iteration 5 (Documentation Changes)

### Security
No issues. The changes are purely documentation (notes files) and configuration cleanup (golangci.yml version removal). No secrets, credentials, injection vectors, or authentication concerns exposed.

### Design
No issues. The design reviewer noted that the documentation references persistent architectural debt in the codebase (dual data storage, inconsistent viewport patterns, scroll logic duplication). These are documented technical debt items, not problems with the current changeset itself.

### Logic
No issues. The changes are non-executable documentation. No logic bugs, edge cases, or race conditions possible.

### Error Handling
No issues. The changed files contain no executable code, therefore no error handling to evaluate.

### Data Integrity
No issues. The changed files are configuration and documentation only. No data validation, null safety, or consistency concerns.

### Verdict
**PASS**

All five review agents returned CLEAR status. The changes consist of:
1. `.golangci.yml` - Removed deprecated version field (non-functional change)
2. `docs/notes/2026-01-25-notes-223905-continuous-improvement.md` - Added code review findings
3. `docs/notes/2026-01-26-notes-224652-spec-bubbles-viewport-integration.md` - Added iteration notes

These are purely documentation and configuration changes with no security, logic, or data integrity implications.

## Iteration 6: Autonomous Loop Assessment

### Task Selection

Re-assessed spec to identify any remaining automated work.

### Analysis

The spec has two unchecked items remaining:
- Story 1: `[ ] Manual verification: output scrolls correctly, tailing works on new content`
- Story 2: `[ ] Manual verification: file tabs scroll independently`

Both items explicitly require "manual verification", meaning human interaction with a running TUI instance.

### Verification of Automated Coverage

Ran `make check` to confirm all automated tests pass. Examined test coverage for scroll functionality:

- `TestScrollUpOutputTab`: Validates viewport-based output scrolling
- `TestScrollDownOutputTab`: Validates tailing behaviour and scroll down
- `TestScrollPageUpOutputTab`, `TestScrollPageDownOutputTab`: Page navigation
- `TestWindowResizeScrollClamping`: Resize handling
- `TestViewportScrollPerformance`: Performance characteristics
- File tab scroll tests: Use `fileViewports` map and `syncFileViewport()`

The automated tests comprehensively cover the functional behaviour of the viewport integration. The "manual verification" items are about confirming the visual experience matches expectations in an actual running terminal.

### Conclusion

This is a hard blocker for the autonomous loop. The remaining work requires:
1. A human to run `orbital --tui` with a spec file
2. Interactive testing of scroll behaviour
3. Visual confirmation that rendering is correct

The autonomous loop cannot complete this spec. A human operator must perform the manual verification steps documented in Iteration 5.

