# User Stories: Notes File Auto-Creation, Tab Refresh, Workflow Fix, and Config Changes

## Project Overview

Orbital CLI is a Go tool that runs Claude Code in a loop for autonomous iteration. It provides a Bubbletea-based TUI that displays spec files, notes files, and context files in tabs. This plan addresses gaps in the current implementation:

1. **Notes file auto-creation**: When the CLI starts and the notes file does not exist, it should be created automatically
2. **Tab content refresh**: File tabs (notes, spec files) should automatically refresh to show the latest content when files are modified externally
3. **Autonomous workflow exit bug**: The autonomous workflow does not exit when the completion promise is output during workflow steps
4. **Iteration timeout default**: The 30-minute default is too long; reduce to 5 minutes
5. **Context window progress bar**: Display token usage relative to model context window, updating on each streamed message
6. **Exit summary**: Print a final summary when the app exits (via CTRL-C or completion promise)
7. **Iteration countdown timer**: Display remaining time in the current iteration
8. **Light and dark mode**: Support both terminal themes with auto-detection
9. **Display active workflow**: Show which workflow preset is running in the TUI
10. **Refactor template variables**: Split `{{files}}` into `{{spec_file}}`, `{{context_files}}`, `{{notes_file}}`
11. **Tighten autonomous 'implement' prompt**: Enforce single-task discipline
12. **Tighten autonomous 'fix' prompt**: Only address review feedback, no new tasks

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

**Epic 3**: TUI Improvements

| Priority | Story | Value |
|----------|-------|-------|
| Must Have | Add context window progress bar | Users see real-time token usage relative to model limits |
| Must Have | Print exit summary on TUI shutdown | Users see completion status, cost, and metrics after exit |
| Must Have | Add iteration countdown timer | Users see time remaining before iteration timeout |
| Must Have | Add light and dark mode with auto-detection | TUI is readable on both light and dark terminal backgrounds |
| Must Have | Display active workflow name | Users know which workflow preset is driving the iteration loop |

**Epic 4**: Workflow Improvements

| Priority | Story | Value |
|----------|-------|-------|
| Must Have | Refactor template variables | Distinct placeholders clarify spec vs context vs notes |
| Must Have | Tighten autonomous 'implement' prompt | Enforces single-task discipline per iteration |
| Must Have | Tighten autonomous 'fix' prompt | Prevents scope creep when fixing review issues |

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

### [x] **Ticket: Fix cost display resetting to zero when step/iteration starts**

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
- [x] Given a workflow is running and step 1 completes with $0.50 cost, when step 2 starts, then cost display shows $0.50 (not $0.00)
- [x] Given iteration 1 completes with $1.00 cost, when iteration 2 starts, then cost display shows $1.00 (not $0.00)
- [x] Given Claude is streaming output, when cost is not yet available, then display shows previous accumulated cost (not $0.00)

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] Step start callbacks include accumulated cost/tokens
- [x] Iteration start callbacks include accumulated cost/tokens
- [x] Cost display never resets to zero mid-session
- [x] All tests passing (`make check`)

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

## Epic: TUI Improvements

### [x] **Ticket: Add context window progress bar showing token usage**

**As a** user
**I want** to see a progress bar showing token usage relative to the model's context window
**So that** I can monitor how much context is being consumed and avoid hitting limits

**Context**: The TUI currently displays raw token counts (`Tokens: 1,234 in / 5,678 out`) but provides no visual indication of how close the session is to the model's context window limit. Users cannot easily assess whether they are approaching capacity. A progress bar showing `tokens used / context window` would provide immediate visual feedback.

Claude CLI stream-json output includes token counts in both `assistant` events (intermediate) and `result` events (final). The context window size is not provided in the stream but can be determined from the model being used (opus, sonnet, haiku all have 200K context windows).

**Description**: Add a context window progress bar to the TUI progress panel. The bar shows the ratio of total tokens (input + output) to the model's context window. Update the bar on every streamed message. Reset the bar when a new Claude CLI session starts.

**Implementation Requirements**:

1. **Add model-to-context-window mapping** in `internal/config/config.go`:
   ```go
   var ModelContextWindows = map[string]int{
       "opus":   200000,
       "sonnet": 200000,
       "haiku":  200000,
   }
   ```

2. **Extend ProgressInfo struct** in `internal/tui/model.go`:
   ```go
   type ProgressInfo struct {
       // ... existing fields ...
       ContextWindow int  // Model's context window size
   }
   ```

3. **Pass context window to TUI** from `cmd/orbital/root.go`:
   - Look up context window from `ModelContextWindows[cfg.Model]`
   - Include in `ProgressInfo` sent to TUI

4. **Render context window progress bar** in `internal/tui/view.go`:
   - Calculate ratio: `(TokensIn + TokensOut) / ContextWindow`
   - Use existing `RenderProgressBar()` function
   - Display format: `Context: [████████░░░░░░░░░░░░] 45,678/200,000 (23%)`
   - Position: Line 3 of progress panel (after budget bar)

5. **Update on every StatsMsg** in `internal/tui/model.go`:
   - The existing `StatsMsg` handler already updates `TokensIn` and `TokensOut`
   - Progress bar will automatically reflect new values on next render

6. **Reset on new session**:
   - When `ProgressInfo` is received with `Iteration == 1` and tokens are zero, reset context bar
   - Alternatively: Add explicit `SessionStartMsg` to signal reset

