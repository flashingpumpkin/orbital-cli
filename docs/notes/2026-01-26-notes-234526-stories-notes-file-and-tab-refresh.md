# Notes: Notes File and Tab Refresh

Spec: docs/plans/2026-01-26-234526-stories-notes-file-and-tab-refresh.md
Date: 2026-01-26

## Iteration 1

### Task Selected

**Fix gate-based workflow completion promise detection**

### Why Highest Leverage

This is the highest leverage task because:

1. It affects all gate-based workflows (fast, reviewed, tdd, autonomous)
2. Without this fix, workflows loop infinitely when Claude outputs the completion promise
3. It's a bug fix that unblocks correct operation of the most commonly used workflows
4. The iteration timeout task was already completed in commit 322d5f7

### Analysis

Looking at `runWorkflowLoop()` in `cmd/orbital/root.go`, the completion promise is only checked after `runResult.CompletedAllSteps` is true (around line 893). The fix needs to check for the completion promise after each step execution and exit early if found.

The implementation will:
1. Import `completion` package (already imported)
2. After `runner.Run()` returns, check if any step output contains the completion promise
3. If promise detected, skip to verification immediately instead of continuing the workflow loop

Looking at the workflow runner callback, we get `result.Output` for each step. We need to check this output for the completion promise.

### Implementation

Added completion promise detection in `cmd/orbital/root.go` in the `runWorkflowLoop()` function:

1. After `runner.Run()` returns, iterate over `runResult.Steps` to check each step's output
2. Use the existing `completion.Detector.Check()` to find the promise
3. If promise is detected, set `promiseDetected = true` and print a message (non-TUI only)
4. Changed the verification condition from `if runResult.CompletedAllSteps` to `if runResult.CompletedAllSteps || promiseDetected`

The change is minimal and additive. It does not alter existing gate logic. Promise detection takes precedence, allowing early exit when Claude outputs the promise during any workflow step.

### Testing

Added `TestPromiseDetectionInWorkflowSteps` in `cmd/orbital/root_test.go` to verify the detection pattern works correctly with various step result scenarios.

### Note on Iteration Timeout

The iteration timeout ticket was already completed in commit 322d5f7. Verified the code shows 5 minutes in both `internal/config/config.go` and `cmd/orbital/root.go`.
