# User Stories: Worktree Resume Validation

## Problem Statement

When resuming a worktree session with `orbital continue`, the code assumes the worktree still exists and is valid. Several scenarios can cause confusing failures:

1. **Worktree manually deleted**: Path from state no longer exists on disk
2. **Detached HEAD at creation**: Original "branch" was "HEAD", breaks merge
3. **Original branch deleted/renamed**: Merge target no longer exists
4. **Git worktree deregistered**: Directory exists but not a git worktree

**Affected code:**
- `cmd/orbital/continue.go:62-83` - Resume logic
- `cmd/orbital/root.go:532-579` - Merge phase
- `internal/worktree/setup.go:35-42` - GetCurrentBranch

---

## User Story 1: Validate Worktree Exists Before Resume

**As a** developer resuming a worktree session
**I want** orbital to verify the worktree still exists
**So that** I get a clear error instead of cryptic failures

### Context

`continue.go:78-83` uses the worktree path from state without checking if it exists:

```go
if wtState != nil {
    files = wtState.SpecFiles
    effectiveWorkingDir = wtState.Path  // Never validated!
    fmt.Printf("Resuming in worktree: %s\n\n", effectiveWorkingDir)
}
```

If the worktree was manually deleted (e.g., `rm -rf .orbital/worktrees/swift-falcon`), errors appear deep in execution.

### Acceptance Criteria

- [ ] Given worktree path in state does not exist, when `orbital continue` runs, then error message says "Worktree directory not found: <path>"
- [ ] Given worktree path exists but is not a git worktree, when `orbital continue` runs, then error says "Directory exists but is not a git worktree"
- [ ] Given worktree is valid, when `orbital continue` runs, then resume proceeds normally
- [ ] Given worktree is missing, when error is shown, then user is told how to clean up: "Run 'orbital worktree cleanup' to remove stale entries"

### Technical Notes

```go
// cmd/orbital/continue.go

func validateWorktree(wtState *worktree.WorktreeState) error {
    // Check path exists
    info, err := os.Stat(wtState.Path)
    if os.IsNotExist(err) {
        return fmt.Errorf("worktree directory not found: %s\nRun 'orbital worktree cleanup' to remove stale entries", wtState.Path)
    }
    if err != nil {
        return fmt.Errorf("failed to check worktree path: %w", err)
    }
    if !info.IsDir() {
        return fmt.Errorf("worktree path is not a directory: %s", wtState.Path)
    }

    // Check it's a git worktree (has .git file, not directory)
    gitPath := filepath.Join(wtState.Path, ".git")
    gitInfo, err := os.Stat(gitPath)
    if os.IsNotExist(err) {
        return fmt.Errorf("not a git worktree (missing .git): %s", wtState.Path)
    }
    if gitInfo.IsDir() {
        return fmt.Errorf("not a git worktree (.git is directory, not file): %s", wtState.Path)
    }

    return nil
}
```

### Definition of Done

- [ ] Validation function implemented
- [ ] Called before setting `effectiveWorkingDir`
- [ ] Clear error messages with remediation steps
- [ ] Test cases for missing path, non-worktree directory
- [ ] All existing tests pass

**Effort Estimate**: S

---

## User Story 2: Handle Detached HEAD State

**As a** developer on a detached HEAD
**I want** orbital to refuse worktree mode with a clear error
**So that** I don't end up in an unrecoverable state

### Context

`GetCurrentBranch()` returns "HEAD" for detached state:

```go
// setup.go:35-42
cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
// Returns "HEAD" when detached
```

The merge phase then tries to checkout "HEAD" as a branch name, which fails confusingly.

### Acceptance Criteria

- [ ] Given repository is in detached HEAD state, when `orbital --worktree` runs, then error says "worktree mode requires a branch, currently on detached HEAD"
- [ ] Given error is shown, when user reads it, then suggestion says "checkout a branch first: git checkout -b <branch-name>"
- [ ] Given repository has a branch checked out, when `orbital --worktree` runs, then setup proceeds normally

### Technical Notes

```go
// internal/worktree/setup.go

func GetCurrentBranch(dir string) (string, error) {
    cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to get current branch: %w", err)
    }

    branch := strings.TrimSpace(string(output))
    if branch == "HEAD" {
        return "", fmt.Errorf("worktree mode requires a branch, currently on detached HEAD\nCheckout a branch first: git checkout -b <branch-name>")
    }

    return branch, nil
}
```

### Definition of Done

