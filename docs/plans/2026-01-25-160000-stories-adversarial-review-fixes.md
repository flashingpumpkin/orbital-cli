# User Stories: Adversarial Review Critical Fixes

## Project Overview

This plan addresses critical and high-severity issues identified in the adversarial security review of Orbital CLI. The issues span security vulnerabilities, reliability gaps, and performance problems that could cause system compromise, data loss, or resource exhaustion during extended autonomous execution sessions.

**Source**: `docs/code-reviews/2026-01-25-adversarial-review-main.md`

**Priority**: These stories address issues that must be fixed before production use.

## Story Mapping Overview

### Epic 1: Security Hardening
- SEC-1: Make `--dangerously-skip-permissions` configurable

### Epic 2: Reliability Improvements
- REL-1: Propagate errors from Queue.Pop()
- REL-2: Add timeouts to git cleanup commands

### Epic 3: Performance Optimisations
- PERF-1: Implement ring buffer for TUI output
- PERF-2: Cache wrapped lines in TUI
- PERF-3: Use strings.Builder for parser concatenation
- PERF-4: Eliminate double parsing in executor
- PERF-5: Add streaming-only mode for executor
- PERF-6: Increase scanner buffer limit

### Epic 4: Design Improvements
- DESIGN-1: Add parser format validation and warnings

---

## Epic 1: Security Hardening

### [x] **Ticket: SEC-1 - Make permission skip flag configurable**

**As a** user running Orbital CLI
**I want** control over whether Claude can skip permission prompts
**So that** I can choose security over convenience based on my context

**Context**: The executor currently hardcodes `--dangerously-skip-permissions` in all Claude CLI invocations. This allows anyone controlling a spec file to instruct Claude to execute arbitrary shell commands without confirmation, enabling full system compromise.

**Description**: Add a configuration option to control whether the `--dangerously-skip-permissions` flag is passed to Claude CLI. The default should be safe (flag disabled), requiring explicit opt-in for dangerous mode.

**Implementation Requirements**:
- Add `DangerouslySkipPermissions bool` field to `config.Config` struct
- Add `--dangerous` CLI flag to root command, defaulting to `false`
- Modify `executor.BuildArgs()` to conditionally include the flag
- Update TOML config support to allow `dangerous = true` in config file
- Add warning message when dangerous mode is enabled

**Acceptance Criteria**:
- [x] Given default config, when executor builds args, then `--dangerously-skip-permissions` is NOT included
- [x] Given `--dangerous` CLI flag, when executor builds args, then `--dangerously-skip-permissions` IS included
- [x] Given `dangerous = true` in TOML config, when executor builds args, then flag IS included
- [x] Given dangerous mode enabled, when session starts, then warning is printed to stderr
- [x] Existing tests updated to account for new default behaviour

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] `config.Config` updated with new field
- [x] `cmd/orbital/root.go` updated with new flag
- [x] `internal/executor/executor.go` conditionally includes flag
- [x] `internal/config/file.go` parses TOML option
- [x] Unit tests for executor.BuildArgs() updated
- [x] Unit tests for config parsing updated

**Dependencies**:
- None

**Risks**:
- Breaking change for existing users who rely on current behaviour
- Mitigation: Document the change prominently in release notes

**Notes**: This is the highest priority security fix. Users who need the old behaviour can opt in explicitly.

**Effort Estimate**: S (2-3 hours)

---

## Epic 2: Reliability Improvements

### [x] **Ticket: REL-1 - Propagate errors from Queue.Pop()**

**As a** developer debugging queue issues
**I want** Queue.Pop() to return errors when save fails
**So that** I can detect and handle persistence failures instead of experiencing silent data loss

**Context**: The current `Queue.Pop()` implementation clears the in-memory queue and ignores save errors. If the save fails (disk full, permissions), the queue file still contains old data. On reload, files reappear causing duplicate processing.

**Description**: Change `Queue.Pop()` to return an error and propagate save failures to callers.

**Implementation Requirements**:
- Change `Pop()` signature from `func (q *Queue) Pop() []string` to `func (q *Queue) Pop() ([]string, error)`
- Return error from `q.save()` instead of ignoring it
- Update all callers to handle the error
- Use file locking consistently with Add/Remove operations

