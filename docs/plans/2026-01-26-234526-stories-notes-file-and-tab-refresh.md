# User Stories: Notes File Auto-Creation, Tab Refresh, Workflow Fix, and Config Changes

## Project Overview

Orbital CLI is a Go tool that runs Claude Code in a loop for autonomous iteration. It provides a Bubbletea-based TUI that displays spec files, notes files, and context files in tabs. This plan addresses gaps in the current implementation:

1. **Notes file auto-creation**: When the CLI starts and the notes file does not exist, it should be created automatically
2. **Tab content refresh**: File tabs (notes, spec files) should automatically refresh to show the latest content when files are modified externally
3. **Autonomous workflow exit bug**: The autonomous workflow does not exit when the completion promise is output during workflow steps
4. **Iteration timeout default**: The 30-minute default is too long; reduce to 5 minutes

## Story Mapping Overview

**Epic 1**: File Management Improvements

| Priority | Story | Value |
|----------|-------|-------|
| Must Have | Auto-create notes file on startup | Users can immediately see notes tab without manual file creation |
| Must Have | Auto-refresh file tabs periodically | Users always see current file content without manual reload |

**Epic 2**: Workflow and Configuration Fixes

| Priority | Story | Value |
|----------|-------|-------|
| Must Have | Fix autonomous workflow completion promise detection | Autonomous workflow exits correctly when work is complete |
| Must Have | Reduce default iteration timeout to 5 minutes | Faster feedback on stuck iterations |

## Epic: File Management Improvements

### [x] **Ticket: Auto-create notes file when CLI starts**

**As a** user
**I want** the notes file to be created automatically when I start the CLI
**So that** I can immediately use the notes tab without manually creating the file

**Context**: Currently, the notes file path is generated but the file itself is not created. Users see an error when clicking the Notes tab if the file does not exist. The notes file should be created with a sensible default header when the CLI starts.

**Description**: When Orbital CLI starts, after determining the notes file path (either from `--notes` flag or auto-generated), check if the file exists. If not, create the parent directory if needed and create the file with a minimal header containing the date and spec file reference.

**Implementation Requirements**:
- Check if notes file exists after path is determined in `cmd/orbital/root.go`
- Create parent directories using `os.MkdirAll` if they do not exist
- Create notes file with minimal header: `# Notes\n\nSpec: <spec-file-name>\nDate: <YYYY-MM-DD>\n`
- Handle errors gracefully (log warning but do not fail startup)
- Preserve existing file content if file already exists

**Acceptance Criteria**:
- [x] Given the CLI starts with a notes file path that does not exist, when the TUI loads, then the notes file is created with a header
- [x] Given the CLI starts with a notes file that already exists, when the TUI loads, then the existing content is preserved
- [x] Given the notes file parent directory does not exist, when the CLI starts, then the directory is created
- [x] Given file creation fails (permissions), when the CLI starts, then a warning is logged but startup continues

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] Unit test for notes file creation logic
- [x] Integration with existing root.go startup flow
- [x] Error handling for permission issues
- [x] All tests passing (`make check`)

**Dependencies**:
- None (standalone change)

**Risks**:
- File permission issues on some systems (mitigated by logging warning)
- Accidental overwrite of existing files (mitigated by existence check)

**Notes**: The notes file creation should happen after `spec.NotesFile` is set but before the TUI program starts. This ensures the file exists when the Notes tab is first accessed.

**Effort Estimate**: XS (1-2 hours)

---

### [x] **Ticket: Add periodic file content refresh for spec and notes tabs**

**As a** user
**I want** the spec and notes file tabs to automatically show the latest content
**So that** I can see changes made by Claude or external editors without manually pressing 'r'

**Context**: Currently, file content is loaded once when a tab is first clicked and cached in `fileContents` map. Users must press 'r' to reload. During autonomous loops, Claude frequently modifies spec files (checking off items) and notes files. Users should see these changes automatically.

**Description**: Implement a periodic refresh mechanism that reloads the content of the currently active file tab at a configurable interval. When the active tab is a file tab (spec or notes), schedule a refresh every N seconds. Compare the file modification time before reloading to avoid unnecessary reads.

**Implementation Requirements**:
- Add a `tea.Tick` command that fires every 2 seconds when a file tab is active
- Track file modification times in a `fileModTimes map[string]time.Time`
- On tick, check if active tab is a file tab and if file mtime has changed
- If mtime changed, trigger reload via existing `loadFileCmd`
- Preserve scroll position when content refreshes (viewport already handles this)
- Do not refresh when on Output tab (index 0)

**Acceptance Criteria**:
- [x] Given a spec file tab is active and the file is modified externally, when 2 seconds pass, then the tab content updates to show new content
- [x] Given a notes file tab is active and Claude appends to it, when 2 seconds pass, then the new content is visible
- [x] Given the file has not been modified, when the tick fires, then no reload occurs (mtime check)
- [x] Given the Output tab is active, when the tick fires, then no file refresh is attempted
- [x] Given the user scrolls within a file, when content refreshes, then scroll position is preserved

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] Tick-based refresh logic in model.go Update function
- [x] Modification time tracking to avoid unnecessary reloads
- [x] Unit test for mtime comparison logic
- [x] All tests passing (`make check`)

