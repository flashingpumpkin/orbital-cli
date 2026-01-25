# User Stories: Fix File Tabs Initialization

**Date:** 2026-01-25
**Status:** Ready for Implementation
**Related Commit:** 0e5c9af3e1ac4432f95516094cb16ed5f04e6699

## Problem Statement

File tabs feature implemented in commit 0e5c9af is not displaying in the TUI. Technical analysis shows tabs are never built during initialization because `buildTabs()` is only called when handling `SessionMsg`, but the session is set directly in `tui.New()` instead of via a message.

## Root Cause

Location: `internal/tui/program.go:29-30`

```go
model := NewModel()
model.session = session  // Direct assignment bypasses SessionMsg handler
```

The `buildTabs()` call in the `SessionMsg` handler (model.go:236) never executes during initialization, leaving only the default Output tab.

## User Stories

### Story 1: File Tabs Display on Startup
**As a** user running orbital with spec files
**I want** to see tabs for all my spec, notes, and context files in the TUI
**So that** I can navigate between different files during execution

**Acceptance Criteria:**
- [x] When orbital starts with a spec file, tab bar shows "1:Output" and "2:Spec: [filename]"
- [x] When notes file exists, tab bar includes "Notes" tab
- [x] When context files are specified, tab bar includes "Ctx: [filename]" tabs for each
- [x] Tabs appear immediately when TUI starts, not after first iteration
- [x] Tab numbering (1-9) for keyboard navigation is correct

**Implementation:**
- File: `internal/tui/program.go`
- After line 30 (`model.session = session`), add: `model.tabs = model.buildTabs()`

### Story 2: Multiple Spec Files Show Separate Tabs
**As a** user running orbital with multiple spec files
**I want** each spec file to have its own tab
**So that** I can review different specifications during execution

**Acceptance Criteria:**
- [x] Each spec file in `session.SpecFiles` gets its own tab
- [x] Tab names show "Spec: [basename]" format
- [x] Tabs are numbered in order (2, 3, 4...) after Output tab
- [x] Clicking or pressing number keys (2, 3, 4) switches to correct spec file

**Testing Command:**
```bash
./orbital --spec docs/plans/spec1.md --spec docs/plans/spec2.md
```

Expected: `1:Output│2:Spec: spec1.md│3:Spec: spec2.md│4:Notes│...`

### Story 3: Context Files Display as Tabs
**As a** user with context files configured
**I want** to see context files as separate tabs
**So that** I can review the context being provided to Claude

**Acceptance Criteria:**
- [x] Context files specified via `--context` flag appear as tabs
- [x] Multiple context files (comma-separated) each get their own tab
- [x] Tab names show "Ctx: [basename]" format
- [x] Context tabs appear after spec and notes tabs
- [x] Both ", " and "," separators in context file list are handled correctly

**Testing Command:**
```bash
./orbital --spec spec.md --context CLAUDE.md,README.md
```

Expected: `1:Output│2:Spec: spec.md│3:Notes│4:Ctx: CLAUDE.md│5:Ctx: README.md`

### Story 4: Tab Behavior Remains Consistent
**As a** user interacting with file tabs
**I want** all tab features (switching, scrolling, reloading) to work correctly
**So that** I can navigate files during execution

**Acceptance Criteria:**
- [x] Keyboard navigation (1-9, Tab, arrows, h/l) works for all tabs
- [x] Mouse clicking on tabs switches to correct tab
- [x] File content loads on-demand when tab is selected
- [x] Arrow keys (up/down, k/j) scroll file content
- [x] Page up/down scrolls by viewport height
- [x] 'r' key reloads current file tab content
- [x] Line numbers display correctly in file tabs

**Regression Test:**
All existing tab functionality from commit 0e5c9af must continue working after the fix.

## Technical Implementation

**Change Location:** `internal/tui/program.go:22-54` in the `New()` function

**Before:**
```go
model := NewModel()
model.session = session
model.progress = progress
if len(worktree) > 0 {
    model.worktree = worktree[0]
}
```

**After:**
```go
model := NewModel()
model.session = session
model.tabs = model.buildTabs()  // Add this line
model.progress = progress
if len(worktree) > 0 {
    model.worktree = worktree[0]
}
```

## Verification Plan

### Manual Testing
1. Run orbital with single spec file:
   ```bash
   ./orbital --spec docs/plans/2026-01-25-105100-stories-fix-file-tabs-initialization.md
   ```
   Verify: Tab bar shows Output and Spec tabs

2. Run with multiple specs and context:
   ```bash
   ./orbital --spec spec1.md --spec spec2.md --context CLAUDE.md
   ```
   Verify: All files appear as tabs

3. Test tab switching:
   - Press 1, 2, 3 to switch tabs
   - Press Tab to cycle forward
   - Press Shift+Tab to cycle backward
   - Click tabs with mouse

4. Test file tab scrolling:
   - Switch to file tab (press 2)
   - Press k/up to scroll up
   - Press j/down to scroll down
   - Press PgUp/PgDown for page scrolling
   - Press r to reload file

### Integration Testing
Run orbital in worktree mode and verify tabs display correctly:
```bash
./orbital --worktree --spec docs/plans/some-spec.md
```

## Definition of Done

- [x] Tabs display correctly on TUI startup with all session files
- [x] All keyboard shortcuts for tab navigation work
- [x] Mouse clicking on tabs works
- [x] File content loading and scrolling works in file tabs
- [x] Manual testing passes all verification scenarios
- [x] No regression in existing TUI functionality
- [x] Build completes without errors: `go build ./cmd/orbital`
- [x] Code follows existing patterns in `internal/tui/`

## Notes

This is a minimal one-line fix that aligns the initialization path with the runtime update path. The `buildTabs()` function already exists and works correctly - it just wasn't being called during initialization.

All tab rendering, switching, and interaction code from commit 0e5c9af is correct and requires no changes.
