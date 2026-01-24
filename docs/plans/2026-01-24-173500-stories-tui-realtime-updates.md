# User Stories: TUI Real-Time Updates

## Project Overview

The Orbital CLI TUI (Terminal User Interface) currently fails to update in real-time during Claude CLI execution. Token counts, cost tracking, and workflow step indicators remain static until an iteration completes, providing users with no feedback during potentially long-running operations. This epic addresses the identified issues to deliver a responsive, continuously updating TUI that reflects execution progress as it happens.

**Target Users:** Developers using Orbital CLI to run autonomous Claude Code iterations who need real-time visibility into execution progress, resource consumption, and workflow state.

**Business Value:** Real-time feedback reduces user anxiety during long operations, enables early detection of runaway costs, and provides confidence that the system is working correctly.

## Story Mapping Overview

**Epic: TUI Real-Time Updates**

Priority order based on user impact:

1. **Critical:** Real-time token and cost updates during streaming
2. **High:** Workflow step indicator updates at step start
3. **High:** Iteration counter updates at iteration start
4. **Medium:** Non-blocking message delivery
5. **Low:** TUI startup synchronisation

---

## Epic: TUI Real-Time Updates

### [x] **Ticket 1: Send Stats Updates on Assistant Messages**

**As a** developer monitoring an Orbital execution
**I want** to see token counts and cost update in real-time as Claude streams responses
**So that** I can monitor resource consumption during execution rather than waiting for completion

**Context:** Currently, `StatsMsg` is only sent when parsing the final "result" event. The parser extracts usage stats from assistant messages but the bridge discards them, leaving the TUI displaying stale values throughout execution.

**Description:** Modify the bridge to send `StatsMsg` when assistant messages contain usage information, not just on result events. This enables the TUI to display incrementing token counts and cost as Claude generates output.

**Implementation Requirements:**
- Modify `processLine()` in `internal/tui/bridge.go` to send `StatsMsg` on assistant events
- Only send stats when values are non-zero to avoid unnecessary updates
- Preserve existing result event handling for final stats
- Ensure stats from assistant messages reflect cumulative totals

**Acceptance Criteria:**
- [x] Given Claude is streaming a response, when an assistant message with usage stats is received, then the TUI token counters update immediately
- [x] Given Claude is streaming, when multiple assistant messages arrive, then token counts increment progressively
- [x] Given the TUI is displaying updated stats, when the result event arrives, then final stats are displayed correctly
- [x] Given an assistant message has no usage stats, when processed by the bridge, then no StatsMsg is sent

**Definition of Done** (Single Commit):
- [x] `internal/tui/bridge.go` modified to send StatsMsg on assistant events
- [x] Unit tests added for new StatsMsg triggering logic
- [x] Existing bridge tests pass
- [x] Manual testing confirms real-time token updates in TUI

**Dependencies:**
- None (foundational change)

**Risks:**
- High-frequency stats updates could cause TUI flicker; mitigate by rate-limiting if needed

**Notes:** The parser already extracts stats from assistant messages via `parseAssistantMessage()`. This change enables the bridge to use that data.

**Effort Estimate:** S

---

### [x] **Ticket 2: Add TUI Notification to Workflow Step Start Callback**

**As a** developer running a multi-step workflow
**I want** to see the current step name update when a step begins execution
**So that** I know which workflow step is currently running without waiting for it to complete

**Context:** The workflow runner has a `SetStartCallback` that fires when each step begins, but it only prints to the formatter in non-TUI mode. TUI users see the step name update only after the step completes, providing no visibility into current execution state.

**Description:** Modify the workflow step start callback in `root.go` to send a `ProgressInfo` update to the TUI when a step begins, including the step name, position, and total step count.

**Implementation Requirements:**
- Modify `runner.SetStartCallback()` in `cmd/orbital/root.go` to call `tuiProgram.SendProgress()` when TUI is active
- Include step name, position, and total in the progress update
- Preserve existing non-TUI formatter output

**Acceptance Criteria:**
- [x] Given a multi-step workflow is running with TUI enabled, when a new step begins, then the TUI immediately displays the step name
- [x] Given the TUI is showing step progress, when step position changes, then the step counter (e.g., "2/4") updates
- [x] Given TUI is disabled (minimal mode), when a step begins, then the formatter prints the step start as before

**Definition of Done** (Single Commit):
- [x] `cmd/orbital/root.go` start callback modified to notify TUI
- [x] Manual testing confirms step name appears at step start in TUI
- [x] Non-TUI mode behaviour unchanged (verified manually)

**Dependencies:**
- None

**Risks:**
- Progress updates must preserve other fields (tokens, cost) to avoid resetting displayed values