**Acceptance Criteria**:
- [x] Given save succeeds, when Pop() called, then files returned and no error
- [x] Given save fails (e.g., read-only filesystem), when Pop() called, then error returned
- [x] Given Pop() returns error, when caller handles it, then queue state is recoverable
- [x] All existing callers updated to handle error return

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] `internal/state/queue.go` updated with new signature
- [x] All callers in codebase updated
- [x] Unit tests for Pop() error cases added
- [x] Integration test for save failure scenario

**Dependencies**:
- None

**Risks**:
- Callers may need additional error handling logic
- Mitigation: Error should be logged but not necessarily fatal

**Notes**: Consider wrapping in `withLock()` for consistency with Add/Remove.

**Effort Estimate**: XS (1-2 hours)

---

### [x] **Ticket: REL-2 - Add timeouts to git cleanup commands**

**As a** user running worktree cleanup
**I want** git commands to timeout if they hang
**So that** my process does not hang indefinitely requiring manual intervention

**Context**: The `Cleanup.Run()` method in `worktree/merge.go` uses raw `exec.Command` without timeout. If git hangs (lock contention, network filesystem), the orbital process hangs indefinitely.

**Description**: Use `runGitCommand()` (which has a 30-second timeout) consistently for all git operations in the cleanup phase, or add context-based timeouts.

**Implementation Requirements**:
- Refactor `Cleanup.Run()` to accept a `context.Context` parameter
- Use `exec.CommandContext` with timeout for all git commands
- Apply consistent 30-second timeout matching `runGitCommand()` in `git.go`
- Return clear error message when timeout occurs

**Acceptance Criteria**:
- [x] Given git command completes normally, when cleanup runs, then success returned
- [x] Given git command hangs, when 30 seconds elapsed, then timeout error returned
- [x] Given timeout occurs, when error examined, then message indicates timeout
- [x] Context cancellation properly terminates git processes

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] `Cleanup.Run()` accepts context parameter
- [x] All git commands use `exec.CommandContext`
- [x] 30-second timeout applied consistently
- [x] Unit tests with mock commands added
- [x] Callers updated to pass context

**Dependencies**:
- None

**Risks**:
- 30 seconds may be too short for large repos
- Mitigation: Could make timeout configurable in future

**Notes**: Consider extracting timeout constant to share with `git.go`.

**Effort Estimate**: S (2-3 hours)

---

## Epic 3: Performance Optimisations

### [x] **Ticket: PERF-1 - Implement ring buffer for TUI output**

**As a** user running long sessions
**I want** memory usage to remain bounded regardless of output volume
**So that** my session does not crash from out-of-memory errors

**Context**: The TUI's `outputLines` slice grows unbounded. After 1000 iterations with 1000 lines each, memory usage reaches 200MB+, eventually causing OOM. This is the most critical performance issue.

**Description**: Replace the unbounded `outputLines` slice with a ring buffer that maintains a fixed maximum number of lines. Oldest lines are evicted when the limit is reached.

**Implementation Requirements**:
- Create a `RingBuffer` type with configurable max size (default 10,000 lines)
- Replace `outputLines []string` with ring buffer in Model
- Update `AppendOutput()` to use ring buffer semantics
- Update `wrapAllOutputLines()` to iterate ring buffer correctly
- Ensure scroll position handling works with ring buffer

**Acceptance Criteria**:
- [x] Given buffer at capacity, when new line added, then oldest line evicted
- [x] Given 50,000 lines added, when memory checked, then buffer holds exactly 10,000 lines
- [x] Given scrolling up in buffer, when at eviction boundary, then scroll stops at oldest available line
- [x] Given window resize, when buffer rewrapped, then all visible lines correct
- [x] Memory usage remains constant regardless of session length

**Definition of Done** (Single Commit):
- [x] Feature complete in one atomic commit
- [x] Ring buffer type implemented with tests
- [x] `internal/tui/model.go` updated to use ring buffer
- [x] Scroll logic updated for ring buffer semantics
- [x] Unit tests for ring buffer edge cases
- [x] Integration test for long session simulation

**Dependencies**:
- None (but PERF-2 builds on this)

