# User Stories: TUI Output Window Scrollback

## Project Overview

Orbital CLI's TUI output window currently auto-scrolls to show the most recent output. Users cannot scroll up to review earlier output from Claude's responses during long-running iterations. This forces users to lose context when commands, errors, or reasoning scroll past the visible viewport.

**Target Users**: Developers using Orbital CLI to monitor autonomous Claude Code iterations, especially those debugging issues or understanding Claude's decision-making process.

**Business Value**: Improved debugging experience and transparency into Claude's actions. Users can review the full session history without missing critical information, making Orbital more practical for complex, long-running tasks.

## Story Mapping Overview

**Epic**: TUI Output Scrollback

Five tickets implement scrollback functionality:
- Ticket 1: Add scroll state to TUI model
- Ticket 2: Implement scroll-up with auto-tail unlock
- Ticket 3: Implement scroll-down with auto-tail re-lock
- Ticket 4: Add page up/down scrolling
- Ticket 5: Handle terminal resize during scrollback

## Epic: TUI Output Scrollback

### **Ticket 1: Add output scroll state to TUI model**

**As a** developer using Orbital CLI
**I want** the TUI to track scroll position in the output window
**So that** the system can support scrollback without affecting other functionality

**Context**: The TUI model currently has no mechanism to track scroll position for the output tab. File tabs already support scrollback via the `fileScroll` map. The output window needs similar state tracking, but with an additional concept of "tailing" (locked to bottom, showing most recent output).

**Description**: Add two new fields to the `Model` struct in `internal/tui/model.go`:
1. `outputScroll int` - The line offset from the top of the wrapped output buffer
2. `outputTailing bool` - Whether the output window is locked to the bottom (auto-scrolling)

Initialize both fields in `NewModel()` to default to auto-tail behaviour (current UX).

**Implementation Requirements**:
- Add `outputScroll` field to `Model` struct
- Add `outputTailing` field to `Model` struct
- Initialize `outputScroll = 0` in `NewModel()`
- Initialize `outputTailing = true` in `NewModel()`
- No behaviour changes yet; state is tracked but not used

**Acceptance Criteria**:
- [x] `Model` struct has `outputScroll int` field
- [x] `Model` struct has `outputTailing bool` field
- [x] `NewModel()` initializes `outputScroll` to `0`
- [x] `NewModel()` initializes `outputTailing` to `true`
- [x] All existing tests pass
- [x] TUI behaviour is unchanged (state is tracked but not used yet)

**Definition of Done** (Single Commit):
- [x] Model struct updated with new fields
- [x] NewModel() updated with initialization
- [x] All tests pass
- [x] No functional changes to TUI behaviour

**Dependencies**:
- None

**Risks**:
- None; additive change with no behaviour modification

**Effort Estimate**: XS (under 30 minutes)

---

### **Ticket 2: Implement scroll-up with auto-tail unlock**

**As a** developer using Orbital CLI
**I want** to scroll up in the output window using arrow keys
**So that** I can review earlier output that has scrolled past the viewport

**Context**: The `scrollUp()` function in `internal/tui/model.go:408-421` currently has an early return for the output tab (`m.activeTab == 0`), only handling file tab scrolling. When a user is at the bottom of the output (tailing), pressing up should unlock the tail and position them one line up from the bottom.

**Description**: Update `scrollUp()` to handle output tab scrolling. When `outputTailing == true`, calculate the max scroll offset, unlock tailing, and position one line up from bottom. When already scrolling (`outputTailing == false`), decrement the scroll offset by one line.

**Implementation Requirements**:
- Remove early return for `m.activeTab == 0` in `scrollUp()`
- Add output tab handling before file tab handling
- When tailing: calculate max offset, set `outputTailing = false`, set `outputScroll = maxOffset - 1`
- When not tailing: decrement `outputScroll` if `> 0`
- Extract `wrapAllOutputLines()` helper to reuse line wrapping logic (currently inline in `renderScrollArea()`)
- Handle edge case: nothing to scroll (output fits in viewport)