**Dependencies**:
- Depends on existing `loadFileCmd` and `FileContentMsg` handling
- Uses existing viewport infrastructure for scroll preservation

**Risks**:
- High-frequency file reads could impact performance (mitigated by mtime check)
- Race conditions if file is being written while read (mitigated by Go's atomic file reads)

**Notes**: The 2-second interval balances responsiveness with performance. Could be made configurable via flag in future. The mtime check ensures we only read files that have actually changed.

**Effort Estimate**: S (2-3 hours)

---

## Epic: Workflow and Configuration Fixes

### [x] **Ticket: Fix gate-based workflows to check completion promise during steps**

**As a** user
**I want** workflows with gates to exit when the completion promise is output
**So that** the loop terminates correctly when all work is complete

**Context**: All workflows with gates (`fast`, `reviewed`, `tdd`, `autonomous`) are affected. Currently, the completion promise (`<promise>COMPLETE</promise>`) is only checked AFTER all workflow steps complete and verification runs. If Claude outputs the promise during any workflow step, it is ignored. This causes the workflow to continue looping even when work is done.

The root cause is in `cmd/orbital/root.go`: the `runWorkflowLoop()` function only checks for completion after `CompletedAllSteps = true`, unlike `controller.Run()` which checks every iteration.

**Affected workflows:**
- `fast` (has review gate)
- `reviewed` (has review gate)
- `tdd` (has review gate)
- `autonomous` (has review gate)

**Unaffected:** `spec-driven` (no gates, uses standard loop with promise checking)

**Description**: Add completion promise detection within the workflow loop. After each step executes, check if the output contains the completion promise. If found, skip remaining steps and proceed to verification. This aligns workflow behavior with the standard loop.

**Implementation Requirements**:
- In `runWorkflowLoop()` (cmd/orbital/root.go ~line 850), after each step result:
  - Check `stepResult.Output` for completion promise using `completion.Detect()`
  - If promise found, set a flag to exit the workflow step loop
  - Proceed directly to verification phase
- Import and use existing `completion.Detect()` from `internal/completion`
- Preserve existing gate retry logic (promise detection should take precedence)
- Log when promise is detected mid-workflow for debugging

**Acceptance Criteria**:
- [x] Given the autonomous workflow is running and Claude outputs `<promise>COMPLETE</promise>` during the implement step, when the step completes, then the workflow exits and runs verification
- [x] Given the autonomous workflow is running and Claude outputs the promise during the review step, when the step completes, then the workflow exits and runs verification
- [x] Given the autonomous workflow is running and no promise is output, when steps complete normally, then gate logic continues as before
- [x] Given the promise is detected, when verification runs, then it validates spec completion as normal

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] Promise detection added to workflow loop
- [x] Unit test for promise detection in workflow context
- [x] Existing workflow tests still pass
- [x] All tests passing (`make check`)

**Dependencies**:
- Uses existing `completion.Detect()` function
- Integrates with existing `runWorkflowLoop()` in root.go

**Risks**:
- False positive promise detection in tool output (mitigated: promise is specific XML tag)
- Breaking existing gate logic (mitigated: promise check is additive, gates still work)

**Notes**: The `implement` step prompt currently tells Claude NOT to output the completion promise. This fix allows the promise to work if Claude does output it (e.g., when spec is already complete). The prompt guidance remains as a best practice but is no longer a hard requirement.

**Effort Estimate**: S (2-3 hours)

---

### [ ] **Ticket: Fix cost display resetting to zero when step/iteration starts**

**As a** user
**I want** the cost display to show accumulated cost continuously
**So that** I can track spending during long-running steps without the display resetting

**Context**: The TUI cost display (`$X.XX/$100.00`) resets to `$0.00` every time a new step or iteration starts. This happens because `ProgressInfo` messages sent at step/iteration start do not include the accumulated cost/token values, and the TUI's `ProgressMsg` handler replaces the entire progress struct (setting missing fields to zero).

**Root Cause Analysis**:
1. Step start callback (root.go:777-784) sends `ProgressInfo` without Cost/TokensIn/TokensOut
2. Iteration start callback (root.go:345-350) sends `ProgressInfo` without Cost/TokensIn/TokensOut
3. TUI model.go:213-214 does `m.progress = ProgressInfo(msg)` which replaces entire struct
4. Any field not set in the message becomes 0

**Implementation Requirements**:
- In workflow step start callback (root.go:777-784): Include `loopState.TotalCost`, `loopState.TotalTokensIn`, `loopState.TotalTokensOut`
- In workflow step callback (root.go:817-829): Already correct (includes cost)
- For standard loop iteration start callback: Track accumulated cost at root.go level and include in ProgressInfo
- Alternative: Change TUI to merge ProgressMsg fields instead of replacing entire struct