**Risks**:
- Scroll position calculation becomes more complex
- Mitigation: Thorough testing of scroll edge cases

**Notes**: Consider making max size configurable via flag/config.

**Effort Estimate**: S (3-4 hours)

---

### [ ] **Ticket: PERF-2 - Cache wrapped lines in TUI**

**As a** user scrolling through output
**I want** smooth scrolling without lag
**So that** I can navigate output efficiently

**Context**: Every scroll operation calls `wrapAllOutputLines()` which iterates the entire output buffer and re-wraps all lines. With 50,000 lines, each scroll causes visible lag and 100% CPU usage.

**Description**: Cache wrapped lines and only invalidate the cache when window width changes or new lines are added. Scroll operations should use cached wrapped lines.

**Implementation Requirements**:
- Add `wrappedLinesCache []string` and `cacheWidth int` fields to Model
- Invalidate cache when `WindowSizeMsg` changes width
- Append to cache (rather than full recalculate) when new lines added
- Update scroll functions to use cache instead of calling `wrapAllOutputLines()`

**Acceptance Criteria**:
- [ ] Given cache populated, when scrolling, then no line wrapping occurs
- [ ] Given window width unchanged, when new line added, then only new line wrapped and appended to cache
- [ ] Given window width changed, when rendering, then full cache invalidation and rewrap occurs
- [ ] Scrolling 1000 times on 10,000 lines completes in under 100ms total
- [ ] CPU usage during scrolling is minimal

**Definition of Done** (Single Commit):
- [ ] Feature complete in one atomic commit
- [ ] Cache fields added to Model
- [ ] Cache invalidation logic implemented
- [ ] Incremental cache update for new lines
- [ ] All scroll functions use cache
- [ ] Benchmark tests for scroll performance

**Dependencies**:
- PERF-1 (ring buffer) should be completed first for consistent behaviour

**Risks**:
- Cache coherence bugs could cause display issues
- Mitigation: Add cache validation in debug mode

**Notes**: The ring buffer from PERF-1 simplifies cache management since eviction is predictable.

**Effort Estimate**: S (2-3 hours)

---

### [ ] **Ticket: PERF-3 - Use strings.Builder for parser concatenation**

**As a** user processing large Claude responses
**I want** efficient text accumulation
**So that** CPU is not wasted on quadratic string operations

**Context**: The parser uses `event.Content += block.Text` which creates a new string allocation for each concatenation. With many small text blocks, this results in O(n^2) CPU and memory usage.

**Description**: Replace string concatenation with `strings.Builder` for accumulating text content in the parser.

**Implementation Requirements**:
- Use `strings.Builder` in `parseAssistantMessage()` to accumulate text blocks
- Only call `builder.String()` once at the end
- Apply same pattern to any other concatenation loops in parser

**Acceptance Criteria**:
- [ ] Given 1000 text blocks of 100 bytes each, when parsed, then completes in linear time
- [ ] Given same input, when memory profiled, then allocations are O(n) not O(n^2)
- [ ] Existing parser tests pass without modification
- [ ] Output text content is identical to previous implementation

**Definition of Done** (Single Commit):
- [ ] Feature complete in one atomic commit
- [ ] `parseAssistantMessage()` uses strings.Builder
- [ ] Any other concatenation loops updated
- [ ] Benchmark test comparing old vs new performance
- [ ] All existing parser tests pass

**Dependencies**:
- None

**Risks**:
- None significant

**Notes**: Simple fix with high impact on large responses.

**Effort Estimate**: XS (1 hour)

---

### [ ] **Ticket: PERF-4 - Eliminate double parsing in executor**

**As a** user running iterations
**I want** output to be parsed only once
**So that** CPU is not wasted on redundant JSON parsing

**Context**: The `extractStats()` function re-parses the entire output from scratch even though output was already parsed during streaming. It's called multiple times per execution (lines 177, 195, 208, 230, 248, 261), doubling or tripling JSON parsing overhead.

**Description**: Carry parsed stats through the execution path rather than re-parsing. The streaming parser already accumulates stats; pass those through to the result.

**Implementation Requirements**:
- Store parser instance or stats in executor during streaming
- Retrieve final stats from parser after streaming completes
- Remove or deprecate `extractStats()` function
- Ensure non-streaming path also avoids double parsing

