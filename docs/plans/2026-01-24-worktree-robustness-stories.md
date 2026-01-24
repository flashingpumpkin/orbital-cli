# Worktree Robustness: Adversarial Analysis and User Stories

**Date**: 2026-01-24
**Status**: Draft
**Author**: Adversarial Review

## Executive Summary

This document contains user stories derived from an adversarial analysis of the worktree feature. The analysis focused on identifying failure modes, race conditions, edge cases, and ways the feature can break or be broken.

---

## Critical Issues (Data Loss / Silent Failures)

### Story 1: State File Race Condition Causes Data Loss

**As a** developer running multiple orbital instances in parallel
**I want** concurrent worktree state updates to be atomic
**So that** I don't lose worktree tracking information

**Problem**: The state file (`worktree-state.json`) uses a Load → Modify → Save pattern without any file locking. Two parallel processes can race:

```
Process A: Load() → state has [wt1]
Process B: Load() → state has [wt1]
Process A: Add(wt2) → Save([wt1, wt2])
Process B: Add(wt3) → Save([wt1, wt3])  ← wt2 is LOST!
```

**Location**: `internal/worktree/state.go:84-97` (Add method)

**Acceptance Criteria**:
- [ ] Implement file-based locking (e.g., `worktree-state.json.lock`)
- [ ] Use advisory locking with timeout and retry
- [ ] Alternatively: use atomic write with temp file + rename
- [ ] Add test for concurrent Add() operations

**Severity**: CRITICAL
**Impact**: Silent data loss of worktree tracking

---

### Story 2: State File Corruption Recovery

**As a** developer whose state file got corrupted
**I want** orbital to recover gracefully
**So that** I can continue working without manual intervention

**Problem**: If `worktree-state.json` contains invalid JSON, `Load()` returns an error and all operations fail:

```go
// state.go:57-59
if err := json.Unmarshal(data, &state); err != nil {
    return nil, fmt.Errorf("failed to parse state file: %w", err)
}
```

This blocks `orbital continue`, `orbital status`, and all worktree operations.

**Acceptance Criteria**:
- [ ] Create backup before modifying state file (`.json.bak`)
- [ ] On parse error, attempt to restore from backup
- [ ] If no backup, scan filesystem for actual worktrees and rebuild state
- [ ] Add `orbital worktree repair` command for manual recovery
- [ ] Log warning when recovering from corruption

**Severity**: HIGH
**Impact**: Complete feature failure requiring manual JSON editing

---

### Story 3: Path Inconsistency Between Setup Functions

**As a** developer using worktree mode
**I want** consistent absolute paths throughout the worktree lifecycle
**So that** file operations work correctly regardless of working directory

**Problem**: There's an inconsistency in path handling:

- `SetupDirect()` (setup.go:98-101) converts to absolute path
- `runWorktreeSetup()` (root.go:1157-1159) returns **relative** path from `WorktreePath(name)`

```go
// root.go:1157-1159 - RELATIVE PATH!
return &worktree.SetupResult{
    WorktreePath: worktree.WorktreePath(name),  // Returns ".orbital/worktrees/<name>"
    ...
}
```

But then root.go:295 expects absolute path:
```go
cfg.WorkingDir = setupResult.WorktreePath  // If relative, breaks when cwd changes
```

**Acceptance Criteria**:
- [ ] Ensure `runWorktreeSetup()` returns absolute path (like `SetupDirect()` does)
- [ ] Add test that verifies path is absolute
- [ ] Consider removing `runWorktreeSetup()` and using `SetupDirect()` directly

**Severity**: HIGH
**Impact**: File operations fail silently when working directory differs

---

## High Priority Issues (Feature Failures)

### Story 4: Validate Worktree Exists Before Resume

**As a** developer resuming a worktree session
**I want** orbital to verify the worktree still exists
**So that** I get a clear error instead of cryptic failures

