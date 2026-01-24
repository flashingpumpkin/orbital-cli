# User Stories: Fix Worktree Cleanup Branch Deletion

## Problem Statement

When running `orbit-cli --worktree`, the cleanup phase fails with:

```
Warning: failed to cleanup worktree: failed to delete branch: exit status 1
```

### Root Cause Analysis

The branch name extracted during setup is malformed. For example:
- Expected: `orbit/fix-todo-tracking`
- Actual: `orbit/fix-todo-trackingsuccess`

The "success" suffix is incorrectly appended because `ExtractText` in `internal/output/parser.go` concatenates text content from multiple stream-json events without preserving newlines between them.

**Code path:**
1. `internal/output/parser.go:ExtractText` concatenates event content without newlines
2. `internal/worktree/setup.go:extractMarker` searches for newline to terminate marker value
3. No newline found, so everything to end of string becomes the value
4. Branch name becomes `orbit/fix-todo-trackingsuccess` instead of `orbit/fix-todo-tracking`
5. During cleanup, `git branch -d orbit/fix-todo-trackingsuccess` fails because that branch does not exist

**Affected code:**
- `internal/output/parser.go:346-356` (ExtractText function)
- `internal/worktree/setup.go:125-147` (extractMarker function)

---

## User Story 1: Preserve Newlines in ExtractText

**As a** developer using orbit-cli
**I want** text extraction to preserve newline boundaries between events
**So that** marker extraction correctly identifies line boundaries

### Acceptance Criteria

- [x] `ExtractText` adds newline separators between content from different events
- [x] Content blocks that end with text (not already ending in newline) get a newline appended
- [x] Existing tests continue to pass
- [x] New test validates newline preservation across multiple events

### Technical Notes

Modify `ExtractText` in `internal/output/parser.go`:

```go
func ExtractText(rawOutput string) string {
    parser := NewParser()
    var text strings.Builder
    for _, line := range strings.Split(rawOutput, "\n") {
        event, _ := parser.ParseLine([]byte(line))
        if event != nil && event.Content != "" {
            text.WriteString(event.Content)
            // Preserve line boundaries between events
            if !strings.HasSuffix(event.Content, "\n") {
                text.WriteString("\n")
            }
        }
    }
    return text.String()
}
```

### Definition of Done

- [x] Code change implemented
- [x] Unit tests pass
- [x] Integration with marker extraction verified

---

## User Story 2: Add Robust Marker Extraction Test

**As a** developer
**I want** tests that catch marker extraction edge cases
**So that** regressions like this are caught early

### Acceptance Criteria

- [x] Test case for marker followed immediately by other text on same logical line
- [x] Test case for marker with trailing text in subsequent event
- [x] Test validates branch names are extracted correctly when output contains "success" messages

### Technical Notes

Add test in `internal/worktree/setup_test.go`:

```go
func TestExtractMarker_WithTrailingText(t *testing.T) {
    // Simulate output where marker is followed by success message
    // This should extract only the marker value, not trailing text
}
```

### Definition of Done

- [x] Test cases added
- [x] Tests fail with current code, pass with fix
- [x] Coverage maintained or improved

---

## User Story 3: Improve Cleanup Error Messages

**As a** user of orbit-cli
**I want** descriptive error messages when cleanup fails
**So that** I can understand what went wrong and how to fix it

### Acceptance Criteria

- [x] Error message includes the actual branch name that failed to delete
- [x] Error message includes the git command that was attempted
- [x] Error includes git's stderr output for debugging

### Technical Notes

Modify `Cleanup.Run` in `internal/worktree/merge.go`:

```go
func (c *Cleanup) Run(worktreePath, branchName string) error {
    // ...
    branchCmd := exec.Command("git", "-C", c.workingDir, "branch", "-d", branchName)
    output, err := branchCmd.CombinedOutput()
    if err != nil {
        // Include git's output in error for debugging
        return fmt.Errorf("failed to delete branch %q: %w\ngit output: %s", branchName, err, string(output))
    }
    // ...
}
```

### Definition of Done

- [x] Error messages include branch name and git output
- [x] User can diagnose issue from error message alone

---

## Implementation Order

1. Story 1 (root cause fix)
2. Story 2 (regression tests)
3. Story 3 (improved diagnostics)

## Verification

After implementation, run:
```bash
orbit-cli --workflow fast --worktree docs/plans/2026-01-24-stories-fix-todo-tracking.md
```

Cleanup should complete without warnings.

---

<promise>COMPLETE</promise>
