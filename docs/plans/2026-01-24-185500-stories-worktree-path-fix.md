# User Stories: Worktree Path Consistency Fix

## Problem Statement

There is a critical inconsistency in path handling between two setup functions:

- `SetupDirect()` in `setup.go:98-101` converts to absolute path
- `runWorktreeSetup()` in `root.go:1157-1159` returns **relative** path

This causes silent failures when the working directory changes during execution.

**Affected code:**
- `cmd/orbital/root.go:1157-1159` - Returns relative path
- `internal/worktree/setup.go:98-104` - Converts to absolute path (correct)
- `cmd/orbital/root.go:295` - Uses path as WorkingDir

---

## User Story 1: Fix Path Inconsistency in runWorktreeSetup

**As a** developer using worktree mode
**I want** consistent absolute paths throughout the worktree lifecycle
**So that** file operations work correctly regardless of working directory changes

### Context

The current `runWorktreeSetup()` function returns a relative path:

```go
// root.go:1157-1159 - RELATIVE PATH!
return &worktree.SetupResult{
    WorktreePath: worktree.WorktreePath(name),  // Returns ".orbital/worktrees/<name>"
    ...
}
```

But `root.go:295` uses this as `cfg.WorkingDir`:
```go
cfg.WorkingDir = setupResult.WorktreePath  // If relative, breaks when cwd changes
```

This can cause failures if:
1. Any code changes the current working directory
2. Spec files are resolved relative to the worktree path
3. State persistence uses the path as a key

### Acceptance Criteria

- [ ] Given `runWorktreeSetup()` is called, when it returns, then `WorktreePath` is an absolute path
- [ ] Given worktree is created, when path is persisted to state, then it is absolute
- [ ] Given worktree path is used for WorkingDir, when Claude runs, then it works regardless of Orbital's cwd
- [ ] Given spec files are stored, when they reference the worktree path, then paths are consistent

### Technical Notes

```go
// cmd/orbital/root.go

func runWorktreeSetup(ctx context.Context, specContent string, opts worktreeSetupOptions) (*worktree.SetupResult, error) {
    // ... existing validation and name generation ...

    // Create the worktree using direct git commands
    if err := worktree.CreateWorktree(opts.workingDir, name); err != nil {
        return nil, fmt.Errorf("failed to create worktree: %w", err)
    }

    // CRITICAL: Convert to absolute path immediately
    relPath := worktree.WorktreePath(name)
    absPath, err := filepath.Abs(filepath.Join(opts.workingDir, relPath))
    if err != nil {
        // Cleanup the worktree we just created
        worktree.RemoveWorktree(opts.workingDir, relPath)
        return nil, fmt.Errorf("failed to get absolute worktree path: %w", err)
    }

    return &worktree.SetupResult{
        WorktreePath: absPath,  // NOW ABSOLUTE!
        BranchName:   worktree.BranchName(name),
        CostUSD:      0,
        TokensIn:     0,
        TokensOut:    0,
    }, nil
}
```

### Definition of Done

- [ ] `runWorktreeSetup()` returns absolute path
- [ ] Path conversion happens immediately after worktree creation
- [ ] If path conversion fails, worktree is cleaned up
- [ ] Test verifies path is absolute
- [ ] All existing tests pass
- [ ] Manual test: worktree mode works correctly

**Effort Estimate**: XS

---

## User Story 2: Add Path Validation Throughout Worktree Code

**As a** maintainer
**I want** path validation assertions in worktree code
**So that** relative path bugs are caught early in development

### Context

The path inconsistency was subtle and hard to detect. Adding validation ensures future changes don't reintroduce the bug.

### Acceptance Criteria

- [ ] Given a path is stored in WorktreeState, when Add() is called, then path must be absolute
- [ ] Given a path is used for WorkingDir, when config is validated, then path must be absolute (or ".")
- [ ] Given validation fails, when error is returned, then it clearly states "path must be absolute"
- [ ] Given tests use relative paths incorrectly, when tests run, then they fail with clear message

### Technical Notes