**Acceptance Criteria**:
- [x] Given a Claude CLI session is running, when a message is streamed and parsed, then input/output tokens are extracted
- [x] Given a message contains token data, when the TUI renders, then the context window progress bar reflects the current usage
- [x] Given the model is opus/sonnet/haiku, when the TUI starts, then the context window is set to 200,000
- [x] Given tokens approach 80% of context window, when the TUI renders, then the progress bar displays in warning colour
- [x] Given a new Claude CLI session starts, when the first message arrives, then the progress bar resets to zero

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] Model-to-context-window mapping added to config
- [x] ProgressInfo extended with ContextWindow field
- [x] Context window progress bar rendered in view.go
- [x] Progress bar updates on each StatsMsg
- [x] Warning colour at 80% threshold
- [x] Unit test for ratio calculation
- [x] All tests passing (`make check`)

**Dependencies**:
- Uses existing `RenderProgressBar()` in styles.go
- Uses existing `StatsMsg` flow from bridge.go
- Builds on existing progress panel layout in view.go

**Risks**:
- Unknown model names could lack context window mapping (mitigated: default to 200,000)
- Token counts from `assistant` events are intermediate and may differ from final (mitigated: final `result` event provides authoritative counts)
- Cache tokens (creation + read) inflate input token count (mitigated: this is correct behaviour as cache tokens consume context)

**Notes**: The context window progress bar provides critical visibility into a fundamental constraint. Unlike budget (which is cumulative across iterations), context window usage resets with each Claude CLI invocation since each invocation starts a fresh conversation. The bar should show per-invocation usage, not cumulative across the entire Orbital session.

For future enhancement, consider:
- Displaying context usage per iteration vs cumulative
- Adding context window as a CLI flag for custom model configurations
- Showing cache hit ratio alongside context usage

**Effort Estimate**: S (2-3 hours)

---

### [x] **Ticket: Print exit summary when TUI shuts down**

**As a** user
**I want** to see a summary of the session when the app exits
**So that** I know the completion status, total cost, and key metrics without scrolling through output

**Context**: Currently, TUI mode exits silently without printing any summary. Non-TUI mode prints a summary via `PrintLoopSummary()` and `PrintWorkflowSummary()`. When the TUI exits (either via CTRL-C interrupt or successful completion), users have no immediate visibility into what happened. They must mentally recall the last state of the progress panel or check logs.

The summary should print AFTER the TUI has fully exited, so it appears cleanly in the terminal without interfering with the Bubbletea rendering.

**Description**: After the TUI program exits (via `Quit()` or `Kill()`), print a final summary to stdout. The summary should include completion status, iterations, cost, tokens, duration, and any error information. For workflows, include step-by-step breakdown. For interrupts, include resume instructions.

**Implementation Requirements**:

1. **Ensure TUI cleanup completes** before printing:
   - After `tuiProgram.Quit()` or `tuiProgram.Kill()`, wait for TUI goroutine to finish
   - Call `tuiProgram.Wait()` if available, or use existing channel synchronisation

2. **Print summary in all TUI exit paths** in `cmd/orbital/root.go`:
   - Success path (completion promise verified): Print success summary
   - Error path (budget exceeded, max iterations, timeout): Print error summary
   - Interrupt path (CTRL-C): Print interrupt summary with resume instructions

3. **Summary content** (reuse existing `output.Formatter` methods):
   ```
   ═══════════════════════════════════════════════════════════
   Session Complete
   ═══════════════════════════════════════════════════════════
   Status:     ✓ Completed (or ✗ Interrupted / ✗ Failed)
   Iterations: 5
   Duration:   3m 42s
   Cost:       $1.23
   Tokens:     45,678 in / 12,345 out

   [For workflows with multiple steps:]
   Steps:
     1. implement  ✓  $0.80  (2m 10s)
     2. review     ✓  $0.43  (1m 32s)

   [For interrupts:]
   Resume with: orbital --resume abc123 spec.md
   ═══════════════════════════════════════════════════════════
   ```

4. **Extend existing formatter** in `internal/output/formatter.go`:
   - Add `PrintTUISummary()` method that works after TUI exit
   - Handle case where formatter was used by TUI (colour output safe)

5. **Pass necessary data** from loop to summary:
   - `LoopState` contains iterations, cost, tokens, duration, completion status
   - Workflow step results available from `runWorkflowLoop()` return
   - Session ID for resume instructions

**Acceptance Criteria**:
- [x] Given the TUI is running and I press CTRL-C, when the app exits, then a summary is printed showing "Interrupted" status, accumulated cost, tokens, and resume instructions
- [x] Given the TUI is running and the completion promise is emitted, when the app exits, then a summary is printed showing "Completed" status with final metrics
- [x] Given the TUI is running and budget is exceeded, when the app exits, then a summary is printed showing "Budget Exceeded" status and accumulated metrics
- [x] Given the TUI is running and max iterations reached, when the app exits, then a summary is printed showing "Max Iterations" status
- [ ] Given a workflow with multiple steps completes, when the app exits, then the summary includes per-step breakdown
- [x] Given the TUI is running, when the app exits, then the summary appears AFTER the TUI has fully cleared the screen

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] Summary printed for all TUI exit scenarios (success, error, interrupt)
- [x] Existing `PrintLoopSummary()` reused or extended
- [x] Resume instructions included for interrupts
- [ ] Workflow step breakdown included when applicable
- [x] Summary prints cleanly after TUI exits (no rendering artifacts)
- [x] All tests passing (`make check`)

**Dependencies**:
- Uses existing `LoopState` struct for metrics
- Uses existing `output.Formatter` for styled output
- Integrates with TUI shutdown in root.go

**Risks**:
- TUI may not fully clear screen before summary prints (mitigated: wait for TUI goroutine)
- Summary may be lost if terminal closes immediately (mitigated: flush stdout)
- Colour codes may render incorrectly after TUI (mitigated: reset terminal state)

**Notes**: The summary serves as a "receipt" for the session. Users can screenshot or copy it for records. The resume instructions are critical for interrupted sessions, as users need the session ID to continue. Consider also writing summary to a log file for longer-term record keeping (future enhancement).

**Effort Estimate**: S (2-3 hours)