**Problem**: `continue.go:78-83` uses the worktree path from state without checking if it exists:

```go
if wtState != nil {
    files = wtState.SpecFiles
    effectiveWorkingDir = wtState.Path  // Never validated!
    fmt.Printf("Resuming in worktree: %s\n\n", effectiveWorkingDir)
}
```

If the worktree was manually deleted, the error will be confusing (e.g., "file not found" deep in execution).

**Acceptance Criteria**:
- [ ] Check if worktree path exists before using it
- [ ] Verify it's actually a git worktree (check `.git` file)
- [ ] If missing, offer to clean up state and continue in main repo
- [ ] Add `orbital worktree list` command to show tracked vs actual worktrees

**Severity**: HIGH
**Impact**: Confusing errors when resuming deleted worktree

---

### Story 5: Handle Detached HEAD State

**As a** developer on a detached HEAD
**I want** orbital to refuse worktree mode with a clear error
**So that** I don't end up in an unrecoverable state

**Problem**: `GetCurrentBranch()` returns "HEAD" for detached state:

```go
// setup.go:35-42
cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
// Returns "HEAD" when detached
```

The merge phase then tries to checkout "HEAD" as a branch name, which will fail confusingly.

**Acceptance Criteria**:
- [ ] Detect detached HEAD state before creating worktree
- [ ] Return clear error: "worktree mode requires a branch, currently on detached HEAD"
- [ ] Suggest: "checkout a branch first: git checkout -b <branch-name>"

**Severity**: HIGH
**Impact**: Merge phase fails with unclear error

---

### Story 6: Original Branch Moved During Worktree Execution

**As a** developer whose main branch advanced while working in a worktree
**I want** the merge phase to handle this gracefully
**So that** my changes aren't lost or applied incorrectly

**Problem**: The merge prompt instructs Claude to:
1. Rebase onto original branch
2. Fast-forward merge (`--ff-only`)

If the original branch has new commits, the fast-forward will fail. The current prompt says "only use fast-forward merge" but doesn't handle the failure case well.

**Acceptance Criteria**:
- [ ] Before merge, check if original branch has diverged
- [ ] If diverged, warn user and offer options:
  - Force rebase + regular merge (creates merge commit)
  - Preserve worktree for manual merge
  - Abort and keep worktree
- [ ] Update merge prompt to handle non-fast-forward scenarios
- [ ] Add `--allow-merge-commit` flag for explicit opt-in

**Severity**: HIGH
**Impact**: Merge fails, changes stuck in orphan worktree

---

### Story 7: Original Branch Deleted During Worktree Execution

**As a** developer whose original branch was deleted while working
**I want** orbital to detect this before merge
**So that** I can choose an alternative target branch

**Problem**: No validation that `OriginalBranch` still exists before merge phase.

**Acceptance Criteria**:
- [ ] Before merge, verify original branch exists: `git rev-parse --verify <branch>`
- [ ] If missing, list available branches and prompt for target
- [ ] Store backup of original branch name in case of recovery
- [ ] Add `--merge-target` flag to override target branch

**Severity**: HIGH
**Impact**: Merge fails with cryptic git error

---

### Story 8: Merge Success Marker Case Sensitivity

**As a** developer
**I want** merge success detection to be robust
**So that** valid merges aren't incorrectly marked as failures

**Problem**: The marker check is case-sensitive and whitespace-sensitive:

```go
// merge.go:60-61
const marker = "MERGE_SUCCESS: true"
return strings.Contains(text, marker)
```

Claude outputting `MERGE_SUCCESS:true` (no space), `MERGE_SUCCESS: True`, or `merge_success: true` will fail silently.

**Acceptance Criteria**:
- [ ] Use case-insensitive matching
- [ ] Allow flexible whitespace: `MERGE_SUCCESS:\s*true`
- [ ] Consider also accepting `MERGE SUCCESS: true` (no underscore)
- [ ] Add test cases for variations