**Acceptance Criteria**:
- [ ] Given streaming execution, when result created, then stats come from streaming parser
- [ ] Given non-streaming execution, when result created, then output parsed exactly once
- [ ] Given 10MB output, when executed, then no redundant parsing occurs
- [ ] Token and cost values in result are identical to previous implementation
- [ ] All existing executor tests pass

**Definition of Done** (Single Commit):
- [ ] Feature complete in one atomic commit
- [ ] Parser stats passed through execution path
- [ ] `extractStats()` removed or marked deprecated
- [ ] Streaming and non-streaming paths both optimised
- [ ] Unit tests verify single parse
- [ ] Benchmark comparing old vs new

**Dependencies**:
- None

**Risks**:
- Stats might differ slightly due to timing of when they're captured
- Mitigation: Verify with integration tests

**Notes**: Significant CPU savings on large outputs.

**Effort Estimate**: S (2-3 hours)

---

### [ ] **Ticket: PERF-5 - Add configurable output retention limit**

**As a** user processing large Claude outputs
**I want** to limit how much output is retained in memory
**So that** single large iterations do not cause out-of-memory crashes

**Context**: The executor accumulates the entire stdout in a `bytes.Buffer`. If Claude produces 50MB output, the buffer grows to 50MB, then `stdout.String()` creates another 50MB copy. Combined with TUI buffers, this can exhaust memory.

**Description**: Add a configurable maximum output size. When exceeded, truncate older output while preserving the most recent content (where completion promises are likely to appear).

**Implementation Requirements**:
- Add `MaxOutputSize int` to config (default 10MB)
- Implement truncation in executor's streaming loop
- Preserve most recent output when truncating (completion promise is at end)
- Log warning when truncation occurs
- Ensure completion detection still works after truncation

**Acceptance Criteria**:
- [ ] Given output under limit, when execution completes, then full output retained
- [ ] Given output over limit, when execution completes, then output truncated to limit
- [ ] Given truncation occurs, when checking for promise, then detection still works (promise at end)
- [ ] Given truncation occurs, when verbose mode on, then warning logged
- [ ] Memory usage bounded regardless of output size

**Definition of Done** (Single Commit):
- [ ] Feature complete in one atomic commit
- [ ] Config option added
- [ ] Truncation logic in executor
- [ ] Warning on truncation
- [ ] Tests for truncation behaviour
- [ ] Tests for promise detection after truncation

**Dependencies**:
- None

**Risks**:
- Truncation might lose important context
- Mitigation: Keep recent output where promises appear; log warning

**Notes**: Consider using ring buffer pattern similar to PERF-1.

**Effort Estimate**: S (2-3 hours)

---

### [ ] **Ticket: PERF-6 - Increase scanner buffer limit**

**As a** user processing Claude responses with large tool results
**I want** large JSON lines to be handled without data loss
**So that** file contents and other large tool outputs are not silently dropped

**Context**: The scanner has a 1MB line limit. Claude's stream-json output can have single lines > 1MB (e.g., large file read results). When exceeded, `bufio.Scanner` returns `bufio.ErrTooLong` and the line is lost, potentially missing the completion promise.

**Description**: Increase the scanner buffer limit and add handling for oversized lines.

**Implementation Requirements**:
- Increase buffer limit from 1MB to 10MB
- Add detection for `bufio.ErrTooLong` with clear error message
- Consider chunked reading for extremely large lines (future enhancement)
- Log warning when lines approach limit

**Acceptance Criteria**:
- [ ] Given JSON line of 5MB, when scanned, then line is processed successfully
- [ ] Given JSON line of 10MB, when scanned, then line is processed successfully
- [ ] Given JSON line > 10MB, when scanned, then clear error message returned
- [ ] Given line near limit (>8MB), when scanned, then warning logged
- [ ] Existing functionality unchanged for normal-sized lines

**Definition of Done** (Single Commit):
- [ ] Feature complete in one atomic commit
- [ ] Buffer limit increased in executor
- [ ] Error handling for `ErrTooLong`
- [ ] Warning for lines approaching limit
- [ ] Test with large JSON lines

**Dependencies**:
- None