---

### [x] **Ticket: Add iteration countdown timer to TUI**

**As a** user
**I want** to see a countdown showing time remaining in the current iteration
**So that** I know how long until the iteration times out and can plan accordingly

**Context**: The iteration timeout is configurable via `--timeout` flag (default 5 minutes), but there is no visual indication of time elapsed or remaining. Users cannot tell if an iteration is about to timeout or has just started. This lack of visibility makes it difficult to assess whether an iteration is stuck or simply working on a complex task.

The TUI progress panel currently shows iteration count, budget, and context window. Time remaining would complement these metrics by showing the temporal dimension of iteration progress.

**Description**: Add a countdown timer to the TUI that displays time remaining in the current iteration. The timer should update every second, showing elapsed time and/or remaining time. When time is running low (under 1 minute), the display should change to warning colour. The timer resets when a new iteration starts.

**Important**: The iteration timeout should only apply to implementor workflow steps, not to gate steps. Gates (like `review`) are quick validation checks and should not be subject to the iteration timeout. This prevents a slow review from incorrectly triggering a timeout when the actual implementation work completed successfully.

**Implementation Requirements**:

1. **Extend ProgressInfo struct** in `internal/tui/model.go`:
   ```go
   type ProgressInfo struct {
       // ... existing fields ...
       IterationTimeout  time.Duration // Configured timeout
       IterationStart    time.Time     // When current iteration started
   }
   ```

2. **Pass timeout info to TUI** from `cmd/orbital/root.go`:
   - Include `cfg.IterationTimeout` in ProgressInfo
   - Set `IterationStart` in iteration start callback (line 350)
   - Reset `IterationStart` on each new iteration

3. **Add timer tick** in TUI model:
   ```go
   type timerTickMsg time.Time

   func timerTick() tea.Cmd {
       return tea.Tick(time.Second, func(t time.Time) tea.Msg {
           return timerTickMsg(t)
       })
   }
   ```

4. **Calculate remaining time** in view rendering:
   ```go
   elapsed := time.Since(m.progress.IterationStart)
   remaining := m.progress.IterationTimeout - elapsed
   ```

5. **Display location** (Format A - inline with iteration bar):
   - Position: End of iteration progress line (Line 1 of progress panel)
   - Format: `Iteration 2/50 [████░░░░░░] 3m 42s remaining`
   - Timer appears after the progress bar, naturally extending the iteration info

6. **Warning state** when remaining time < 1 minute:
   - Change timer text to warning colour (orange)
   - Optionally pulse or highlight

7. **Handle edge cases**:
   - Timer shows "—" when no iteration is running
   - Timer shows "0s" (not negative) when timeout exceeded
   - Timer resets cleanly on iteration boundary

**Acceptance Criteria**:
- [x] Given an implementor step is running, when 1 second passes, then the timer updates to show new remaining time
- [x] Given an implementor step starts, when the TUI renders, then the timer shows the full timeout duration as remaining
- [x] Given remaining time drops below 1 minute, when the TUI renders, then the timer displays in warning colour
- [x] Given an iteration completes, when a new iteration starts, then the timer resets to full timeout duration
- [x] Given no iteration is running (between iterations), when the TUI renders, then the timer shows a neutral state or is hidden
- [x] Given the timeout is 5 minutes and 3 minutes have elapsed, when the TUI renders, then the timer shows approximately 2 minutes remaining
- [x] Given a gate step (e.g., review) is running, when the TUI renders, then the timer is hidden or shows "—" (no timeout applies)

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] ProgressInfo extended with timeout and start time fields
- [x] Timer tick implemented in TUI model
- [x] Countdown displayed inline with iteration bar (Format A)
- [x] Timer only active during implementor steps (not gates)
- [x] Warning colour at < 1 minute remaining
- [x] Timer resets on new iteration
- [x] Unit test for remaining time calculation
- [x] All tests passing (`make check`)

**Dependencies**:
- Uses existing progress panel layout in view.go (Line 1 - iteration bar)
- Uses existing tick pattern (similar to file refresh tick)
- Integrates with iteration callbacks in root.go
- Requires step type information (implementor vs gate) passed to TUI

**Risks**:
- High-frequency ticks (every second) could impact TUI performance (mitigated: single tick, minimal work)
- Timer drift if system clock changes (mitigated: use monotonic time via time.Since)
- Iteration start time not available if iteration crashes before callback (mitigated: show "—")

**Notes**: The countdown timer provides critical temporal awareness during long iterations. Combined with the existing iteration count and budget displays, users have complete visibility into session constraints. Consider making the timer format configurable in future (elapsed vs remaining vs both).

For future enhancement:
- Add audio/visual alert when timeout approaches
- Show iteration duration after completion in summary
- Configurable warning threshold (currently hardcoded to 1 minute)

**Effort Estimate**: S (2-3 hours)

---

### [x] **Ticket: Add light and dark mode with auto-detection**

**As a** user
**I want** the TUI to automatically adapt to my terminal's colour scheme
**So that** text is readable whether I use a light or dark terminal background

**Context**: The current TUI uses an amber colour palette designed exclusively for dark terminal backgrounds. Users with light terminal themes (white or light grey backgrounds) experience poor contrast and readability issues. The amber colours that look vibrant on black become washed out or invisible on white.

The project already uses lipgloss/termenv which provides `HasDarkBackground()` for automatic theme detection. This allows the TUI to detect the terminal background and select an appropriate colour palette without user configuration.

**Description**: Implement dual colour palettes (light and dark) with automatic detection at startup. The dark palette retains the current amber theme. The light palette uses darker, more saturated variants of the same colours for visibility on light backgrounds. Users can override auto-detection via CLI flag or config file.

**Implementation Requirements**:

1. **Create theme infrastructure** in new file `internal/tui/themes.go`:
   ```go
   type Theme string
   const (
       ThemeAuto  Theme = "auto"
       ThemeDark  Theme = "dark"
       ThemeLight Theme = "light"
   )

   func DetectTheme() Theme {
       if termenv.HasDarkBackground() {
           return ThemeDark
       }
       return ThemeLight
   }
   ```

2. **Define light colour palette** in `internal/tui/styles.go`:
   ```go
   // Dark theme (current - for dark backgrounds)
   ColourAmber      = lipgloss.Color("#FFB000")  // Bright amber
   ColourAmberLight = lipgloss.Color("#FFD966")  // Light amber

   // Light theme (new - for light backgrounds)
   ColourAmberDark    = lipgloss.Color("#8B6914")  // Dark amber
   ColourAmberDarkDim = lipgloss.Color("#5C4A0A")  // Darker amber
   ```

3. **Create style factory functions**:
   ```go
   func DarkStyles() Styles { ... }   // Current amber-on-black
   func LightStyles() Styles { ... }  // Dark amber-on-white
   func GetStyles(theme Theme) Styles {
       if theme == ThemeLight {
           return LightStyles()
       }
       return DarkStyles()
   }
   ```

4. **Add theme to config** in `internal/config/config.go`:
   ```go
   type Config struct {
       // ... existing fields ...
       Theme string  // "auto", "light", "dark"
   }
   ```

5. **Add CLI flag** in `cmd/orbital/root.go`:
   ```go
   rootCmd.Flags().String("theme", "auto", "Colour theme: auto, light, dark")
   ```

6. **Apply theme at startup** in `internal/tui/program.go`:
   - Detect theme if "auto"
   - Pass resolved theme to model creation
   - Apply before TUI starts

7. **Update session selector** in `internal/tui/selector/styles.go`:
   - Apply same theme logic to selector component

**Light Theme Colour Mapping**:

| Element | Dark Theme | Light Theme |
|---------|------------|-------------|
| Primary (headers, active) | #FFB000 (bright amber) | #8B6914 (dark amber) |
| Secondary (labels) | #B38F00 (amber faded) | #5C4A0A (darker amber) |
| Body text | #FFD966 (light amber) | #6B5A1E (medium amber) |
| Borders active | #FFB000 | #8B6914 |
| Borders inactive | #996600 | #A08050 |
| Success | #00FF00 | #008000 |
| Warning | #FFAA00 | #CC5500 |
| Error | #FF3300 | #CC0000 |
| Background | (terminal default) | (terminal default) |

**Acceptance Criteria**:
- [x] Given a dark terminal background, when the TUI starts with theme "auto", then the dark colour palette is applied
- [x] Given a light terminal background, when the TUI starts with theme "auto", then the light colour palette is applied
- [x] Given `--theme dark` flag, when the TUI starts, then the dark palette is used regardless of terminal background
- [x] Given `--theme light` flag, when the TUI starts, then the light palette is used regardless of terminal background
- [x] Given `theme = "light"` in config file, when the TUI starts without flag, then the light palette is used
- [x] Given the light palette is active, when viewing the TUI on a light terminal, then all text has sufficient contrast for readability
- [x] Given the session selector is displayed, when theme is applied, then selector uses matching colour palette

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] Theme detection using `termenv.HasDarkBackground()`
- [x] Dark palette (current amber theme) preserved
- [x] Light palette with adjusted colours created
- [x] CLI flag `--theme` added
- [x] Config file support for theme setting
- [x] Session selector themed consistently
- [x] Fallback to dark theme if detection fails
- [x] Unit test for theme detection logic
- [x] All tests passing (`make check`)

**Dependencies**:
- Uses existing `termenv` package (already a dependency)
- Uses existing `lipgloss` colour system
- Integrates with existing config and CLI flag infrastructure

**Risks**:
- Theme detection may fail on some terminals (mitigated: default to dark)
- Light palette colours may need tuning after user testing (mitigated: design with sufficient contrast)
- Session selector has duplicated style definitions (mitigated: update both files)

**Notes**: The amber identity is preserved in both themes. Auto-detection uses `termenv.HasDarkBackground()` which queries the terminal via OSC sequences or falls back to environment variables like `COLORFGBG`. When detection is uncertain, dark mode is the safer default since it matches the original design.

For future enhancement:
- Custom theme colours via config file
- High contrast mode for accessibility
- Theme switching at runtime (currently requires restart)

**Effort Estimate**: M (3-4 hours)

---

### [x] **Ticket: Display active workflow name in TUI**

**As a** user
**I want** to see which workflow is currently running in the TUI
**So that** I understand the execution pattern and know what steps to expect

**Context**: The TUI currently shows step information (step name, position, total steps) but does not display which workflow preset is active. Users running different workflows (fast, spec-driven, reviewed, tdd, autonomous) have no visual confirmation of which one is executing. This is especially important when workflows are configured via config file rather than CLI flag.

**Description**: Display the workflow name in the TUI, either in the header panel or session info panel. The workflow name should be visible at all times so users can confirm the execution pattern. For the default `spec-driven` workflow, consider showing "spec-driven" or omitting it to reduce noise.

**Implementation Requirements**:

1. **Extend ProgressInfo struct** in `internal/tui/model.go`:
   ```go
   type ProgressInfo struct {
       // ... existing fields ...
       WorkflowName string  // e.g., "autonomous", "tdd", "reviewed"
   }
   ```

2. **Pass workflow name to TUI** from `cmd/orbital/root.go`:
   - Include `workflow.Name` in ProgressInfo when workflow is active
   - For standard loop (no workflow), use "spec-driven" or empty string

3. **Display in session info panel** (alongside spec file paths):
   ```
   Spec: docs/plans/my-spec.md
   Workflow: autonomous
   ```

   Or in header panel:
   ```
   orbital v1.0.0  [autonomous]  Session: abc123
   ```