**Severity**: MEDIUM
**Impact**: Valid merges marked as failed, worktree preserved unnecessarily

---

## Medium Priority Issues (UX / Reliability)

### Story 9: Multiple Worktrees - Selection on Resume

**As a** developer with multiple active worktrees
**I want** to choose which worktree to resume
**So that** I can work on multiple features in parallel

**Problem**: `continue.go:66-69` only uses the first worktree:

```go
if err == nil && len(worktrees) > 0 {
    wt := worktrees[0]  // Always first!
    wtState = &wt
}
```

**Acceptance Criteria**:
- [ ] If multiple worktrees exist, list them with numbers
- [ ] Prompt user to select (or use `--worktree-name` flag)
- [ ] Sort by `CreatedAt` descending (most recent first)
- [ ] Show branch name and spec files for each option

**Severity**: MEDIUM
**Impact**: Can only work on one worktree at a time

---

### Story 10: Cleanup Failure Accumulates Orphan Worktrees

**As a** developer
**I want** failed cleanups to be retried or reported clearly
**So that** orphan worktrees don't accumulate on disk

**Problem**: If cleanup fails partway, the worktree or branch may be left behind:

```go
// merge.go:109-113 - worktree removed, but branch delete fails
removeCmd := exec.Command("git", "-C", c.workingDir, "worktree", "remove", ...)
// ... if this succeeds but branch delete fails, worktree is gone but branch remains

// root.go:569-570 - only logs warning
if err := cleanup.Run(...); err != nil {
    fmt.Fprintf(os.Stderr, "Warning: failed to cleanup worktree: %v\n", err)
}
```

**Acceptance Criteria**:
- [ ] Add `orbital worktree cleanup` command to find and remove orphans
- [ ] Compare state file with actual git worktrees: `git worktree list`
- [ ] Offer to remove orphan branches matching `orbital/*` pattern
- [ ] Add `--force` flag to remove without confirmation
- [ ] Log cleanup failures to a file for later review

**Severity**: MEDIUM
**Impact**: Disk space waste, git clutter

---

### Story 11: Worktree with Uncommitted Changes Blocks Cleanup

**As a** developer who forgot to commit changes
**I want** cleanup to warn me about uncommitted work
**So that** I don't accidentally lose changes

**Problem**: `git worktree remove --force` will still fail if there are uncommitted changes. The current error message doesn't explain why.

**Acceptance Criteria**:
- [ ] Before cleanup, check for uncommitted changes: `git -C <worktree> status --porcelain`
- [ ] If changes exist, show them and prompt for confirmation
- [ ] Offer options: commit, stash, or force-remove
- [ ] Add `--discard-changes` flag for scripts

**Severity**: MEDIUM
**Impact**: Cleanup fails with unclear error

---

### Story 12: Parallel Worktree Setup Race Condition

**As a** developer running multiple `orbital --worktree` commands simultaneously
**I want** each to get a unique name
**So that** worktree creation doesn't fail

**Problem**: Name generation checks state file, but another process could create a worktree between check and creation:

```
Process A: GenerateUniqueName() → "swift-falcon"
Process B: GenerateUniqueName() → "swift-falcon"
Process A: CreateWorktree("swift-falcon") → success
Process B: CreateWorktree("swift-falcon") → FAILS (already exists)
```

**Acceptance Criteria**:
- [ ] Use file lock during name generation + creation
- [ ] Or: catch "already exists" error and retry with new name
- [ ] Add unique suffix (timestamp or random) if collision detected
- [ ] Test with concurrent setup attempts

**Severity**: MEDIUM
**Impact**: Worktree creation fails in parallel scenarios

---

### Story 13: Atomic State File Writes

**As a** developer
**I want** state file writes to be atomic
**So that** power failures don't corrupt my state

**Problem**: `os.WriteFile` is not atomic - power failure mid-write corrupts the file:

```go
// state.go:76
if err := os.WriteFile(m.StatePath(), data, 0644); err != nil {
```

**Acceptance Criteria**:
- [ ] Write to temp file first (same directory for same filesystem)
- [ ] Sync to disk: `file.Sync()`
- [ ] Rename temp file to target (atomic on POSIX)
- [ ] On Windows, may need retry logic

**Severity**: MEDIUM
**Impact**: State corruption on power failure

---

### Story 14: Worktree Status Command

**As a** developer
**I want** to see the status of all worktrees
**So that** I can understand what's active and clean up stale entries

**Problem**: No way to inspect worktree state without reading JSON manually.

**Acceptance Criteria**:
- [ ] Add `orbital worktree list` command
- [ ] Show: name, path, branch, original branch, created date, spec files
- [ ] Indicate if path exists on disk
- [ ] Indicate if git worktree is registered: `git worktree list`
- [ ] Show mismatch between state file and filesystem

**Severity**: MEDIUM
**Impact**: Poor observability

---

## Lower Priority Issues (Edge Cases)

### Story 15: Handle Git Submodules

**As a** developer working in a repository with submodules
**I want** worktree mode to either work correctly or refuse clearly
**So that** I don't end up with broken submodule state

**Problem**: Git worktrees don't automatically initialize submodules. The worktree will have empty submodule directories.

**Acceptance Criteria**:
- [ ] Detect if repo has submodules: `.gitmodules` exists
- [ ] Warn user about submodule limitations
- [ ] Optionally run `git submodule update --init` in new worktree
- [ ] Add `--init-submodules` flag

**Severity**: LOW
**Impact**: Submodules missing in worktree (build failures)

---

### Story 16: Handle Bare Repositories

**As a** developer accidentally running orbital in a bare repo
**I want** a clear error message
**So that** I understand why it's failing

**Problem**: `CheckGitRepository()` passes for bare repos, but worktree creation will fail.

**Acceptance Criteria**:
- [ ] Detect bare repo: `git rev-parse --is-bare-repository`
- [ ] Return clear error: "worktree mode not supported in bare repositories"

**Severity**: LOW
**Impact**: Confusing error for edge case

---

### Story 17: Merge Phase Timeout

**As a** developer
**I want** the merge phase to have a timeout
**So that** it doesn't hang indefinitely on complex merges

**Problem**: Merge phase invokes Claude with no timeout:

```go
// root.go:1185-1192
merge := worktree.NewMerge(adapter)
return merge.Run(ctx, mergeOpts)  // No explicit timeout
```

The context may have a timeout from the parent, but it's not explicit.

**Acceptance Criteria**:
- [ ] Add explicit timeout for merge phase (e.g., 10 minutes default)
- [ ] Add `--merge-timeout` flag
- [ ] On timeout, preserve worktree and inform user

**Severity**: LOW
**Impact**: Merge can hang on complex conflicts

---

### Story 18: Validate Worktree Name Characters

**As a** developer providing a custom worktree name
**I want** validation before any git operations
**So that** I get a clear error early

**Problem**: `ValidateWorktreeName()` is called inside `CreateWorktree()`, but if the user provides `--worktree-name` with invalid characters, the error appears after setup has started.

**Acceptance Criteria**:
- [ ] Validate `--worktree-name` flag in `runOrbit()` before any setup
- [ ] Provide clear examples of valid names
- [ ] Suggest auto-correction for common mistakes (uppercase → lowercase)

**Severity**: LOW
**Impact**: Poor error UX

---

### Story 19: Paths with Spaces or Special Characters

**As a** developer with spaces in my path
**I want** worktree mode to work correctly
**So that** I can use orbital in any directory

**Problem**: Some git commands in the codebase might not properly quote paths. Need to audit and test.

**Acceptance Criteria**:
- [ ] Test worktree creation in directory with spaces
- [ ] Test with unicode characters in path
- [ ] Test with very long paths (near OS limit)
- [ ] Ensure all git commands use `-C` flag consistently