```go
// internal/worktree/state.go

func (m *StateManager) Add(wt WorktreeState) error {
    // Validate path is absolute
    if wt.Path != "" && !filepath.IsAbs(wt.Path) {
        return fmt.Errorf("worktree path must be absolute, got: %s", wt.Path)
    }

    // ... existing logic ...
}

// internal/config/config.go

func (c *Config) Validate() error {
    // ... existing validation ...

    // WorkingDir must be "." or absolute
    if c.WorkingDir != "." && c.WorkingDir != "" && !filepath.IsAbs(c.WorkingDir) {
        return fmt.Errorf("working directory must be absolute path or \".\", got: %s", c.WorkingDir)
    }

    return nil
}
```

### Definition of Done

- [ ] Path validation added to `StateManager.Add()`
- [ ] Path validation added to `Config.Validate()`
- [ ] Validation errors are clear and actionable
- [ ] Existing tests updated if they use relative paths incorrectly
- [ ] All tests pass

**Effort Estimate**: XS

---

## User Story 3: Consolidate Setup Functions

**As a** maintainer
**I want** a single setup function used consistently
**So that** there's no risk of using the wrong one

### Context

There are two setup paths:
- `SetupDirect()` in `setup.go` - Uses Claude (deprecated)
- `runWorktreeSetup()` in `root.go` - Uses local name generation

Having two functions increases the risk of using the wrong one or having divergent behavior.

### Acceptance Criteria

- [ ] Given worktree setup is needed, when code is reviewed, then only one function is used
- [ ] Given `SetupDirect()` is kept, when it's called, then it delegates to common implementation
- [ ] Given deprecated code paths exist, when they're identified, then they're removed
- [ ] Given setup logic changes, when change is made, then it affects all call sites

### Technical Notes

Option A: Remove `SetupDirect()` entirely if unused
```go
// Delete internal/worktree/setup.go:SetupDirect() if not called
// Keep only git helpers and state management
```

Option B: Have `SetupDirect()` delegate to common logic
```go
// internal/worktree/setup.go

func SetupDirect(dir, name string) (*SetupResult, error) {
    if err := CreateWorktree(dir, name); err != nil {
        return nil, err
    }

    absPath, err := filepath.Abs(filepath.Join(dir, WorktreePath(name)))
    if err != nil {
        RemoveWorktree(dir, WorktreePath(name))
        return nil, fmt.Errorf("failed to get absolute path: %w", err)
    }

    return &SetupResult{
        WorktreePath: absPath,
        BranchName:   BranchName(name),
    }, nil
}
```

### Definition of Done

- [ ] Single authoritative setup implementation
- [ ] No duplicate path conversion logic
- [ ] Dead code removed
- [ ] Tests updated for consolidated function
- [ ] All tests pass

**Effort Estimate**: S

---

## Implementation Order

1. **Story 1** (Fix path inconsistency) - Critical bug fix
2. **Story 2** (Add validation) - Prevent regression
3. **Story 3** (Consolidate functions) - Cleanup

## Verification

```bash
# Test that path is absolute
orbital --worktree spec.md

# Check state file
cat .orbital/worktree-state.json | jq '.worktrees[0].path'
# Should show absolute path like "/home/user/project/.orbital/worktrees/swift-falcon"
# NOT relative like ".orbital/worktrees/swift-falcon"

# Test with different cwd
cd /tmp
orbital continue --working-dir /home/user/project
# Should work correctly
```

---

## Dependencies

- None (isolated fix)

## Risks

- Existing state files may have relative paths (handle gracefully on load)
- Tests may assume relative paths (update tests)

## Migration

For existing state files with relative paths:

```go
func (m *StateManager) Load() (*StateFile, error) {
    // ... existing load logic ...

    // Migrate relative paths to absolute
    for i := range state.Worktrees {
        if state.Worktrees[i].Path != "" && !filepath.IsAbs(state.Worktrees[i].Path) {
            absPath, err := filepath.Abs(filepath.Join(m.workingDir, state.Worktrees[i].Path))
            if err == nil {
                state.Worktrees[i].Path = absPath
            }
        }
    }

    return &state, nil
}
```