**Acceptance Criteria**:
- [x] Given the output tab is active and tailing, when pressing `↑`, then `outputTailing` becomes `false`
- [x] Given the output tab is active and tailing, when pressing `↑`, then `outputScroll` is set to one line up from bottom
- [x] Given the output tab is active and not tailing, when pressing `↑`, then `outputScroll` decrements by 1
- [x] Given the output tab is active and at the top, when pressing `↑`, then `outputScroll` stays at 0
- [x] Given output fits in viewport, when pressing `↑`, then nothing happens
- [x] Given a file tab is active, when pressing `↑`, then file tab scrolling works as before

**Definition of Done** (Single Commit):
- [x] `wrapAllOutputLines()` helper function extracted
- [x] `scrollUp()` updated to handle output tab
- [x] Unit tests added for scroll-up behaviour
- [x] All existing tests pass
- [x] Manual verification: pressing `↑` in output tab scrolls up

**Dependencies**:
- Ticket 1 (scroll state must exist)

**Risks**:
- Wrapping calculation must match `renderScrollArea()` exactly or scroll position will be incorrect
- Must handle wrapped lines correctly (scroll by wrapped line, not original line)

**Effort Estimate**: S (2-3 hours)

---

### **Ticket 3: Implement scroll-down with auto-tail re-lock**

**As a** developer using Orbital CLI
**I want** to scroll down to the bottom of the output window and resume auto-tailing
**So that** I can return to watching the latest output after reviewing earlier content

**Context**: After scrolling up to review earlier output, users need a way to return to the bottom and resume auto-tailing. When scrolling down reaches the bottom, the TUI should automatically re-lock to tail mode, ensuring new output is always visible.

**Description**: Update `scrollDown()` to handle output tab scrolling. When already tailing, do nothing (already at bottom). When scrolling, increment offset and detect when reaching bottom. If at bottom, set `outputTailing = true` to resume auto-tailing.

**Implementation Requirements**:
- Remove early return for `m.activeTab == 0` in `scrollDown()`
- Add output tab handling before file tab handling
- When tailing: return early (no-op, already at bottom)
- When not tailing: increment `outputScroll` by 1
- When reaching max offset: set `outputTailing = true` to re-lock
- Use `wrapAllOutputLines()` helper from Ticket 2

**Acceptance Criteria**:
- [x] Given the output tab is active and tailing, when pressing `↓`, then nothing happens
- [x] Given the output tab is active and not tailing, when pressing `↓`, then `outputScroll` increments by 1
- [x] Given the output tab is active and scrolled near bottom, when pressing `↓` to reach bottom, then `outputTailing` becomes `true`
- [x] Given the output tab is active and at bottom, when new output arrives, then the window auto-scrolls (tailing)
- [x] Given a file tab is active, when pressing `↓`, then file tab scrolling works as before

**Definition of Done** (Single Commit):
- [x] `scrollDown()` updated to handle output tab
- [x] Unit tests added for scroll-down and re-lock behaviour
- [x] All existing tests pass
- [x] Manual verification: pressing `↓` to bottom resumes auto-tailing

**Dependencies**:
- Ticket 1 (scroll state must exist)
- Ticket 2 (`wrapAllOutputLines()` helper)

**Risks**:
- Must correctly detect "at bottom" condition to avoid off-by-one errors
- Re-lock logic must trigger reliably

**Effort Estimate**: S (2-3 hours)

---

### **Ticket 4: Add page up/down scrolling for output window**

**As a** developer using Orbital CLI
**I want** to page up/down in the output window using PgUp/PgDown keys
**So that** I can quickly navigate through large amounts of output

**Context**: The `scrollPageUp()` and `scrollPageDown()` functions currently skip the output tab. Page scrolling should jump by the viewport height (e.g., 20 lines if viewport is 20 lines tall), allowing rapid navigation through long output.

**Description**: Update `scrollPageUp()` and `scrollPageDown()` to handle output tab scrolling. Page up moves scroll offset up by `layout.ScrollAreaHeight` lines (clamping to 0). Page down moves scroll offset down by viewport height and re-locks to tail if reaching bottom.