**Severity**: LOW
**Impact**: Failures in non-standard path configurations

---

### Story 20: Signal Handling During Worktree Creation

**As a** developer who hits Ctrl+C during worktree setup
**I want** partial state to be cleaned up
**So that** I don't have orphan worktrees

**Problem**: If SIGINT arrives between worktree creation and state persistence, we have an orphan worktree not tracked in state.

```go
// root.go:257-292 - gap between creation and state save
setupResult, err := runWorktreeSetup(...)  // Worktree created
// ... SIGINT could arrive here ...
if err := wtStateManager.Add(*wtState); err != nil {  // State not saved!
```

**Acceptance Criteria**:
- [ ] Register cleanup handler before worktree creation
- [ ] On SIGINT during setup, remove the just-created worktree
- [ ] Or: save state immediately after creation, mark as "incomplete"
- [ ] Complete state on successful setup start

**Severity**: LOW
**Impact**: Orphan worktrees on interrupted setup

---

## Testing Stories

### Story 21: Integration Tests for Worktree Lifecycle

**As a** maintainer
**I want** integration tests for the full worktree lifecycle
**So that** regressions are caught early

**Problem**: Setup tests note (setup_test.go:65):
> Integration tests for SetupDirect would require a real git repository. These are deferred.

**Acceptance Criteria**:
- [ ] Create test fixtures with real git repos (temp directories)
- [ ] Test: setup → iteration → merge → cleanup cycle
- [ ] Test: setup → interrupt → continue → complete cycle
- [ ] Test: parallel setup collision handling
- [ ] Test: cleanup with uncommitted changes
- [ ] Test: merge with diverged branches

**Severity**: HIGH (for test coverage)
**Impact**: Regression detection

---

### Story 22: Fuzz Testing for State File Parsing

**As a** maintainer
**I want** fuzz testing for state file parsing
**So that** malformed input doesn't cause panics

**Acceptance Criteria**:
- [ ] Add fuzz test for `StateFile` JSON unmarshaling
- [ ] Test with truncated JSON, invalid UTF-8, very large files
- [ ] Ensure all errors are handled gracefully (no panics)

**Severity**: LOW
**Impact**: Robustness

---

## Implementation Priority

### Phase 1: Critical Fixes
1. Story 3: Path inconsistency (blocks correct operation)
2. Story 1: State file race condition (data loss)
3. Story 2: State file corruption recovery (complete failure)

### Phase 2: High Priority Fixes
4. Story 4: Validate worktree exists on resume
5. Story 5: Handle detached HEAD
6. Story 6: Handle diverged original branch
7. Story 8: Case-insensitive marker matching

### Phase 3: UX Improvements
8. Story 14: Worktree status command
9. Story 9: Multiple worktree selection
10. Story 10: Cleanup command for orphans
11. Story 11: Uncommitted changes warning

### Phase 4: Edge Cases & Polish
12. Story 12: Parallel setup race condition
13. Story 13: Atomic state writes
14. Story 7: Deleted original branch
15. Remaining stories

---

## Appendix: Code Locations

| Component | File | Key Functions |
|-----------|------|---------------|
| Setup | `internal/worktree/setup.go` | `SetupDirect()` |
| Merge | `internal/worktree/merge.go` | `Merge.Run()`, `Cleanup.Run()` |
| State | `internal/worktree/state.go` | `Add()`, `Remove()`, `Load()`, `Save()` |
| Git | `internal/worktree/git.go` | `CreateWorktree()`, `RemoveWorktree()`, `DeleteBranch()` |
| Names | `internal/worktree/names.go` | `GenerateUniqueName()` |
| CLI | `cmd/orbital/root.go` | `runWorktreeSetup()`, merge/cleanup orchestration |
| Resume | `cmd/orbital/continue.go` | Resume logic with worktree detection |