4. **Handle default workflow**:
   - Option A: Always show workflow name
   - Option B: Only show non-default workflows (hide "spec-driven")

**Acceptance Criteria**:
- [x] Given a workflow is configured, when the TUI starts, then the workflow name is displayed
- [x] Given the `--workflow autonomous` flag, when the TUI renders, then "autonomous" is visible
- [x] Given a workflow in config file, when the TUI renders, then the workflow name from config is shown
- [x] Given no workflow is specified (default spec-driven), when the TUI renders, then either "spec-driven" is shown or the field is omitted

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] ProgressInfo extended with WorkflowName field
- [x] Workflow name passed from root.go to TUI
- [x] Workflow name displayed in session info or header
- [x] All tests passing (`make check`)

**Dependencies**:
- Uses existing session info panel or header panel
- Integrates with workflow configuration in root.go

**Risks**:
- None significant (simple display addition)

**Notes**: The workflow name provides important context about how the iteration loop behaves. Users can verify their config is correct without checking CLI flags or config files. Consider also showing this in the exit summary.

**Effort Estimate**: XS (1 hour)

---

## Epic: Workflow Improvements

### [x] **Ticket: Refactor template variables for workflow prompts**

**As a** user
**I want** distinct template variables for spec file, context files, and notes file
**So that** prompts can clearly distinguish between the task source and reference material

**Context**: The current `{{files}}` placeholder bundles all file arguments together without distinguishing their purpose. Claude cannot tell which file contains the tasks to implement vs which files are additional context or reference material. This ambiguity leads to confusion in prompt interpretation.

**Description**: Add new template variables that separate the primary spec file from additional context files. Keep `{{files}}` for backwards compatibility.

**Implementation Requirements**:

1. **Add new template variables** in `internal/workflow/executor.go`:
   ```go
   // New placeholders:
   {{spec_file}}      // Primary spec/stories file (first file argument)
   {{context_files}}  // Additional reference files (remaining file arguments)
   {{notes_file}}     // Notes file path

   // Keep for backwards compatibility:
   {{files}}          // All files (existing behaviour)
   {{plural}}         // "s" if multiple files
   ```

2. **Update Runner struct** to track spec vs context:
   ```go
   type Runner struct {
       // ...
       specFile     string
       contextFiles []string
       notesFile    string
   }

   func (r *Runner) SetSpecFile(path string) { r.specFile = path }
   func (r *Runner) SetContextFiles(paths []string) { r.contextFiles = paths }
   func (r *Runner) SetNotesFile(path string) { r.notesFile = path }
   ```

3. **Update buildPrompt** to handle new placeholders:
   ```go
   result = strings.ReplaceAll(result, "{{spec_file}}", r.specFile)
   if len(r.contextFiles) > 0 {
       result = strings.ReplaceAll(result, "{{context_files}}", formatFileList(r.contextFiles))
   } else {
       result = strings.ReplaceAll(result, "{{context_files}}", "(none provided)")
   }
   result = strings.ReplaceAll(result, "{{notes_file}}", r.notesFile)
   ```

4. **Update cmd/orbital/root.go** to pass files with correct semantics:
   - First file argument → spec file
   - Remaining file arguments → context files
   - Notes file from config/flag

**Acceptance Criteria**:
- [x] Given `{{spec_file}}` in a prompt, when the prompt is built, then only the first file path is substituted
- [x] Given `{{context_files}}` in a prompt with multiple files, when the prompt is built, then files 2..N are listed
- [x] Given `{{context_files}}` in a prompt with one file, when the prompt is built, then "(none provided)" is shown
- [x] Given `{{notes_file}}` in a prompt, when the prompt is built, then the notes file path is substituted
- [x] Given `{{files}}` in a prompt, when the prompt is built, then all files are listed (backwards compatible)

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] New template variables implemented in executor.go
- [x] Runner tracks spec file, context files, and notes file separately
- [x] Existing `{{files}}` placeholder still works
- [x] Unit tests for new template substitution
- [x] All tests passing (`make check`)

**Dependencies**:
- Modifies `internal/workflow/executor.go`
- Modifies `cmd/orbital/root.go`

**Risks**:
- Breaking existing custom prompts using `{{files}}` (mitigated: keep `{{files}}` working)

**Notes**: This is a foundational change that enables the prompt tightening stories. The separation of spec vs context files allows prompts to give clear instructions about which file to read for tasks vs which files are reference material.

**Effort Estimate**: S (2-3 hours)

---

### [ ] **Ticket: Tighten autonomous 'implement' step for single-task discipline**

**As a** user
**I want** the autonomous 'implement' step to work on exactly one task per iteration
**So that** changes are atomic and context rot is minimised

**Context**: The current 'implement' step prompt says "pick the highest-leverage task" but does not explicitly forbid working on multiple items. Claude sometimes completes several tasks in one iteration, violating the "one task per iteration" principle from the system prompt. This leads to larger, harder-to-review changes and increased risk of context rot.

**Description**: Rewrite the autonomous 'implement' step prompt to explicitly enforce single-task discipline with clear constraints.

**Implementation Requirements**:

Update the 'implement' step in `internal/workflow/presets.go`:

```go
{
    Name: "implement",
    Prompt: `Study the spec file to understand the remaining work:
{{spec_file}}

Context files (reference only, do not modify):
{{context_files}}

Notes file for cross-iteration context:
{{notes_file}}

TASK SELECTION:
Pick exactly ONE task from the spec file. Choose the highest-leverage task:
the one that unblocks the most work or provides the most value.

CONSTRAINTS:
- Complete ONE task only. Do not start additional tasks.
- Do not work on multiple items even if they seem related.
- If you finish early, exit. Do not fill time with extra work.
- Small, focused changes are better than large, sweeping ones.

