# Session Notes: TUI Real-Time Updates Implementation

Date: 2026-01-24

## Summary

All six tickets from the TUI Real-Time Updates epic have been implemented:

1. **Ticket 6 (Parser Stat Accumulation)**: Fixed the parser to properly track stats across assistant messages and result events without double-counting. Added separate tracking for assistant tokens (replaced per message) and result tokens (accumulated across iterations).

2. **Ticket 1 (Stats on Assistant Messages)**: Modified the bridge to send `StatsMsg` when assistant or result events contain usage stats, enabling real-time token and cost updates during streaming.

3. **Ticket 2 (Workflow Step Start Notification)**: Updated the workflow runner's start callback to send progress updates to the TUI when a step begins, showing the step name immediately.

4. **Ticket 3 (Iteration Start Notification)**: Updated the loop controller's iteration start callback to send progress updates to the TUI when an iteration begins.

5. **Ticket 4 (Non-Blocking Message Queue)**: Refactored the bridge to use a buffered channel (100 messages) with a background goroutine pump. Messages are dropped gracefully if the queue is full, preventing stream processing from blocking on slow TUI rendering.

6. **Ticket 5 (TUI Startup Synchronisation)**: Added a 50ms delay after starting the TUI goroutine to ensure the event loop is ready before the main loop begins sending messages.

## Key Implementation Details

### Parser Changes (`internal/output/parser.go`)

- Added `assistantTokensIn/Out` for tracking intermediate values within an iteration
- Added `resultTokensIn/Out` for accumulating values across iterations
- Assistant messages update assistant counters, which are combined with result counters for the display value
- Result messages reset assistant counters and add to result counters
- Cost and duration always accumulate (used for budget tracking)

### Bridge Changes (`internal/tui/bridge.go`)

- Added `msgQueue` channel with 100 message buffer
- Added `messagePump()` goroutine that forwards messages to the TUI program
- Added `sendMsg()` helper with non-blocking send (drops messages if queue full)
- Added `Close()` method for proper cleanup
- All `program.Send()` calls replaced with `sendMsg()`

### Root Command Changes (`cmd/orbital/root.go`)

- Iteration start callback now sends `ProgressInfo` to TUI
- Workflow step start callback now sends `ProgressInfo` to TUI (includes step name, position, total)
- Added 50ms delay after starting TUI goroutine for startup synchronisation

## Tests Added

- `TestParseLine_AssistantThenResult_NoDoubleCount`: Verifies no double-counting
- `TestParseLine_MultipleIterations_AccumulatesCorrectly`: Verifies correct accumulation across iterations
- `TestBridgeStatsMsg`: Verifies stats are sent on assistant/result events
- `TestBridgeStatsMsgProgressive`: Verifies progressive stat updates
- `TestBridgeMessageQueue`: Tests queue behaviour (send, full queue, close, sends after close)

## Verification

All tests pass:
- `go test ./...` passes
- `go vet ./...` passes
- `go build ./...` succeeds

## Notes for Future

- The 50ms startup delay is pragmatic but not ideal. A better approach would be implementing a ready signal via the TUI's Init() function.
- The message queue drops messages when full. Consider adding a metric for dropped messages if users report missing updates.
- The queue size (100) was chosen as a reasonable default. May need adjustment based on real-world usage patterns.

## Code Review: 2026-01-24

**Reviewer:** Gate Review Agent

### Summary

Changes implement the TUI Real-Time Updates epic. Six tickets covering real-time stats updates, workflow step notifications, iteration notifications, non-blocking message queue, TUI startup synchronisation, and parser stat accumulation fixes.

### Correctness

**Parser (`internal/output/parser.go`)**
- Token tracking logic is sound: assistant tokens are replaced per message, result tokens accumulate across iterations
- The combination logic (`stats.TokensIn = resultTokensIn + assistantTokensIn`) correctly shows intermediate progress during streaming
- When a result arrives, assistant counters reset and result counters accumulate, preventing double-counting
- Cost and duration accumulation is correct for budget tracking

**Bridge (`internal/tui/bridge.go`)**
- Non-blocking message queue implemented correctly with `select` and default clause
- Proper use of `atomic.Bool` for closed state avoids race conditions
- `Close()` is idempotent (uses `Swap` to check if already closed)
- Message pump goroutine exits cleanly when channel is closed

**Root command (`cmd/orbital/root.go`)**
- Iteration and step start callbacks correctly send progress updates to TUI
- 50ms startup delay is documented and justified

### Edge Cases

1. **Queue full handling**: Messages are dropped silently. Acceptable for real-time updates (tests verify this behaviour).
2. **Nil program**: When `program` is nil, no pump goroutine starts. Tests cover this.
3. **Close after close**: Idempotent, no panic. Tests verify.
4. **Send after close**: Ignored gracefully via atomic check. Tests verify.

### Code Quality

- Clear comments explaining the stat accumulation strategy
- Good separation of concerns between parser (data) and bridge (transport)
- Tests are comprehensive with clear names and descriptions

### Test Coverage

New tests added:
- `TestParseLine_AssistantThenResult_NoDoubleCount` - validates no double-counting
- `TestParseLine_MultipleIterations_AccumulatesCorrectly` - validates cross-iteration accumulation
- `TestBridgeMessageQueue` - tests queue behaviour under various conditions
- `TestBridgeStatsMsg` - tests stats sending on different event types
- `TestBridgeStatsMsgProgressive` - tests progressive stat updates

All tests pass.

### Minor Observations (Non-Blocking)

1. **Bridge.Close() not called by Program**: The `tui.Program` struct creates a Bridge but has no cleanup method to call `bridge.Close()`. When the TUI exits, the message pump goroutine may remain alive (blocked on channel read). Since program exit follows shortly, this is not a practical issue, but adding a `Program.Close()` method that calls `bridge.Close()` would be cleaner for explicit resource management.

2. **No metrics for dropped messages**: As noted in the implementation, there's no visibility into whether messages are being dropped. A debug counter could help diagnose issues if users report missing updates.

### Verdict

The implementation is correct, well-tested, and addresses all acceptance criteria. The minor observations above are not blockers and can be addressed in future work if desired.
