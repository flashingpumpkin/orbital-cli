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

## Code Review - Iteration 1

### Security
No issues. The promise detection is a feature, not a security control. The verification step provides the actual completion assurance. Process spawning uses proper argument passing without shell interpretation. No injection risks found.

### Design
_ISSUES_FOUND

1. **Duplicate Detector Instantiation** (DRY violation): Detector created at line 248 and again at line 894 inside the workflow loop. Should pass detector to `runWorkflowLoop()` instead.

2. **Feature Envy**: `runWorkflowLoop` duplicates substantial logic from `loop.Controller.Run()` (budget checking, promise detection, verification, queue management). Bug fixes must be applied in two places.

3. **SRP Violation**: `runWorkflowLoop` is 250 lines handling 8+ responsibilities (loop state, workflow execution, TUI updates, promise detection, verification, queue management, output formatting, error handling). Should be broken into smaller functions.

4. **Boolean Flag Scattered**: `tuiProgram == nil` checks appear 11 times. Should use Strategy pattern with a `ProgressReporter` interface.

5. **runVerification Duplicates Controller.verifyCompletion**: Near-identical verification logic in two places with different error handling.

### Logic
No critical issues. Empty steps handled correctly. Range over nil slice is safe in Go. The OR condition behaves correctly. Minor observation: redundant detector creation per iteration is wasteful but not buggy.

### Error Handling
_ISSUES_FOUND

1. **Silent Error Swallowing in TUI Mode** (MEDIUM): Lines 920-925 - verification errors are completely invisible when TUI is enabled. Only console mode prints the error.

2. **Verification Cost Lost on Execution Error** (MEDIUM): When `Execute` fails in `runVerification`, the cost/tokens consumed are lost from tracking (returns nil result on error).

3. **No Observability for Promise Detection in TUI Mode** (LOW): TUI receives no notification when promise is detected in a specific step.

### Data Integrity
_ISSUES_FOUND

1. **Missing nil check for stepResult pointer** (HIGH): Each `stepResult` in `runResult.Steps` is a pointer (`[]*StepResult`). No nil check before accessing `stepResult.Output`. Would panic if any element is nil.

2. **Empty promise string causes false positives** (HIGH): If `cfg.CompletionPromise` is empty, `strings.Contains(output, "")` returns true for any non-empty output. Config validation does not check for empty promise.

3. **Missing test coverage for edge cases** (MEDIUM): Tests don't cover nil steps slice, nil elements within steps slice, or empty promise string.

4. **Duplicate detector instantiation** (LOW): Creates maintenance risk if one instance is changed without updating the other.

### Verdict
**FAIL**

Critical issues requiring attention before merge:

1. Empty `CompletionPromise` validation missing - could cause false positive completion detection
2. No nil safety for stepResult pointers in the loop
3. Verification errors invisible in TUI mode
4. Significant code duplication between `runWorkflowLoop` and `loop.Controller`

The design issues (SRP, duplicate logic) are technical debt but not blocking. The data validation and error visibility issues should be fixed.

## Iteration 2 - Code Review Fixes

### Issues Addressed

All four critical issues from the code review have been fixed:

**1. Empty CompletionPromise validation (HIGH)**
- Added validation in `internal/config/config.go` `Validate()` method
- Returns error "completion promise cannot be empty" if `CompletionPromise` is empty
- Prevents false positive completion detection where `strings.Contains(output, "")` would return true

**2. Nil safety for stepResult pointers (HIGH)**
- Added nil check in `cmd/orbital/root.go` line 895: `if stepResult == nil { continue }`
- Prevents panic when iterating over `runResult.Steps` if any element is nil

**3. Verification errors invisible in TUI mode (MEDIUM)**
- Added `SendOutput` method to `internal/tui/program.go`
- Updated verification error handling in `runWorkflowLoop` to send messages to TUI
- Verification errors, unchecked item counts, and success messages now appear in TUI output panel
- Messages prefixed with warning/success indicators for visibility

**4. Duplicate detector instantiation (LOW)**
- Removed duplicate `completion.New()` call from `runWorkflowLoop`
- Changed to use `strings.Contains()` directly with `cfg.CompletionPromise`
- This is safe because config validation now ensures promise is non-empty

### Design Issues Not Addressed (Tech Debt)

The following design issues were noted but not fixed in this iteration as they are not blocking:
- Feature envy between `runWorkflowLoop` and `loop.Controller`
- SRP violation in `runWorkflowLoop` (250 lines, 8+ responsibilities)
- Scattered `tuiProgram == nil` checks (could use Strategy pattern)
- Duplicate verification logic between `runVerification` and `Controller.verifyCompletion`

### Verification

All tests pass: `make check` successful

## Iteration 3 - Auto-create Notes File

### Task Selected

**Auto-create notes file when CLI starts**

### Implementation

Added `ensureNotesFile()` function in `cmd/orbital/root.go` that:
1. Checks if the notes file already exists (preserves existing content)
2. Creates the file with a header if it doesn't exist
3. Header format: `# Notes\n\nSpec: <spec-basename>\nDate: <YYYY-MM-DD>\n`
4. Logs a warning on error but doesn't fail startup

Integrated the function call after the notes directory is created (line 232) in `runOrbit()`.

### Testing

Added three tests in `cmd/orbital/root_test.go`:
- `TestEnsureNotesFile_CreatesNewFile`: Verifies file creation with proper header
- `TestEnsureNotesFile_PreservesExistingFile`: Verifies existing files are not overwritten
- `TestEnsureNotesFile_HandlesNestedPath`: Verifies function works with existing directories

All tests pass: `make check` successful