**Notes:** The `ProgressInfo` struct already has `StepName`, `StepPosition`, and `StepTotal` fields. The callback needs to populate these and send them.

**Effort Estimate:** XS

---

### [x] **Ticket 3: Add TUI Notification to Iteration Start Callback**

**As a** developer monitoring a long-running Orbital session
**I want** to see the iteration counter update when a new iteration begins
**So that** I know when the loop has moved to the next iteration without waiting for completion

**Context:** The loop controller has an `IterationStartCallback` but it only updates `iterationStartTime` and prints to the formatter. TUI users see the iteration counter update only after `iterationCallback` fires, which happens after the entire iteration completes.

**Description:** Modify the iteration start callback to send a `ProgressInfo` update to the TUI when a new iteration begins, updating the iteration counter immediately.

**Implementation Requirements:**
- Modify `controller.SetIterationStartCallback()` in `cmd/orbital/root.go` to call `tuiProgram.SendProgress()` when TUI is active
- Include iteration number and max iterations in the progress update
- Preserve other progress fields (tokens, cost) from previous state

**Acceptance Criteria:**
- [x] Given an Orbital loop is running with TUI enabled, when a new iteration starts, then the iteration counter updates immediately
- [x] Given the TUI is showing iteration 3/50, when iteration 4 starts, then the display shows 4/50 before any output streams
- [x] Given TUI is disabled, when an iteration starts, then the formatter prints as before

**Definition of Done** (Single Commit):
- [x] `cmd/orbital/root.go` iteration start callback modified to notify TUI
- [x] Manual testing confirms iteration counter updates at iteration start
- [x] Non-TUI mode behaviour unchanged

**Dependencies:**
- None

**Risks:**
- Must track previous progress state to avoid resetting token/cost values when sending iteration update

**Notes:** May need to track last known progress state to merge iteration update with existing token/cost values.

**Effort Estimate:** XS

---

### [x] **Ticket 4: Implement Non-Blocking Message Queue for Bridge**

**As a** developer using the TUI during high-output Claude sessions
**I want** stream processing to continue smoothly even if TUI rendering is slow
**So that** I do not experience stalled output or delayed responses

**Context:** The bridge calls `tea.Program.Send()` synchronously while holding a mutex. If TUI rendering is slow, stream processing stalls because `Send()` blocks until the message is processed. This creates a serialisation bottleneck during high-throughput streaming.

**Description:** Replace direct `Send()` calls in the bridge with a buffered channel that decouples stream processing from TUI message delivery. A separate goroutine pumps messages from the queue to the TUI.

**Implementation Requirements:**
- Add a buffered `msgQueue chan tea.Msg` to the `Bridge` struct
- Implement `startMessagePump()` goroutine that reads from queue and calls `Send()`
- Modify `processLine()` to send to queue instead of calling `Send()` directly
- Use non-blocking send with fallback (drop or overwrite) if queue is full
- Ensure proper cleanup when bridge is closed

**Acceptance Criteria:**
- [x] Given high-throughput Claude output, when the TUI is rendering, then stream processing continues without blocking
- [x] Given the message queue is full, when a new message arrives, then it is handled gracefully (dropped or oldest overwritten)
- [x] Given the TUI is shut down, when the bridge is closed, then the message pump goroutine exits cleanly
- [x] Performance: Stream processing latency does not exceed 10ms even during heavy TUI rendering

**Definition of Done** (Single Commit):
- [x] `internal/tui/bridge.go` refactored to use message queue
- [x] Message pump goroutine implemented with proper lifecycle management
- [x] Unit tests for queue behaviour (normal operation, full queue, shutdown)
- [x] Manual testing confirms smooth output during heavy streaming

**Dependencies:**
- Tickets 1-3 (benefit most from non-blocking sends)

**Risks:**
- Queue sizing: too small causes drops, too large uses memory; start with 100 messages
- Message ordering must be preserved

**Notes:** Consider adding a metric for dropped messages to detect if queue size is insufficient.

**Effort Estimate:** S

---

### [x] **Ticket 5: Add TUI Startup Synchronisation**

**As a** developer starting an Orbital session
**I want** early progress messages to be reliably delivered to the TUI
**So that** I see initial state correctly without race condition losses

**Context:** The TUI runs in a background goroutine via `tuiProgram.Run()`. The loop starts immediately after, potentially sending messages before the TUI event loop is ready. Messages sent before `Run()` starts may be lost.

**Description:** Add synchronisation to ensure the TUI is ready to receive messages before the main loop begins execution. This prevents potential message loss during startup.

