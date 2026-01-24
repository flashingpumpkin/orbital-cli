# User Stories: Worktree Merge Phase Robustness

## Problem Statement

The merge phase is the most critical part of the worktree lifecycle - it's where completed work gets integrated back into the main branch. Several issues can cause merge to fail silently or produce incorrect results:

1. **Case-sensitive marker matching**: "MERGE_SUCCESS: True" won't match
2. **No merge timeout**: Claude can hang on complex conflicts
3. **No merge validation**: We trust Claude's output without verification
4. **Rebase can silently break code**: No post-merge verification

**Affected code:**
- `internal/worktree/merge.go` - Merge execution and marker detection
- `cmd/orbital/root.go:532-579` - Merge orchestration

---

## User Story 1: Case-Insensitive Merge Success Detection

**As a** developer
**I want** merge success detection to be robust
**So that** valid merges aren't incorrectly marked as failures

### Context

The marker check is case-sensitive and whitespace-sensitive:

```go
// merge.go:60-61
const marker = "MERGE_SUCCESS: true"
return strings.Contains(text, marker)
```

Claude outputting `MERGE_SUCCESS:true` (no space), `MERGE_SUCCESS: True`, or `merge_success: true` will fail silently.

### Acceptance Criteria

- [ ] Given Claude outputs "MERGE_SUCCESS: true", when marker is checked, then merge is marked successful
- [ ] Given Claude outputs "MERGE_SUCCESS: True", when marker is checked, then merge is marked successful
- [ ] Given Claude outputs "MERGE_SUCCESS:true" (no space), when marker is checked, then merge is marked successful
- [ ] Given Claude outputs "merge_success: true", when marker is checked, then merge is marked successful
- [ ] Given Claude outputs "MERGE_SUCCESS: false", when marker is checked, then merge is marked failed
- [ ] Given no marker is found, when merge completes, then merge is marked failed with warning

### Technical Notes

```go
// internal/worktree/merge.go

import "regexp"

var mergeSuccessPattern = regexp.MustCompile(`(?i)merge[_\s]*success\s*:\s*(true|false)`)

func containsSuccessMarker(rawOutput string) bool {
    text := output.ExtractText(rawOutput)
    matches := mergeSuccessPattern.FindStringSubmatch(text)
    if matches == nil {
        return false
    }
    return strings.EqualFold(matches[1], "true")
}
```

### Definition of Done

- [ ] Regex pattern for flexible matching implemented
- [ ] Case-insensitive matching for "true"/"false"
- [ ] Tests for all acceptance criteria variations
- [ ] All existing tests pass

**Effort Estimate**: XS

---

## User Story 2: Add Merge Phase Timeout

**As a** developer
**I want** the merge phase to have a timeout
**So that** it doesn't hang indefinitely on complex merges

### Context

Merge phase invokes Claude with no explicit timeout. If Claude gets stuck in a conflict resolution loop, it runs indefinitely.

### Acceptance Criteria

- [ ] Given merge takes longer than timeout, when timeout expires, then merge is cancelled
- [ ] Given timeout occurs, when error is shown, then worktree is preserved
- [ ] Given `--merge-timeout` flag is set, when merge runs, then custom timeout is used
- [ ] Given default timeout (10 minutes), when merge completes in time, then normal flow continues
- [ ] Given timeout occurs, when error is shown, then partial merge state is described

### Technical Notes

```go
// cmd/orbital/root.go

var mergeTimeout time.Duration

func init() {
    rootCmd.PersistentFlags().DurationVar(&mergeTimeout, "merge-timeout", 10*time.Minute, "Timeout for merge phase")
}

// In runOrbit:
mergeCtx, mergeCancel := context.WithTimeout(ctx, mergeTimeout)
defer mergeCancel()

mergeResult, mergeErr := runWorktreeMerge(mergeCtx, worktreeMergeOptions{...})

if errors.Is(mergeErr, context.DeadlineExceeded) {
    fmt.Fprintf(os.Stderr, "Merge phase timed out after %v\n", mergeTimeout)
    fmt.Fprintf(os.Stderr, "Worktree preserved at: %s\n", wtState.Path)
    fmt.Fprintf(os.Stderr, "Complete merge manually or increase timeout with --merge-timeout\n")
    os.Exit(4)
}
```

### Definition of Done

- [ ] `--merge-timeout` flag added with 10 minute default
- [ ] Context timeout applied to merge phase
- [ ] Clear error message on timeout
- [ ] Worktree preserved on timeout
- [ ] Tests for timeout behavior
- [ ] All existing tests pass