EXECUTION:
1. Identify the single highest-leverage unchecked task
2. Implement that task fully
3. Verify the outcome (tests, lint, typecheck)
4. Document in notes: which task, why chosen, key decisions
5. Check off the completed item in the spec file
6. Commit all changes with a descriptive message
7. Exit (do not output completion promise)`,
},
```

**Acceptance Criteria**:
- [ ] Given the autonomous 'implement' step runs, when Claude selects work, then exactly one task is chosen
- [ ] Given a task is completed quickly, when Claude finishes, then it exits without starting additional work
- [ ] Given related tasks exist, when Claude implements one, then it does not "helpfully" do the others
- [ ] Given the prompt references `{{spec_file}}`, when built, then the correct path is substituted

**Definition of Done** (Single Commit):
- [ ] Feature complete in one atomic commit
- [ ] Autonomous 'implement' prompt updated in presets.go
- [ ] Prompt uses new `{{spec_file}}`, `{{context_files}}`, `{{notes_file}}` variables
- [ ] Clear CONSTRAINTS section enforcing single-task discipline
- [ ] All tests passing (`make check`)

**Dependencies**:
- Depends on: Refactor template variables (for `{{spec_file}}`, `{{context_files}}`, `{{notes_file}}`)

**Risks**:
- Claude may still try to work on multiple tasks (mitigated: explicit constraints + uppercase emphasis)
- Tasks may be too small individually (mitigated: "highest-leverage" selection criteria)

**Notes**: The "one task per iteration" principle is fundamental to the autonomous loop methodology. Working on multiple tasks increases context rot risk, makes debugging harder, and produces larger diffs that are harder to review. The explicit CONSTRAINTS section uses directive language to reinforce the system prompt guidance.

**Effort Estimate**: XS (1 hour)

---

### [ ] **Ticket: Tighten autonomous 'fix' step to only address review feedback**

**As a** user
**I want** the autonomous 'fix' step to only fix issues from the review
**So that** it does not pick up new tasks and conflate "fixing" with "implementing"

**Context**: When the review gate fails, the 'fix' step should ONLY address the review feedback. Currently, Claude sometimes reads the spec file and picks up new work, conflating "fixing review issues" with "continuing implementation". This scope creep defeats the purpose of the review gate and produces changes that weren't reviewed.

**Description**: Rewrite the autonomous 'fix' step prompt to explicitly forbid reading the spec file for new tasks and constrain it to only fixing the issues identified in the review.

**Implementation Requirements**:

Update the 'fix' step in `internal/workflow/presets.go`:

```go
{
    Name:     "fix",
    Deferred: true,
    Prompt: `Read the notes file for review feedback from the previous iteration:
{{notes_file}}

YOUR ONLY JOB: Fix the issues identified by the reviewers.

CONSTRAINTS:
- Do NOT read the spec file for new tasks
- Do NOT pick up additional work beyond what reviewers flagged
- Do NOT implement new features or enhancements
- Do NOT refactor code beyond what is needed to fix the issues
- ONLY address the specific issues listed in the review feedback

EXECUTION:
1. Read the review feedback section in the notes file
2. For each issue raised by reviewers, implement a targeted fix
3. Verify fixes are correct (tests, lint, typecheck)
4. Update notes with what was fixed and why
5. Commit with message describing the fixes (reference the review issues)
6. Exit`,
},
```

**Acceptance Criteria**:
- [ ] Given the 'fix' step runs after review failure, when Claude executes, then only review issues are addressed
- [ ] Given the spec file has unchecked tasks, when 'fix' runs, then those tasks are NOT started
- [ ] Given reviewers flagged 3 issues, when 'fix' completes, then exactly those 3 issues are addressed
- [ ] Given a fix requires minor refactoring, when Claude implements it, then refactoring is limited to what's needed

**Definition of Done** (Single Commit):
- [ ] Feature complete in one atomic commit
- [ ] Autonomous 'fix' prompt updated in presets.go
- [ ] Prompt explicitly forbids reading spec file for new tasks
- [ ] Clear CONSTRAINTS section preventing scope creep
- [ ] All tests passing (`make check`)

**Dependencies**:
- Depends on: Refactor template variables (for `{{notes_file}}`)

**Risks**:
- Review feedback may be unclear (mitigated: review prompt should write specific, actionable feedback)
- Fixes may require touching code that wasn't flagged (acceptable: minimal scope expansion for correctness)

**Notes**: The fix step scope creep is particularly problematic because it conflates two different modes of operation: "fix what's broken" vs "continue building". By explicitly forbidding spec file reading for new tasks, we ensure the fix step stays focused on addressing reviewer concerns. This maintains the integrity of the review gate.

**Effort Estimate**: XS (1 hour)

---

## Backlog Prioritisation

**Must Have (Sprint 1):**
1. Reduce default iteration timeout to 5 minutes (XS - quick win) ✓
2. Fix gate-based workflow completion promise detection (S - bug fix) ✓
3. Fix cost display resetting to zero when step/iteration starts (XS - bug fix) ✓
4. Auto-create notes file when CLI starts (XS) ✓
5. Add periodic file content refresh for spec and notes tabs (S) ✓
6. Add context window progress bar showing token usage (S)
7. Print exit summary when TUI shuts down (S)
8. Add iteration countdown timer to TUI (S)
9. Add light and dark mode with auto-detection (M)
10. Display active workflow name in TUI (XS)
11. Refactor template variables for workflow prompts (S)
12. Tighten autonomous 'implement' step for single-task discipline (XS)
13. Tighten autonomous 'fix' step to only address review feedback (XS)

**Should Have (Future):**
- Configurable refresh interval via flag
- Visual indicator when file content has changed
- fsnotify-based watching (more efficient than polling)
- Context window as CLI flag for custom model configurations

