# Notes: Adversarial Review Critical Fixes

## 2026-01-25

### Session Start

Working through adversarial review fixes from `docs/plans/2026-01-25-160000-stories-adversarial-review-fixes.md`.

### SEC-1: Make permission skip flag configurable

**Completed**

Implementation details:
1. Added `DangerouslySkipPermissions bool` to `config.Config` in `internal/config/config.go`
2. Added `Dangerous bool` to `FileConfig` in `internal/config/file.go` for TOML support
3. Added `--dangerous` CLI flag to root command (default: false) in `cmd/orbital/root.go`
4. Modified `executor.BuildArgs()` to conditionally include `--dangerously-skip-permissions` only when enabled
5. Added warning message to stderr when dangerous mode is enabled
6. Updated existing tests that assumed `--dangerously-skip-permissions` was always present
7. Added new tests: `TestBuildArgs_WithDangerousMode`, `TestBuildArgs_WithoutDangerousMode`, `TestLoadFileConfig_WithDangerous`, `TestLoadFileConfig_WithoutDangerous`

Breaking change: By default, `--dangerously-skip-permissions` is no longer passed to Claude CLI. Users must explicitly opt-in via `--dangerous` flag or `dangerous = true` in config.

## Code Review - Iteration 1

### Security
No issues. The implementation follows secure-by-default principles:
- Boolean flag cannot be exploited for injection
- Go's `exec.CommandContext()` uses `execve()` syscall with separate argv array (not shell interpolation)
- Config file requires local filesystem access (already implies code execution capability)
- CLI flag takes precedence over config file for safe mode

### Design
No issues. The implementation follows existing patterns:
- Single Responsibility maintained across config, file config, and executor
- Coupling is appropriate (executor depends only on config.Config)
- Naming is consistent (CLI `--dangerous`, TOML `dangerous`, struct `DangerouslySkipPermissions`)
- Precedence logic matches existing patterns for `agents` flag

### Logic
**ISSUES FOUND**

**BUG: Missing `DangerouslySkipPermissions` in `continue.go`**

File: `cmd/orbital/continue.go` lines 161-176

The `continue` command creates a `config.Config` struct but does not include the `DangerouslySkipPermissions` field. This means:
1. `--dangerous` CLI flag is ignored when using `orbital continue`
2. `dangerous = true` in config file is not applied
3. Users expecting consistent behaviour between `orbital <spec>` and `orbital continue` will find dangerous mode silently fails to activate

Missing code:
- Line 176 should include `DangerouslySkipPermissions: dangerous,`
- After line 201, needs the config file override logic
- After line 201, needs the warning message when dangerous mode is enabled

### Error Handling
No issues. Boolean flags have safe defaults, nil checks are in place, warning is output to stderr.

### Data Integrity
No issues. Boolean type provides inherent validation, nil check for `fileConfig` is present, precedence logic is correct.

### Verdict
**FAIL** - Logic review found a critical bug in `continue.go` where the `DangerouslySkipPermissions` field is not set, causing inconsistent behaviour between `orbital` and `orbital continue` commands.

## Code Review Feedback - Addressed

### BUG FIX: `DangerouslySkipPermissions` missing in continue.go

**Fixed** in this iteration.

Changes made to `cmd/orbital/continue.go`:
1. Added `DangerouslySkipPermissions: dangerous,` to the config struct (line 176)
2. Added config file override logic after fileConfig handling (lines 204-208)
3. Added warning message when dangerous mode is enabled (lines 210-213)

The fix mirrors the implementation in `root.go` exactly, ensuring consistent behaviour between `orbital <spec>` and `orbital continue` commands.

Verification: `make check` passes (lint + tests).

### REL-1: Propagate errors from Queue.Pop()

**Completed**

Implementation details:
1. Changed `Pop()` signature from `func (q *Queue) Pop() []string` to `func (q *Queue) Pop() ([]string, error)`
2. Wrapped the pop logic in `withLock()` for consistency with `Add()` and `Remove()` operations
3. Return error from `save()` instead of ignoring it
4. Updated callers in `cmd/orbital/root.go` and `cmd/orbital/continue.go` to handle the error
5. Updated existing tests to handle the new return signature
6. Added new test `TestQueue_Pop_ReturnsErrorWhenSaveFails` to verify error propagation