**Effort Estimate**: S

---

## User Story 3: Validate Merge Actually Succeeded

**As a** developer
**I want** merge success to be verified with git commands
**So that** I don't rely solely on Claude's output

### Context

Currently, we trust Claude to output "MERGE_SUCCESS: true" only if the merge actually worked. But Claude could output this marker even if the merge failed or was incomplete.

### Acceptance Criteria

- [ ] Given merge marker says success, when validation runs, then git state is checked
- [ ] Given worktree branch is not merged into original, when validation fails, then error is shown
- [ ] Given validation fails, when error is shown, then worktree is preserved
- [ ] Given validation passes, when merge completes, then cleanup proceeds
- [ ] Given `--skip-merge-validation` flag is set, when merge completes, then validation is skipped

### Technical Notes

```go
// internal/worktree/merge.go

func (m *Merge) ValidateMergeSuccess(workingDir, worktreeBranch, originalBranch string) error {
    // Check that worktree branch is an ancestor of original branch
    // (meaning all commits from worktree are in original)
    cmd := exec.Command("git", "-C", workingDir, "merge-base", "--is-ancestor",
        worktreeBranch, originalBranch)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("merge validation failed: worktree branch %q is not merged into %q",
            worktreeBranch, originalBranch)
    }
    return nil
}
```

### Definition of Done

- [ ] Merge validation function implemented
- [ ] Called after Claude reports success
- [ ] Clear error if validation fails
- [ ] Worktree preserved on validation failure
- [ ] `--skip-merge-validation` flag for escape hatch
- [ ] Tests for validation scenarios
- [ ] All existing tests pass

**Effort Estimate**: S

---

## User Story 4: Add Post-Merge Build Verification

**As a** developer
**I want** an optional build/test verification after merge
**So that** I catch rebase-induced issues before cleanup

### Context

Rebase can silently break code if conflicts were resolved incorrectly. An optional post-merge verification step can catch this.

### Acceptance Criteria

- [ ] Given `--post-merge-verify` flag with command, when merge succeeds, then command is run
- [ ] Given verification command fails, when error is shown, then worktree is preserved
- [ ] Given verification command succeeds, when cleanup runs, then normal flow continues
- [ ] Given no flag is set, when merge succeeds, then cleanup proceeds without verification
- [ ] Given verification fails, when user decides to proceed, then `--force-cleanup` allows it

### Technical Notes

```go
// cmd/orbital/root.go

var postMergeVerify string

func init() {
    rootCmd.PersistentFlags().StringVar(&postMergeVerify, "post-merge-verify", "",
        "Command to run after merge to verify success (e.g., 'go build ./...')")
}

// After successful merge:
if postMergeVerify != "" {
    fmt.Printf("Running post-merge verification: %s\n", postMergeVerify)
    verifyCmd := exec.Command("sh", "-c", postMergeVerify)
    verifyCmd.Dir = workingDir
    verifyCmd.Stdout = os.Stdout
    verifyCmd.Stderr = os.Stderr
    if err := verifyCmd.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Post-merge verification failed: %v\n", err)
        fmt.Fprintf(os.Stderr, "Worktree preserved at: %s\n", wtState.Path)
        fmt.Fprintf(os.Stderr, "Fix issues and run 'orbital worktree cleanup' manually\n")
        os.Exit(4)
    }
}
```

### Definition of Done

- [ ] `--post-merge-verify` flag implemented
- [ ] Command executed in main repo directory
- [ ] Clear output of verification command
- [ ] Worktree preserved on verification failure
- [ ] `--force-cleanup` flag for override
- [ ] Tests for verification flow
- [ ] All existing tests pass

**Effort Estimate**: S

---

## Implementation Order

1. **Story 1** (Case-insensitive matching) - Quick fix, high impact
2. **Story 2** (Timeout) - Prevents hangs
3. **Story 3** (Validation) - Defense in depth
4. **Story 4** (Post-merge verification) - Optional enhancement

## Verification

```bash
# Test case-insensitive matching (requires mock)
# See merge_test.go for marker detection tests

# Test timeout
orbital --worktree --merge-timeout 10s spec.md
# (simulate slow merge)

# Test validation
# Manually corrupt merge state and verify detection

# Test post-merge verification
orbital --worktree --post-merge-verify "go build ./..." spec.md
```

---

## Dependencies

- Stories 1-3 are independent
- Story 4 depends on merge phase completion

## Risks

- Merge validation may have edge cases with complex merge histories
- Post-merge verification command could have side effects