**Acceptance Criteria**:
- [ ] Given a workflow is running and step 1 completes with $0.50 cost, when step 2 starts, then cost display shows $0.50 (not $0.00)
- [ ] Given iteration 1 completes with $1.00 cost, when iteration 2 starts, then cost display shows $1.00 (not $0.00)
- [ ] Given Claude is streaming output, when cost is not yet available, then display shows previous accumulated cost (not $0.00)

**Definition of Done** (Single Commit):
- [ ] Feature complete in one atomic commit
- [ ] Step start callbacks include accumulated cost/tokens
- [ ] Iteration start callbacks include accumulated cost/tokens
- [ ] Cost display never resets to zero mid-session
- [ ] All tests passing (`make check`)

**Dependencies**:
- None (standalone fix)

**Risks**:
- None significant

**Notes**: The `StatsMsg` from the Bridge does provide streaming updates for tokens (from `assistant` events), but cost data only arrives in the `result` event at the end of a Claude invocation. This fix ensures the ACCUMULATED cost persists across step/iteration boundaries.

**Effort Estimate**: XS (1 hour)

---

### [x] **Ticket: Reduce default iteration timeout from 30 minutes to 5 minutes**

**As a** user
**I want** iterations to timeout after 5 minutes by default instead of 30
**So that** I get faster feedback when an iteration is stuck or unresponsive

**Context**: The current 30-minute default iteration timeout is too long. Most productive iterations complete well under 5 minutes. A 30-minute timeout means users wait excessively long before learning an iteration is stuck. Reducing to 5 minutes provides faster feedback while still allowing complex operations to complete.

**Description**: Change the default `IterationTimeout` from 30 minutes to 5 minutes in both the config defaults and the CLI flag default. Users can still override with `--timeout` flag if needed.

**Implementation Requirements**:
- Update `internal/config/config.go` line 83: change `30 * time.Minute` to `5 * time.Minute`
- Update `cmd/orbital/root.go` line 106: change flag default from `30*time.Minute` to `5*time.Minute`
- Update `CLAUDE.md` documentation to reflect new default
- Update any tests that assert the 30-minute default

**Acceptance Criteria**:
- [x] Given no `--timeout` flag is provided, when the CLI starts, then IterationTimeout is 5 minutes
- [x] Given `--timeout 10m` flag is provided, when the CLI starts, then IterationTimeout is 10 minutes
- [x] Given an iteration runs longer than 5 minutes, when no flag override is set, then the iteration times out

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] Config default updated
- [x] CLI flag default updated
- [x] CLAUDE.md updated
- [x] Tests updated for new default
- [x] All tests passing (`make check`)

**Dependencies**:
- None (standalone change)

**Risks**:
- Complex iterations may timeout unexpectedly (mitigated: users can use `--timeout` flag)
- Breaking change for users expecting 30m default (mitigated: document in release notes)

**Notes**: The 5-minute default is a balance between responsiveness and allowing complex operations. Users running large refactors or slow operations should use `--timeout 30m` explicitly.

**Effort Estimate**: XS (30 minutes)

---

## Backlog Prioritisation

**Must Have (Sprint 1):**
1. Reduce default iteration timeout to 5 minutes (XS - quick win) ✓
2. Fix gate-based workflow completion promise detection (S - bug fix) ✓
3. Fix cost display resetting to zero when step/iteration starts (XS - bug fix)
4. Auto-create notes file when CLI starts (XS)
5. Add periodic file content refresh for spec and notes tabs (S)

**Should Have (Future):**
- Configurable refresh interval via flag
- Visual indicator when file content has changed
- fsnotify-based watching (more efficient than polling)

**Could Have (Future):**
- Live diff view showing what changed
- Notification when spec items are checked off

**Won't Have:**
- Real-time collaborative editing
- File locking mechanism

## Technical Considerations

### File Creation Location
The notes file creation should happen in `cmd/orbital/root.go` around line 232, immediately after `spec.NotesFile` is set. This keeps all file setup logic in one place.

### Tick-Based Refresh Architecture
```go
// New message type
type fileRefreshTickMsg time.Time

// Schedule tick when switching to file tab
func fileRefreshTick() tea.Cmd {
    return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
        return fileRefreshTickMsg(t)
    })
}

// Handle in Update
case fileRefreshTickMsg:
    if m.activeTab > 0 && tab.Type == TabFile {
        // Check mtime and reload if changed
    }
    return m, fileRefreshTick() // Schedule next tick
```

### Modification Time Tracking
Add to Model struct:
```go
fileModTimes map[string]time.Time // Last known mtime per file
```

Check mtime before reload:
```go
info, _ := os.Stat(path)
if info.ModTime().After(m.fileModTimes[path]) {
    m.fileModTimes[path] = info.ModTime()
    return m, loadFileCmd(path)
}
```

## Success Metrics

| Metric | Target |
|--------|--------|
| Notes tab accessible on first click | 100% (no "file not found" errors) |
| File changes visible within | 3 seconds of modification |
| Unnecessary file reads | 0 (mtime gating) |
| Scroll position preserved on refresh | 100% |