**Risks**:
- Larger buffer uses more memory per execution
- Mitigation: 10MB is reasonable for modern systems; buffer is reused

**Notes**: 10MB should handle most realistic scenarios while preventing unbounded allocation.

**Effort Estimate**: XS (1 hour)

---

## Epic 4: Design Improvements

### [ ] **Ticket: DESIGN-1 - Add parser format validation and warnings**

**As a** user running Orbital after Claude CLI updates
**I want** clear feedback when output format changes
**So that** I can diagnose issues instead of experiencing silent failures

**Context**: The parser depends on Claude CLI's undocumented stream-json format. When parsing fails, it returns nil without error. If Claude CLI's format changes, Orbital breaks silently with no stats, no completion detection, and no clear error.

**Description**: Add format validation that detects unrecognised event types and logs warnings. Track successful parse counts and warn if no valid events are parsed.

**Implementation Requirements**:
- Add event type validation against known types
- Log warning for unrecognised event types (once per type)
- Track count of successfully parsed events
- Add `Validate()` method to check if any valid events were parsed
- Add version/format detection if Claude CLI provides it

**Acceptance Criteria**:
- [ ] Given valid stream-json, when parsed, then no warnings logged
- [ ] Given unrecognised event type, when parsed, then warning logged (once per type)
- [ ] Given no valid events parsed, when Validate() called, then returns error
- [ ] Given partial format change, when some events unrecognised, then valid events still processed
- [ ] Warning messages include guidance on version compatibility

**Definition of Done** (Single Commit):
- [ ] Feature complete in one atomic commit
- [ ] Known event types enumerated in parser
- [ ] Warning logging for unknown types
- [ ] Event count tracking
- [ ] Validate() method added
- [ ] Tests for unknown event handling
- [ ] Tests for empty parse detection

**Dependencies**:
- None

**Risks**:
- False positives if Claude CLI adds new event types we don't know about
- Mitigation: Warnings are informational, not errors; only fail if NO events parsed

**Notes**: Consider adding version check against Claude CLI if version info is available.

**Effort Estimate**: S (2-3 hours)

---

## Backlog Prioritisation

**Must Have (Sprint 1):**
- [x] SEC-1: Make permission skip flag configurable (security critical)
- [x] PERF-1: Implement ring buffer for TUI output (prevents OOM)
- [x] REL-1: Propagate errors from Queue.Pop() (prevents silent data loss)
- [x] REL-2: Add timeouts to git cleanup commands (prevents hangs)

**Should Have (Sprint 2):**
- [ ] PERF-2: Cache wrapped lines in TUI (UI responsiveness)
- [ ] PERF-3: Use strings.Builder for parser concatenation (easy win)
- [ ] PERF-4: Eliminate double parsing in executor (CPU efficiency)
- [ ] PERF-6: Increase scanner buffer limit (prevents data loss)

**Could Have (Sprint 3):**
- [ ] PERF-5: Add configurable output retention limit (memory safety)
- [ ] DESIGN-1: Add parser format validation and warnings (maintainability)

**Won't Have:**
- Async state file writes (too complex for current scope)
- State file versioning (future migration feature)
- Full parser rewrite (incremental improvements preferred)

---

## Technical Considerations

### Backwards Compatibility
- SEC-1 changes default behaviour (safer but breaking)
- REL-1 changes function signature (requires caller updates)
- All other changes are internal optimisations

### Testing Strategy
- Unit tests for each component change
- Integration tests for behaviour changes
- Benchmark tests for performance improvements
- Manual testing with long-running sessions for memory issues

### Architecture Patterns
- Ring buffer pattern for bounded memory (PERF-1, PERF-5)
- Cache invalidation pattern for performance (PERF-2)
- Context propagation for timeouts (REL-2)

---

## Success Metrics

### Security
- Default mode is safe (no `--dangerously-skip-permissions`)
- Opt-in required for dangerous mode

### Reliability
- No silent data loss from Queue operations
- No indefinite hangs from git commands
- Clear error messages for all failure modes

### Performance
- Memory usage bounded regardless of session length
- Scroll operations complete in < 10ms
- Large outputs (10MB+) processed without OOM

### Observability
- Warnings for unusual conditions (truncation, unknown formats)
- Clear error messages for configuration issues
