# User Stories: Worktree Cleanup Improvements

## Problem Statement

Worktree cleanup can fail in several ways, leaving orphaned worktrees and branches that accumulate over time:

1. **Partial cleanup**: Worktree removed but branch deletion fails
2. **Uncommitted changes**: Cleanup fails with unclear error
3. **No cleanup command**: No way to find and remove orphans
4. **Interrupted setup**: SIGINT during setup leaves partial state

**Affected code:**
- `internal/worktree/merge.go` - Cleanup.Run()
- `cmd/orbital/root.go:567-578` - Cleanup orchestration
- `internal/worktree/git.go` - RemoveWorktree, DeleteBranch

---

## User Story 1: Add Worktree Cleanup Command

**As a** developer
**I want** a command to find and remove orphan worktrees
**So that** I can clean up stale entries that accumulate over time

### Context

If cleanup fails partway, the worktree or branch may be left behind. There's no way to identify and clean these up except manually.

### Acceptance Criteria

- [ ] Given orphan worktrees exist, when `orbital worktree cleanup` runs, then they are listed
- [ ] Given state file has entries for non-existent worktrees, when cleanup runs, then entries are removed
- [ ] Given git has worktrees not in state file, when cleanup runs, then they are listed as orphans
- [ ] Given `--dry-run` flag is set, when cleanup runs, then actions are shown but not executed
- [ ] Given `--force` flag is set, when cleanup runs, then no confirmation is asked
- [ ] Given branches matching `orbital/*` exist without worktrees, when cleanup runs, then they are listed

### Technical Notes

```go
// cmd/orbital/cleanup.go

var cleanupCmd = &cobra.Command{
    Use:   "cleanup",
    Short: "Clean up orphaned worktrees and branches",
    RunE:  runCleanup,
}

func runCleanup(cmd *cobra.Command, args []string) error {
    // 1. Load state file
    sm := worktree.NewStateManager(workingDir)
    state, err := sm.Load()

    // 2. Get actual worktrees from git
    gitWorktrees := getGitWorktrees(workingDir)

    // 3. Find orphans:
    //    - State entries where path doesn't exist
    //    - Git worktrees not in state
    //    - Branches matching orbital/* without worktrees

    // 4. Display findings and prompt for cleanup

    // 5. Remove orphans (or dry-run)
}

func getGitWorktrees(dir string) []string {
    cmd := exec.Command("git", "-C", dir, "worktree", "list", "--porcelain")
    // Parse output for worktree paths
}
```

### Definition of Done

- [ ] `orbital worktree cleanup` command implemented
- [ ] Detects state file orphans
- [ ] Detects git worktree orphans
- [ ] Detects orphan branches
- [ ] `--dry-run` flag shows what would be cleaned
- [ ] `--force` flag skips confirmation
- [ ] Tests for orphan detection
- [ ] All existing tests pass

**Effort Estimate**: M

---

## User Story 2: Warn About Uncommitted Changes Before Cleanup

**As a** developer who forgot to commit changes
**I want** cleanup to warn me about uncommitted work
**So that** I don't accidentally lose changes

### Context

`git worktree remove --force` will fail if there are uncommitted changes. The current error message doesn't explain why.

### Acceptance Criteria

- [ ] Given worktree has uncommitted changes, when cleanup starts, then changes are listed
- [ ] Given changes are found, when user is prompted, then options are: commit, stash, discard, abort
- [ ] Given `--discard-changes` flag is set, when cleanup runs, then changes are discarded without prompt
- [ ] Given no uncommitted changes, when cleanup runs, then it proceeds normally
- [ ] Given user chooses abort, when cleanup stops, then worktree is preserved

### Technical Notes

```go
// internal/worktree/merge.go

func (c *Cleanup) checkUncommittedChanges(worktreePath string) (bool, string, error) {
    cmd := exec.Command("git", "-C", worktreePath, "status", "--porcelain")
    output, err := cmd.Output()
    if err != nil {
        return false, "", fmt.Errorf("failed to check git status: %w", err)
    }
    status := strings.TrimSpace(string(output))
    return status != "", status, nil
}

func (c *Cleanup) Run(worktreePath, branchName string) error {
    // Check for uncommitted changes first
    hasChanges, status, err := c.checkUncommittedChanges(worktreePath)
    if err != nil {
        return err
    }
    if hasChanges && !c.discardChanges {
        return fmt.Errorf("worktree has uncommitted changes:\n%s\nUse --discard-changes to force cleanup", status)
    }

    // Proceed with cleanup...
}
```

### Definition of Done

- [ ] Uncommitted change detection implemented
- [ ] Changes displayed before cleanup
- [ ] `--discard-changes` flag implemented
- [ ] Clear error message with status output
- [ ] Tests for uncommitted change scenarios
- [ ] All existing tests pass

**Effort Estimate**: S

---

## User Story 3: Atomic Cleanup with Rollback