**Implementation Requirements**:
- Update `scrollPageUp()` to handle output tab
- Update `scrollPageDown()` to handle output tab
- Page up: similar to scroll-up but decrement by `m.layout.ScrollAreaHeight`
- Page down: similar to scroll-down but increment by `m.layout.ScrollAreaHeight`
- Page down should re-lock to tail when reaching bottom
- Clamp scroll offset to valid range `[0, maxOffset]`

**Acceptance Criteria**:
- [ ] Given the output tab is active and tailing, when pressing `PgUp`, then scroll jumps up one page
- [ ] Given the output tab is active and tailing, when pressing `PgUp`, then `outputTailing` becomes `false`
- [ ] Given the output tab is active and scrolled, when pressing `PgUp` near top, then scroll clamps to 0
- [ ] Given the output tab is active and scrolled, when pressing `PgDown`, then scroll jumps down one page
- [ ] Given the output tab is active and scrolled, when pressing `PgDown` to reach bottom, then `outputTailing` becomes `true`
- [ ] Given a file tab is active, when pressing `PgUp`/`PgDown`, then file tab scrolling works as before

**Definition of Done** (Single Commit):
- [ ] `scrollPageUp()` updated to handle output tab
- [ ] `scrollPageDown()` updated to handle output tab
- [ ] Unit tests added for page scrolling
- [ ] All existing tests pass
- [ ] Manual verification: page scrolling jumps by viewport height

**Dependencies**:
- Ticket 1 (scroll state must exist)
- Ticket 2 (`wrapAllOutputLines()` helper)
- Ticket 3 (scroll-down re-lock logic)

**Risks**:
- Page boundary calculations must account for wrapped lines
- Edge case: page size larger than total output

**Effort Estimate**: S (2-3 hours)

---

### **Ticket 5: Respect scroll state in renderScrollArea**

**As a** developer using Orbital CLI
**I want** the output window to render the scrolled viewport correctly
**So that** I can see the lines I scrolled to instead of always seeing the tail

**Context**: The `renderScrollArea()` function in `internal/tui/model.go:685-713` currently always renders the tail of the output buffer. It calculates `startIdx` to show the most recent lines regardless of user scroll position. This function must respect the scroll state added in previous tickets.

**Description**: Update `renderScrollArea()` to check scroll state and render the appropriate viewport. When `outputTailing == true`, use current behaviour (show tail). When `outputTailing == false`, use `outputScroll` as the start index.

**Implementation Requirements**:
- Update `renderScrollArea()` to check `m.outputTailing`
- When tailing: calculate `startIdx` from tail (current behaviour)
- When not tailing: use `m.outputScroll` as `startIdx`
- Clamp `startIdx` to valid range (handle edge cases)
- Reuse `wrapAllOutputLines()` helper to avoid duplication
- Ensure rendering uses same wrapping logic as scroll calculations (consistency critical)

**Acceptance Criteria**:
- [ ] Given `outputTailing == true`, when rendering, then the most recent lines are shown
- [ ] Given `outputTailing == false` and `outputScroll == 0`, when rendering, then the first lines are shown
- [ ] Given `outputTailing == false` and `outputScroll == 50`, when rendering, then lines starting at offset 50 are shown
- [ ] Given terminal is resized, when `outputScroll` is invalid for new size, then it is clamped to valid range
- [ ] Given output buffer is shorter than viewport, when rendering, then all output is shown with padding

**Definition of Done** (Single Commit):
- [ ] `renderScrollArea()` updated to respect scroll state
- [ ] Unit tests added for rendering with different scroll states
- [ ] All existing tests pass
- [ ] Manual verification: scrolling changes visible output

**Dependencies**:
- Ticket 1 (scroll state must exist)
- Ticket 2 (`wrapAllOutputLines()` helper and scroll-up logic)
- Ticket 3 (scroll-down logic)

**Risks**:
- Wrapping logic must match between scroll calculations and rendering
- Off-by-one errors can cause jittery scrolling or incorrect viewport
- Terminal resize during scroll can cause scroll position to become invalid

**Effort Estimate**: S (2-3 hours)

---

### **Ticket 6: Handle terminal resize during scrollback**

**As a** developer using Orbital CLI
**I want** the scroll position to remain stable when I resize my terminal
**So that** I don't lose my place when adjusting the window