The fix ensures that if the queue file cannot be saved (disk full, permissions), the error is propagated to callers. The files are still returned to allow the caller to decide how to handle the situation (proceed with warning or fail).

### REL-2: Add timeouts to git cleanup commands

**Completed**

Implementation details:
1. Modified `Cleanup.Run()` to accept a `context.Context` parameter
2. Added `runGitWithTimeout()` helper method that creates a 30-second timeout context
3. Updated all git commands in cleanup to use `exec.CommandContext` with timeout
4. Added proper timeout detection and error messages for timeouts
5. Added cancellation handling for parent context
6. Updated callers in `cmd/orbital/root.go` and `cmd/orbital/continue.go` to pass context
7. Added tests for context parameter and error handling

The fix uses the same 30-second timeout constant (`gitCommandTimeout`) from `git.go` to maintain consistency across all git operations in the worktree package.

### PERF-1: Implement ring buffer for TUI output

**Completed**

Implementation details:
1. Created `RingBuffer` type in `internal/tui/ringbuffer.go` with configurable max size
2. Default capacity is 10,000 lines (`DefaultMaxOutputLines` constant)
3. Changed `Model.outputLines` from `[]string` to `*RingBuffer`
4. Updated `NewModel()` to initialise ring buffer
5. Updated `OutputLineMsg` handler to use `Push()` instead of `append()`
6. Updated `wrapAllOutputLines()` to use `Iterate()` instead of range loop
7. Updated `AppendOutput()` and `ClearOutput()` methods
8. Updated tests in `model_test.go` to use ring buffer methods
9. Added comprehensive tests in `ringbuffer_test.go`:
   - Push below/at/over capacity
   - Get with out-of-range index
   - ToSlice for empty, partial, and wrapped buffers
   - Clear and reuse
   - Iterate with early termination
   - Memory bound verification (50,000 pushes maintains exactly 10,000 lines)

The ring buffer evicts the oldest lines when capacity is reached, ensuring memory usage remains bounded regardless of session length. Scroll position handling continues to work correctly because the scroll offset is always relative to the current buffer content.

## Sprint 1 Complete

All "Must Have (Sprint 1)" items from the adversarial review have been completed:

1. **SEC-1**: Make permission skip flag configurable (plus bug fix for continue.go from code review)
2. **REL-1**: Propagate errors from Queue.Pop()
3. **REL-2**: Add timeouts to git cleanup commands
4. **PERF-1**: Implement ring buffer for TUI output

Remaining items (PERF-2, PERF-3, PERF-4, PERF-5, PERF-6, DESIGN-1) are explicitly marked as Sprint 2 and Sprint 3 in the stories file, outside the scope of this iteration.

All verification checks pass: `make check` (lint + tests).

## Code Review - Iteration 2 (Sprint 1 Complete)

### Security
**ISSUES FOUND**

**Command Injection via Unvalidated Branch Names in Merge Phase**

File: `internal/worktree/merge.go` lines 80-106 and `cmd/orbital/root.go` line 266

The merge phase constructs prompts containing branch names that Claude will execute as git commands. The `originalBranch` value comes from `worktree.GetCurrentBranch()` which only trims whitespace but does not validate against shell metacharacters.

Attack scenario: An attacker creates a branch with a malicious name (e.g., `main; rm -rf /`), runs orbital in worktree mode, and on merge phase the unvalidated branch name gets embedded in Claude's prompt.