- [ ] Detached HEAD detection implemented in GetCurrentBranch
- [ ] Clear error message with suggestion
- [ ] Test for detached HEAD scenario
- [ ] All existing tests pass

**Effort Estimate**: XS

---

## User Story 3: Validate Original Branch Before Merge

**As a** developer whose original branch was deleted while working
**I want** orbital to detect this before merge
**So that** I can choose an alternative target branch

### Context

No validation that `OriginalBranch` still exists before merge phase. If the branch was deleted or renamed, the merge prompt fails with a cryptic git error.

### Acceptance Criteria

- [ ] Given original branch no longer exists, when merge phase starts, then error says "Original branch '<name>' no longer exists"
- [ ] Given original branch is missing, when error is shown, then available branches are listed
- [ ] Given `--merge-target` flag is provided, when original is missing, then flag value is used instead
- [ ] Given original branch exists, when merge phase runs, then it proceeds normally

### Technical Notes

```go
// cmd/orbital/root.go (before merge phase)

func validateMergeTarget(workingDir, branchName string) error {
    cmd := exec.Command("git", "-C", workingDir, "rev-parse", "--verify", branchName)
    if err := cmd.Run(); err != nil {
        // Branch doesn't exist, list alternatives
        listCmd := exec.Command("git", "-C", workingDir, "branch", "--format=%(refname:short)")
        output, _ := listCmd.Output()
        branches := strings.TrimSpace(string(output))
        return fmt.Errorf("original branch %q no longer exists\nAvailable branches:\n%s\nUse --merge-target to specify an alternative", branchName, branches)
    }
    return nil
}
```

Add flag:
```go
rootCmd.PersistentFlags().StringVar(&mergeTarget, "merge-target", "", "Override target branch for merge (default: original branch)")
```

### Definition of Done

- [ ] Branch existence validation before merge
- [ ] Clear error with available branches listed
- [ ] `--merge-target` flag implemented
- [ ] Tests for missing branch scenario
- [ ] All existing tests pass

**Effort Estimate**: S

---

## User Story 4: Detect Diverged Original Branch

**As a** developer whose main branch advanced while working in a worktree
**I want** orbital to warn me before merge
**So that** I can decide how to handle the divergence

### Context

The merge prompt uses `--ff-only` which fails if the original branch has new commits. Users get a confusing error from Claude about merge failure.

### Acceptance Criteria

- [ ] Given original branch has commits not in worktree branch, when merge starts, then warning is shown
- [ ] Given branches have diverged, when warning is shown, then user sees commit count difference
- [ ] Given `--allow-merge-commit` flag is set, when branches diverge, then merge proceeds with merge commit
- [ ] Given no flag and divergence detected, when merge would fail, then worktree is preserved with explanation

### Technical Notes

```go
// Check for divergence before merge
func checkBranchDivergence(workingDir, worktreeBranch, originalBranch string) (int, error) {
    // Count commits in original not in worktree
    cmd := exec.Command("git", "-C", workingDir, "rev-list", "--count",
        fmt.Sprintf("%s..%s", worktreeBranch, originalBranch))
    output, err := cmd.Output()
    if err != nil {
        return 0, err
    }
    count, _ := strconv.Atoi(strings.TrimSpace(string(output)))
    return count, nil
}
```

### Definition of Done

- [ ] Divergence detection implemented
- [ ] Warning message shows commit count
- [ ] `--allow-merge-commit` flag implemented
- [ ] Merge prompt updated to allow non-ff when flag set
- [ ] Tests for divergence scenarios
- [ ] All existing tests pass

**Effort Estimate**: M

---

## Implementation Order

1. **Story 2** (Detached HEAD) - Prevents broken state creation
2. **Story 1** (Worktree validation) - Better resume experience
3. **Story 3** (Branch validation) - Prevents merge failures
4. **Story 4** (Divergence detection) - Advanced merge handling

## Verification

```bash
# Test detached HEAD
git checkout --detach HEAD
orbital --worktree spec.md  # Should fail with clear message

# Test missing worktree
rm -rf .orbital/worktrees/swift-falcon
orbital continue  # Should fail with clear message

# Test missing branch
git branch -D main
orbital continue  # After worktree work, should show alternatives
```

---

## Dependencies

- Story 1 depends on worktree state management (state.go)
- Story 3/4 run during merge phase after main work is complete

## Risks

- Divergence detection adds complexity to merge prompt
- Need to ensure `--allow-merge-commit` works correctly with Claude