**Context**: When the terminal is resized, line wrapping changes. A line that was 3 wrapped lines at 80 characters wide might become 2 wrapped lines at 120 characters wide. The `outputScroll` offset (measured in wrapped lines) becomes invalid and needs recalculation or clamping.

**Description**: Update the `tea.WindowSizeMsg` handler to recalculate scroll offset validity when not tailing. Clamp `outputScroll` to the new maximum offset based on the new viewport height and re-wrapped output lines.

**Implementation Requirements**:
- Update `tea.WindowSizeMsg` case in `Update()` function
- After layout recalculation, check if `outputTailing == false`
- If scrolling: recalculate max offset with new dimensions
- Clamp `outputScroll` to `[0, maxOffset]`
- If output now fits in viewport: optionally reset to tailing

**Acceptance Criteria**:
- [ ] Given user is scrolled and terminal is resized larger, when rendering, then scroll position is maintained or clamped
- [ ] Given user is scrolled and terminal is resized smaller, when rendering, then scroll position is clamped to valid range
- [ ] Given user is scrolled and terminal height increases enough to fit all output, when rendering, then output is shown (optionally resume tailing)
- [ ] Given user is tailing and terminal is resized, when rendering, then tailing continues (no change)

**Definition of Done** (Single Commit):
- [ ] `tea.WindowSizeMsg` handler updated with scroll position clamping
- [ ] Unit tests added for resize scenarios
- [ ] All existing tests pass
- [ ] Manual verification: resize during scroll maintains or clamps position gracefully

**Dependencies**:
- Ticket 1 (scroll state must exist)
- Ticket 2 (`wrapAllOutputLines()` helper)
- Ticket 5 (rendering must respect scroll state)

**Risks**:
- Wrapping changes on resize make "maintain exact line" challenging
- Best effort: clamp to valid range, accept some scroll position shift

**Effort Estimate**: S (1-2 hours)

---

## Backlog Prioritisation

**Must Have:**
- Ticket 1: Add output scroll state to TUI model
- Ticket 2: Implement scroll-up with auto-tail unlock
- Ticket 3: Implement scroll-down with auto-tail re-lock
- Ticket 5: Respect scroll state in renderScrollArea

**Should Have:**
- Ticket 4: Add page up/down scrolling for output window
- Ticket 6: Handle terminal resize during scrollback

**Could Have:**
- Scroll position indicator (e.g., "Line 150-180 of 500")
- Jump to top/bottom keybindings (`g` / `G`)
- Search in output (`/` to search)

**Won't Have:**
- Copy mode (select and copy text)
- Persistent scroll on new output (when scrolled, new output pushes buffer but doesn't change viewport)

## Technical Considerations

**Scroll state isolation**: The scroll state (`outputScroll`, `outputTailing`) is independent of session state persistence. It's ephemeral UI state that doesn't need to survive across Orbital restarts.

**Consistency with file tabs**: The implementation pattern mirrors file tab scrolling (`fileScroll` map, `scrollUp()`/`scrollDown()` functions). This provides implementation guidance and UX consistency.

**Line wrapping**: The critical correctness requirement is that scroll calculations and rendering use identical line wrapping logic. Extract `wrapAllOutputLines()` helper to ensure single source of truth.

**Performance**: Wrapping the entire output buffer on every render may be expensive for very long sessions (thousands of lines). If performance issues arise, consider:
1. Memoizing wrapped lines (invalidate on terminal resize or new output)
2. Lazy wrapping (only wrap visible viewport + some buffer)

For MVP, simple implementation is acceptable. Optimize if needed.

## Success Metrics

- Users can scroll up in output window with `↑`/`k` keys
- Users can scroll down in output window with `↓`/`j` keys
- Users can page scroll with `PgUp`/`PgDown` keys
- Output auto-tails by default (preserves current UX)
- Scrolling up unlocks auto-tail
- Scrolling to bottom re-locks auto-tail
- Terminal resize maintains scroll position or clamps gracefully
- No regression in file tab scrolling
- All existing tests pass
- Manual testing confirms smooth scrolling during long Orbital sessions