Severity: MEDIUM (requires local attacker who can create branches, and Claude's execution environment would need to allow shell metacharacters)

Recommendation: Add branch name validation in `GetCurrentBranch()` to reject names containing shell metacharacters.

### Design
**ISSUES FOUND**

1. **DRY Violation (HIGH)**: Flag precedence logic for `DangerouslySkipPermissions` is duplicated in `root.go` and `continue.go`. Should be extracted to shared utility.

2. **Tight Coupling (MEDIUM)**: `Model.outputLines` directly holds `*RingBuffer` pointer. Should use interface for flexibility.

3. **Abstraction Leakage (MEDIUM)**: RingBuffer exposes internal fields (`head`, `count`, `cap`) that could allow invariant violations.

4. **Naming Inconsistency (LOW)**: Mismatch between `Dangerous` (TOML/CLI) and `DangerouslySkipPermissions` (Config struct).

### Logic
**ISSUES FOUND**

**Queue.Pop() Returns Files Even When Persistence Fails**

File: `internal/state/queue.go` lines 181-199

When `q.save()` fails, the function clears the in-memory queue state before the save attempt, then returns the files with an error. This creates state inconsistency:
- Files are returned to caller (who may process them)
- Queue file on disk still contains old data (save failed)
- On next load, files reappear causing duplicate processing

Severity: MEDIUM (requires disk failure scenario, but causes duplicate work)

Recommendation: Save before clearing state, or return empty slice on save failure.

### Error Handling
**ISSUES FOUND**

1. **Partial Cleanup State (MEDIUM)**: `Cleanup.Run()` removes worktree first, then attempts branch deletion. If branch deletion fails after worktree removal, state is inconsistent (orphaned branch). Should delete branch first.

2. **Swallowed Cleanup Errors (MEDIUM)**: In `root.go` and `continue.go`, cleanup errors are logged as warnings but execution continues. This can leave orphaned worktrees.

3. **Missing Early Context Check (LOW)**: `runGitWithTimeout()` doesn't check if parent context is already cancelled before starting git command.

4. **Lost Error Context (LOW)**: When timeout AND command error occur together, timeout error is returned, losing the actual git error message.

### Data Integrity
No issues. Ring buffer implementation is correct with proper bounds checking, nil safety, and GC-friendly clearing.

### Verdict
**FAIL**

Summary of issues by severity:
- **HIGH**: 1 (DRY violation in flag precedence logic)
- **MEDIUM**: 5 (command injection risk, Queue.Pop state consistency, partial cleanup, swallowed errors, tight coupling)
- **LOW**: 4 (naming inconsistency, abstraction leakage, missing early context check, lost error context)

The most critical issues that should be addressed:
1. Queue.Pop() should not return files if save fails (prevents duplicate processing)
2. Cleanup.Run() should delete branch before removing worktree (prevents orphaned branches)
3. Flag precedence logic should be extracted to shared utility (prevents divergence)

Note: These issues are design improvements and edge case handling. The core Sprint 1 functionality is correct for normal operation. The issues identified are for hardening against failure scenarios.

## Sprint 2 Progress

### PERF-2: Cache wrapped lines in TUI

**Completed**

Implementation details:
1. Added three new fields to Model struct:
   - `wrappedLinesCache []string` - cached wrapped output lines
   - `cacheWidth int` - width used for current cache
   - `cacheLineCount int` - number of raw lines when cache was built
2. Added `updateWrappedLinesCache()` method to rebuild cache when needed
3. Added `invalidateWrappedLinesCache()` method to clear cache
4. Modified `wrapAllOutputLines()` to return cached lines directly
5. Updated `WindowSizeMsg` handler to invalidate and rebuild cache on resize
6. Updated `OutputLineMsg` handler to incrementally append to cache or rebuild if ring buffer wrapped
7. Updated `AppendOutput()` method to maintain cache incrementally
8. Updated `ClearOutput()` method to invalidate cache
9. Added comprehensive tests in `TestWrappedLinesCaching`:
   - Cache populated after window size set
   - Cache reused on scroll operations
   - Cache invalidated on window width change
   - Cache updated incrementally on new line
   - Cache cleared on ClearOutput
   - Scrolling 1000 times is fast with caching (verified with 5000 lines)

The implementation handles the ring buffer wrap case where old lines are evicted: if the line count doesn't match expectations (cacheLineCount + 1), the cache is fully rebuilt.

Verification: `make check` passes (lint + tests).
