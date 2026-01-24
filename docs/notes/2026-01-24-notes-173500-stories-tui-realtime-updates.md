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