**Could Have (Future):**
- Live diff view showing what changed
- Notification when spec items are checked off
- Cache hit ratio display alongside context usage

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

### Context Window Progress Bar Architecture

**Model-to-Context Mapping:**
```go
// internal/config/config.go
var ModelContextWindows = map[string]int{
    "opus":   200000,
    "sonnet": 200000,
    "haiku":  200000,
}

func GetContextWindow(model string) int {
    if window, ok := ModelContextWindows[model]; ok {
        return window
    }
    return 200000 // Default for unknown models
}
```

**Extended ProgressInfo:**
```go
// internal/tui/model.go
type ProgressInfo struct {
    Iteration     int
    MaxIteration  int
    StepName      string
    StepPosition  int
    StepTotal     int
    GateRetries   int
    MaxRetries    int
    TokensIn      int
    TokensOut     int
    Cost          float64
    Budget        float64
    ContextWindow int  // NEW: Model's context window size
}
```

**Progress Panel Layout (view.go):**
```go
// Line 1: Iteration progress
// Line 2: Budget progress + token counts
// Line 3: Context window progress (NEW)

contextRatio := float64(m.progress.TokensIn+m.progress.TokensOut) / float64(m.progress.ContextWindow)
contextBar := RenderProgressBar(contextRatio, BarWidth, normalStyle, warningStyle)
totalTokens := m.progress.TokensIn + m.progress.TokensOut
contextLine := fmt.Sprintf("Context: %s %s/%s (%d%%)",
    contextBar,
    formatNumber(totalTokens),
    formatNumber(m.progress.ContextWindow),
    int(contextRatio*100))
```

**Token Update Flow:**
```
Claude CLI stream-json
    ↓
parser.go: Extract tokens from assistant/result events
    ↓
bridge.go: Send StatsMsg{TokensIn, TokensOut, Cost}
    ↓
model.go: Update m.progress.TokensIn, m.progress.TokensOut
    ↓
view.go: Render context bar with (TokensIn + TokensOut) / ContextWindow
```

**Reset Behaviour:**
The context bar shows per-invocation usage. When a new Claude CLI invocation starts:
- Parser resets intermediate token counters
- Bridge sends StatsMsg with initial token values
- TUI updates to reflect new invocation's usage

This differs from cost/budget which accumulates across the entire Orbital session.

### Exit Summary Architecture

**Exit Paths in root.go:**
```
TUI Exit Scenarios:
├── Success (completion promise verified)
│   └── tuiProgram.Quit() → Wait() → PrintSuccessSummary()
├── Error (budget/iterations/timeout)
│   └── tuiProgram.Quit() → Wait() → PrintErrorSummary()
└── Interrupt (CTRL-C)
    └── tuiProgram.Kill() → PrintInterruptSummary()
```

**Summary Data Structure:**
```go
type SessionSummary struct {
    Status       string        // "Completed", "Interrupted", "Failed"
    ExitReason   string        // Detailed reason (e.g., "Budget exceeded")
    Iterations   int           // Total iterations run
    Duration     time.Duration // Total session duration
    Cost         float64       // Total cost in USD
    TokensIn     int           // Total input tokens
    TokensOut    int           // Total output tokens
    SessionID    string        // For resume instructions
    WorkflowSteps []StepSummary // Per-step breakdown (if workflow)
}

type StepSummary struct {
    Name     string
    Status   string  // "✓" or "✗"
    Cost     float64
    Duration time.Duration
}
```

**Print Location in root.go:**
```go
// After TUI exits (around line 442-450)
if useTUI {
    tuiProgram.Quit()
    tuiProgram.Wait() // Ensure TUI fully exits

    // Now safe to print to stdout
    summary := buildSessionSummary(loopState, workflowResults, sessionID, err)
    printSessionSummary(formatter, summary)
}
```

**Summary Format:**
```
═══════════════════════════════════════════════════════════
Session Complete
═══════════════════════════════════════════════════════════
Status:     ✓ Completed
Iterations: 5
Duration:   3m 42s
Cost:       $1.23
Tokens:     45,678 in / 12,345 out
═══════════════════════════════════════════════════════════
```

**Interrupt Format (with resume instructions):**
```
═══════════════════════════════════════════════════════════
Session Interrupted
═══════════════════════════════════════════════════════════
Status:     ⚠ Interrupted by user
Iterations: 3
Duration:   2m 15s
Cost:       $0.87
Tokens:     32,100 in / 8,450 out

Resume with:
  orbital --resume abc123def spec.md
═══════════════════════════════════════════════════════════
```

**Terminal State Reset:**
After TUI exits, terminal may be in alternate screen mode. Bubbletea handles this, but ensure:
- `tuiProgram.Wait()` completes before printing
- Use `fmt.Println()` not TUI rendering
- Flush stdout after summary

### Iteration Countdown Timer Architecture

**Extended ProgressInfo:**
```go
type ProgressInfo struct {
    // ... existing fields ...
    IterationTimeout time.Duration // Configured timeout (e.g., 5m)
    IterationStart   time.Time     // When current iteration began
}
```

**Timer Tick in TUI:**
```go
type timerTickMsg time.Time

func timerTick() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return timerTickMsg(t)
    })
}

// In Update()
case timerTickMsg:
    // Just trigger re-render, time calculation happens in view
    return m, timerTick()
```

**Time Calculation in View:**
```go
func (m Model) renderIterationTimer() string {
    if m.progress.IterationStart.IsZero() {
        return "—"
    }

    elapsed := time.Since(m.progress.IterationStart)
    remaining := m.progress.IterationTimeout - elapsed

    if remaining < 0 {
        remaining = 0
    }

    // Format as "Xm Ys remaining"
    mins := int(remaining.Minutes())
    secs := int(remaining.Seconds()) % 60

    style := normalStyle
    if remaining < time.Minute {
        style = warningStyle
    }

    return style.Render(fmt.Sprintf("%dm %02ds remaining", mins, secs))
}
```

