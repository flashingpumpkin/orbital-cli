# Notes: Fix Worktree Cleanup Branch Deletion

## 2026-01-24

### Implementation Summary

All three user stories have been completed:

1. **Story 1: Preserve Newlines in ExtractText** (`internal/output/parser.go:346-364`)
   - Modified `ExtractText` to append a newline after content that does not already end with one
   - This ensures line boundaries are preserved between stream-json events
   - Updated existing tests to reflect the new behaviour

2. **Story 2: Add Robust Marker Extraction Tests** (`internal/worktree/setup_test.go:227-289`)
   - Added three new test cases covering the specific bug scenario:
     - `extracts branch name when followed by success in separate event`
     - `extracts worktree path when followed by success in separate event`
     - `extracts branch correctly when many events follow`
   - Also added a test to `internal/output/parser_test.go` for the core behaviour

3. **Story 3: Improve Cleanup Error Messages** (`internal/worktree/merge.go:108-126`)
   - Modified `Cleanup.Run` to use `CombinedOutput()` instead of `Run()`
   - Error messages now include the specific path/branch name that failed
   - Error messages include git command output for debugging

### Root Cause

The bug occurred because `ExtractText` concatenated content from multiple stream-json events without preserving line boundaries. When Claude output:

```
BRANCH_NAME: orbit/fix-todo-tracking
```

followed by:

```
success
```

in separate events, the extracted text became `BRANCH_NAME: orbit/fix-todo-trackingsuccess` (no newline between them). The `extractMarker` function then extracted everything to end-of-string as the branch name.

### Verification

All tests pass:
- `go test ./...` passes
- `go build ./...` succeeds
