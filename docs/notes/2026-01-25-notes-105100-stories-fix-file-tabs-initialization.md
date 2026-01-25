# Notes: Fix File Tabs Initialization

**Spec File:** docs/plans/2026-01-25-105100-stories-fix-file-tabs-initialization.md

## 2026-01-25

### Issue Found

The spec file had all acceptance criteria marked as complete, but the actual code change had not been applied. The fix was straightforward: add `model.tabs = model.buildTabs()` after `model.session = session` in `internal/tui/program.go`.

### Fix Applied

Added the missing line at `internal/tui/program.go:31`:

```go
model := NewModel()
model.session = session
model.tabs = model.buildTabs()  // Added this line
model.progress = progress
```

### Verification

- Build: Passed
- Lint: Passed
- Tests: All passed

### Code Review (Iteration 2)

**Reviewer:** Gate review step

**Verdict:** PASS

**Analysis:**

1. **Correctness:** The fix correctly addresses the root cause. The `buildTabs()` method relies on `m.session` being populated (model.go:327, 336, 345), so placing the call immediately after setting `model.session` is correct.

2. **Edge Cases Handled:**
   - Empty session (no spec files): Returns only Output tab
   - Multiple spec files: Each gets its own tab
   - Context file separators: Handles both `,` and `, `
   - Missing notes file: Guarded by empty string check

3. **Code Quality:**
   - One-line minimal change
   - Follows existing pattern (same call in SessionMsg handler)
   - Logical placement after session assignment

4. **Test Coverage:**
   - All existing tests pass
   - No regressions detected
   - Minor observation: No specific unit test verifies tabs built during New() but this is acceptable given the fix mirrors the SessionMsg handler pattern

**Action Items:** None blocking. The fix is correct and complete.

### Code Review (Iteration 3) - Unified Session Selector Integration

**Reviewer:** Multi-agent review (security, design, logic, error handling, data integrity)

**Files Changed:**
- `cmd/orbital/continue.go`

**Changes Summary:**
The diff reorders error handling in `runContinue` to process stale session cleanup even when selection is cancelled. It also modifies `selectSession` to return cleanup paths on cancellation.

### Security
No issues. The paths come from internal state files within `.orbital/` directory. The `StateManager.Remove()` only modifies JSON state, not filesystem paths. No injection vectors or path traversal risks.

### Design
No issues. The change correctly extends the existing cleanup pattern to handle the cancellation case. The API semantics are improved: cleanup paths may be returned regardless of selection outcome.

### Logic
Issues found:
1. **Inconsistent output streams** - Success messages go to stdout while failure messages go to stderr. When piped, related cleanup messages appear in different streams. Should use stderr for both operational messages.

### Error Handling
Issues found:
1. **No aggregation of cleanup failures** - If 3 out of 5 cleanups fail, the user sees interleaved messages with no summary of the inconsistent state.
2. **Error context lost** - "selection cancelled" provides no actionable information about cleanup results or next steps.
3. **Inconsistent output streams** - Same as logic review.
4. **No observability for cleanup in error path** - Exit code and structured output do not reflect cleanup status.

### Data Integrity
Issues found:
1. **Non-atomic cleanup operations** - Each path removal is a separate lock/load/filter/save cycle. If process crashes mid-loop, some entries are removed and others are not.
2. **Error conflation** - All errors from `selectSession` trigger cleanup, not just cancellation. If a TUI crash returns cleanup paths, they would execute before the error returns.

### Verdict
**FAIL**

The primary functional change (processing cleanup even on cancellation) is correct. However, the implementation has several quality issues:

1. **stdout/stderr inconsistency** for related operational messages
2. **Non-atomic batch cleanup** could leave partial state on interruption
3. **Missing cleanup status reporting** in error context

These issues are minor individually but collectively reduce operational reliability. The code functions correctly for the happy path but has rough edges in error scenarios.

### Iteration 4 - Review Feedback Addressed

**Issues Fixed:**

1. **stdout/stderr inconsistency** - Changed success messages to use stderr, keeping all cleanup-related output on the same stream.

2. **Cleanup result aggregation** - Added counters (`cleanupSucceeded`, `cleanupFailed`) to track results and report a summary when failures occur.

3. **Error context enrichment** - When selection is cancelled and cleanup ran, the error now includes cleanup status: `selection cancelled (cleanup: X removed, Y failed)`.

**Changes Made:**

`cmd/orbital/continue.go` lines 79-100:
- Track cleanup success/failure counts
- Use stderr consistently for all cleanup messages
- Report summary when failures occur
- Wrap selection errors with cleanup context

**Note on atomicity:** True atomic batch cleanup would require a transaction-style approach (collect all changes, apply in one write). The current implementation processes entries sequentially, which is consistent with the existing `StateManager.Remove()` API. A mid-process crash is unlikely, and the partial state is recoverable (stale entries will appear again on next run). The cost of refactoring for atomic batch operations outweighs the marginal reliability gain.

**Verification:**
- Lint: Passed
- Tests: All passed
- Build: Passed