**As a** developer
**I want** cleanup to be all-or-nothing
**So that** partial cleanup doesn't leave inconsistent state

### Context

Current cleanup does: remove worktree -> delete branch -> remove state. If step 2 fails, we have removed the worktree but kept the branch and state entry.

### Acceptance Criteria

- [ ] Given worktree removal succeeds but branch deletion fails, when cleanup errors, then state entry is kept
- [ ] Given cleanup fails, when user reruns cleanup, then it handles the partial state
- [ ] Given all steps succeed, when cleanup completes, then state is consistent
- [ ] Given branch deletion fails, when error is shown, then manual cleanup instructions are provided

### Technical Notes

```go
func (c *Cleanup) Run(worktreePath, branchName string) error {
    // Track what we've done for error messages
    worktreeRemoved := false

    // Step 1: Remove worktree
    if err := RemoveWorktree(c.workingDir, worktreePath); err != nil {
        return fmt.Errorf("failed to remove worktree: %w", err)
    }
    worktreeRemoved = true

    // Step 2: Delete branch
    if err := DeleteBranch(c.workingDir, branchName); err != nil {
        // Worktree is gone, but branch remains
        return fmt.Errorf("worktree removed but failed to delete branch %q: %w\nManual cleanup: git branch -D %s",
            branchName, err, branchName)
    }

    // Step 3: State removal happens in caller (root.go)
    // If this fails, at least git state is clean

    return nil
}
```

### Definition of Done

- [ ] Cleanup tracks progress for error messages
- [ ] Partial failure gives specific recovery instructions
- [ ] State entry only removed after full git cleanup succeeds
- [ ] Tests for partial cleanup scenarios
- [ ] All existing tests pass

**Effort Estimate**: S

---

## User Story 4: Handle Interrupted Setup

**As a** developer who hits Ctrl+C during worktree setup
**I want** partial state to be cleaned up
**So that** I don't have orphan worktrees

### Context

If SIGINT arrives between worktree creation and state persistence, we have an orphan worktree not tracked in state:

```go
// root.go:257-292 - gap between creation and state save
setupResult, err := runWorktreeSetup(...)  // Worktree created
// ... SIGINT could arrive here ...
if err := wtStateManager.Add(*wtState); err != nil {  // State not saved!
```

### Acceptance Criteria

- [ ] Given SIGINT during setup after worktree creation, when signal is handled, then worktree is removed
- [ ] Given SIGINT before worktree creation, when signal is handled, then no cleanup needed
- [ ] Given setup completes normally, when signal handler is removed, then cleanup handler is deregistered
- [ ] Given cleanup after SIGINT fails, when error occurs, then it's logged but doesn't cause crash

### Technical Notes

```go
// cmd/orbital/root.go

func runWorktreeSetupWithCleanup(ctx context.Context, opts worktreeSetupOptions) (*worktree.SetupResult, error) {
    // Track if we created anything
    var createdWorktreePath string

    // Register cleanup on interrupt
    cleanupDone := make(chan struct{})
    go func() {
        <-ctx.Done()
        if createdWorktreePath != "" {
            // Best-effort cleanup
            worktree.RemoveWorktree(opts.workingDir, createdWorktreePath)
        }
        close(cleanupDone)
    }()

    // Create worktree
    if err := worktree.CreateWorktree(opts.workingDir, name); err != nil {
        return nil, err
    }
    createdWorktreePath = worktree.WorktreePath(name)

    // From here, if interrupted, cleanup will run

    result := &worktree.SetupResult{
        WorktreePath: createdWorktreePath,
        BranchName:   worktree.BranchName(name),
    }

    return result, nil
}
```

### Definition of Done

- [ ] Signal handler registered before worktree creation
- [ ] Cleanup removes worktree on interrupt
- [ ] Cleanup is best-effort (doesn't crash on failure)
- [ ] Handler deregistered on successful completion
- [ ] Tests for interrupt scenarios (may need to use signals)
- [ ] All existing tests pass

**Effort Estimate**: M

---

## Implementation Order

1. **Story 1** (Cleanup command) - Most valuable for users
2. **Story 2** (Uncommitted changes warning) - Prevents data loss
3. **Story 3** (Atomic cleanup) - Better error handling
4. **Story 4** (Interrupted setup) - Edge case handling

## Verification

```bash
# Test cleanup command
orbital worktree cleanup --dry-run

# Test uncommitted changes
cd .orbital/worktrees/swift-falcon
echo "test" > uncommitted.txt
orbital worktree cleanup  # Should warn

# Test partial cleanup
# (Manually break branch deletion and verify recovery)

# Test interrupted setup
orbital --worktree spec.md &
PID=$!
sleep 1
kill -INT $PID
ls .orbital/worktrees/  # Should be empty
```

---

## Dependencies

- Story 1 depends on state management
- Stories 2-4 are independent

## Risks

- Signal handling in Go can be tricky - ensure cleanup doesn't race with normal exit
- `--discard-changes` is dangerous - require double confirmation or `--force`