**Implementation Requirements:**
- Add a ready signal mechanism before starting the loop
- Use a brief delay or ready channel to ensure TUI event loop is running
- Ensure synchronisation does not add noticeable latency to startup

**Acceptance Criteria:**
- [x] Given the TUI is starting, when the first progress message is sent, then it is reliably displayed
- [x] Given rapid startup, when messages are sent immediately after TUI launch, then no messages are lost
- [x] Startup latency: Added synchronisation does not delay loop start by more than 100ms

**Definition of Done** (Single Commit):
- [x] `cmd/orbital/root.go` modified with TUI ready synchronisation
- [x] Manual testing confirms initial messages appear reliably
- [x] Startup time impact measured and acceptable

**Dependencies:**
- None (but benefits all TUI message sends)

**Risks:**
- Bubbletea does not expose a native ready signal; workaround using brief delay or Init() signal

**Notes:** A 50ms delay is a pragmatic workaround. Alternatively, the TUI could send a ready message on Init() that the main goroutine waits for.

**Effort Estimate:** XS

---

### [x] **Ticket 6: Review and Fix Parser Stat Accumulation Logic**

**As a** developer relying on accurate cost and token tracking
**I want** the parser to correctly track cumulative statistics
**So that** displayed values are accurate and not double-counted

**Context:** The parser uses `+=` for result stats but `=` for assistant stats, creating inconsistency. The result event contains total values, not increments, so accumulation could cause double-counting if the parser processes multiple result events or if assistant and result stats overlap.

**Description:** Review and correct the parser's stat tracking to ensure consistent and accurate accumulation. Assistant message stats should update the current values, and result stats should either replace or correctly accumulate without double-counting.

**Implementation Requirements:**
- Audit `parseAssistantMessage()` and `parseResultStats()` for consistent stat handling
- Determine whether result stats are totals or increments (from Claude CLI documentation)
- Fix stat tracking to avoid double-counting
- Add comments documenting the expected behaviour

**Acceptance Criteria:**
- [x] Given multiple assistant messages with usage stats, when parsed, then token counts accumulate correctly
- [x] Given a result event after assistant messages, when parsed, then final stats are accurate (not double-counted)
- [x] Given the parser is reused across iterations, when stats are retrieved, then values reflect correct cumulative totals

**Definition of Done** (Single Commit):
- [x] `internal/output/parser.go` stat handling reviewed and corrected
- [x] Comments added explaining stat accumulation strategy
- [x] Unit tests added for multi-message stat accumulation scenarios
- [x] Existing parser tests pass

**Dependencies:**
- Should be done before or alongside Ticket 1 to ensure correct stats are sent

**Risks:**
- Behaviour depends on Claude CLI output format; may need testing with real output

**Notes:** The Claude CLI stream-json format documentation should clarify whether result stats are totals or increments.

**Effort Estimate:** S

---

## Backlog Prioritisation

**Must Have (Sprint 1):**
- [x] Ticket 1: Send Stats Updates on Assistant Messages (Critical user value)
- [x] Ticket 6: Review and Fix Parser Stat Accumulation Logic (Prerequisite for accurate stats)
- [x] Ticket 2: Add TUI Notification to Workflow Step Start Callback (High visibility)
- [x] Ticket 3: Add TUI Notification to Iteration Start Callback (High visibility)

**Should Have (Sprint 2):**
- [x] Ticket 4: Implement Non-Blocking Message Queue for Bridge (Performance improvement)

**Could Have (Sprint 3):**
- [x] Ticket 5: Add TUI Startup Synchronisation (Edge case fix)

**Won't Have:**
- Complete TUI rewrite or alternative UI frameworks
- Real-time cost estimation (requires pricing API)
- Historical progress graphs

## Technical Considerations

**Architecture:**
- All changes are localised to `internal/tui/` and `cmd/orbital/root.go`
- No changes to core loop logic or workflow engine
- Preserves existing non-TUI behaviour

**Testing Strategy:**
- Unit tests for bridge message sending logic
- Unit tests for parser stat accumulation
- Manual integration testing with real Claude CLI output
- Visual verification of TUI updates during execution

**Backwards Compatibility:**
- All changes are additive; no breaking changes to existing behaviour
- Non-TUI mode remains unaffected

## Success Metrics

**User Experience:**
- Token counts visibly increment during Claude streaming
- Cost updates at least once per second during active streaming
- Step name appears within 100ms of step start
- Iteration counter updates immediately when new iteration begins

**Technical:**
- No dropped messages under normal operation
- Stream processing latency under 10ms
- No increase in memory usage beyond message queue buffer

**Validation:**
- Manual testing with verbose spec files
- User feedback from early adopters
- No regression in existing TUI functionality