**Display Integration (Format A - inline with iteration bar):**
```go
// Progress panel Line 1:
// Current: Iteration 2/50 [████░░░░░░]
// New:     Iteration 2/50 [████░░░░░░] 3m 42s remaining

func (m Model) renderIterationLine() string {
    iterBar := RenderProgressBar(iterRatio, BarWidth, normalStyle, warningStyle)
    timer := m.renderIterationTimer()

    return fmt.Sprintf("Iteration %d/%d %s %s",
        m.progress.Iteration,
        m.progress.MaxIteration,
        iterBar,
        timer)
}
```

Note: Timer only displays during implementor steps, not during gate steps. For gates, the timer portion is omitted or shows "—".

**Iteration Start Callback (root.go):**
```go
controller.SetIterationStartCallback(func(iteration int) {
    tuiProgram.SendProgress(tui.ProgressInfo{
        Iteration:        iteration,
        MaxIteration:     cfg.MaxIterations,
        IterationTimeout: cfg.IterationTimeout,
        IterationStart:   time.Now(),
        // ... preserve other fields ...
    })
})
```

**Timer Lifecycle:**
```
Iteration 1 starts → IterationStart = now, timer shows "5m 00s"
    ↓ (1 second tick)
Timer shows "4m 59s"
    ↓ (continues ticking)
Timer shows "0m 30s" (warning colour)
    ↓
Iteration 1 completes → Iteration 2 starts → IterationStart = now, timer resets
```

### Light/Dark Theme Architecture

**Theme Detection Flow:**
```
CLI Start
    ↓
Check --theme flag → if specified, use it
    ↓
Check config file theme → if specified, use it
    ↓
Auto-detect using termenv.HasDarkBackground()
    ↓
Apply resolved theme to Styles struct
    ↓
Pass Styles to Model and Selector
```

**New File: `internal/tui/themes.go`**
```go
package tui

import "github.com/muesli/termenv"

type Theme string

const (
    ThemeAuto  Theme = "auto"
    ThemeDark  Theme = "dark"
    ThemeLight Theme = "light"
)

func DetectTheme() Theme {
    output := termenv.NewOutput(os.Stdout)
    if output.HasDarkBackground() {
        return ThemeDark
    }
    return ThemeLight
}

func ResolveTheme(configured Theme) Theme {
    if configured == ThemeAuto {
        return DetectTheme()
    }
    return configured
}
```

**Style Factory Pattern:**
```go
// internal/tui/styles.go

func GetStyles(theme Theme) Styles {
    switch theme {
    case ThemeLight:
        return lightStyles()
    default:
        return darkStyles()
    }
}

func darkStyles() Styles {
    return Styles{
        Header: lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB000")),
        // ... current amber palette
    }
}

func lightStyles() Styles {
    return Styles{
        Header: lipgloss.NewStyle().Foreground(lipgloss.Color("#8B6914")),
        // ... dark amber palette for light backgrounds
    }
}
```

**Config Integration:**
```go
// internal/config/config.go
type Config struct {
    // ...
    Theme string `toml:"theme"` // "auto", "light", "dark"
}

func NewConfig() *Config {
    return &Config{
        Theme: "auto",
        // ...
    }
}
```

**CLI Flag:**
```go
// cmd/orbital/root.go
rootCmd.Flags().StringVar(&cfg.Theme, "theme", "auto",
    "Colour theme: auto, light, dark")
```

**Colour Palette Comparison:**

| Colour Role | Dark Theme (current) | Light Theme (new) |
|-------------|---------------------|-------------------|
| Primary | `#FFB000` | `#8B6914` |
| Primary Dim | `#996600` | `#5C4A0A` |
| Body Text | `#FFD966` | `#6B5A1E` |
| Faded | `#B38F00` | `#7A6A30` |
| Success | `#00FF00` | `#008000` |
| Warning | `#FFAA00` | `#CC5500` |
| Error | `#FF3300` | `#CC0000` |

## Success Metrics

| Metric | Target |
|--------|--------|
| Notes tab accessible on first click | 100% (no "file not found" errors) |
| File changes visible within | 3 seconds of modification |
| Unnecessary file reads | 0 (mtime gating) |
| Scroll position preserved on refresh | 100% |
| Context bar updates per streamed message | Every message with token data |
| Context bar accuracy | Matches final token counts from result event |
| Context bar reset on new session | Immediate reset when new invocation starts |
| Warning colour threshold | Triggered at 80% of context window |
| Exit summary printed on CTRL-C | 100% (always shows interrupt summary) |
| Exit summary printed on completion | 100% (always shows success summary) |
| Exit summary printed on error | 100% (budget, iterations, timeout) |
| Summary appears after TUI clears | No rendering artifacts or overlap |
| Resume instructions on interrupt | Session ID and command provided |
| Timer updates every second | Continuous 1-second tick during iteration |
| Timer accuracy | Within 1 second of actual remaining time |
| Timer resets on new iteration | Immediate reset when iteration starts |
| Warning colour threshold | Triggered at < 1 minute remaining |
| Timer shows neutral state between iterations | Displays "—" when no iteration active |
| Theme auto-detection accuracy | Correct detection on iTerm2, Terminal.app, Kitty, WezTerm |
| Light theme readability | All text meets WCAG AA contrast ratio (4.5:1) |
| Dark theme preserved | Existing amber palette unchanged |
| Theme flag override | --theme flag takes precedence over auto-detection |
| Config file theme | theme setting in config.toml respected |
| Fallback behaviour | Defaults to dark when detection fails |
| Workflow name visible | Displayed in TUI when workflow is active |
| Workflow name accuracy | Matches configured workflow preset |
