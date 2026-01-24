# Notes: Worktree Implementation Fixes

Date: 2026-01-24

## Iteration 1

### Completed Tickets

**Ticket 1.1: Create adjective-animal name generator**
- Created `internal/worktree/names.go` with 50 adjectives and 50 animals
- Implemented `GenerateName()` and `GenerateUniqueName(excluded []string)`
- Uses crypto/rand for secure random selection
- Falls back to numeric suffix when all combinations are exhausted
- Full test coverage in `names_test.go`

**Ticket 1.2: Create git worktree helper functions**
- Created `internal/worktree/git.go` with:
  - `CreateWorktree(dir, name string) error`
  - `RemoveWorktree(dir, worktreePath string) error`
  - `DeleteBranch(dir, branchName string) error`
  - `ValidateBranchName(name string) error`
  - `ValidateWorktreeName(name string) error`
  - Helper functions: `WorktreePath()`, `BranchName()`, `ListWorktreeNames()`
- Error messages include git command output for debugging
- Full test coverage in `git_test.go`

**Ticket 2.1: Set cmd.Dir in executor**
- Modified `internal/executor/executor.go` to set `cmd.Dir = e.config.WorkingDir`
- Only sets when WorkingDir is non-empty and not "."
- Added tests verifying the working directory behaviour

**Ticket 3.1: Update state file format**
- Added `Name string` field to `WorktreeState` struct
- Uses `omitempty` JSON tag for backwards compatibility
- CreatedAt was already present, added `omitempty` to SessionID

**Ticket 3.2: Display worktree name in TUI**
- Added `Name` field to `tui.WorktreeInfo` struct
- Updated `renderWorktreePanel()` to display name prominently
- Falls back to path if no name is available

**Ticket 4.1: Add branch name validation**
- Implemented `ValidateBranchName()` in git.go
- Validates: prefix `orbital/`, no spaces, alphanumeric/hyphen/slash only
- Detects corruption patterns (e.g., "success" appended without hyphen)
- Called from `DeleteBranch()` before git operations

**Ticket 4.2: Improve cleanup error messages**
- Error messages in `DeleteBranch()` and `RemoveWorktree()` now include:
  - The branch/path being operated on
  - Git command output
  - Wrapped errors for unwrapping

### Remaining Tickets

The following tickets require integration with the setup flow and are deferred:

- **Ticket 1.3**: Integrate name generator into setup flow
- **Ticket 1.4**: Remove obsolete setup code
- **Ticket 2.2**: Remove directory navigation from merge prompt

### Technical Decisions

1. **Branch prefix**: Changed from `orbit/` to `orbital/` to match the CLI name
2. **Validation approach**: Added corruption detection for common patterns like "success" appended without hyphen separator
3. **Backwards compatibility**: Used `omitempty` JSON tags to handle state files without the Name field

### Test Results

All tests passing:
```
ok  github.com/flashingpumpkin/orbital/internal/worktree
ok  github.com/flashingpumpkin/orbital/internal/executor
ok  github.com/flashingpumpkin/orbital/internal/tui
```

Build successful.

---

## Code Review: Iteration 1

**Reviewer**: Automated review gate
**Date**: 2026-01-24

### Summary

The commit `84c85f0` ("feat(worktree): Add name generator, git helpers, and TUI improvements") implements tickets 1.1, 1.2, 2.1, 3.1, 3.2, 4.1, and 4.2 from the worktree fixes plan. The changes are well-structured, follow project conventions, and have good test coverage.

### Findings

#### Correctness: PASS

1. **Name generator** (`names.go`): Uses `crypto/rand` for secure randomness, handles edge cases (all combinations excluded), and produces valid adjective-animal format.

2. **Git helpers** (`git.go`): Correctly implements worktree creation, removal, and branch deletion. The fallback from `-d` to `-D` for branch deletion handles unmerged branches correctly.

3. **Branch validation** (`git.go`): The corruption detection pattern is clever - it catches suffixes like "success" that are not hyphen-separated, which was the original bug. The check `!strings.HasSuffix(suffix, "-"+pattern)` correctly allows "fix-success" but rejects "fixsuccess".

4. **Executor working directory** (`executor.go`): Correctly sets `cmd.Dir` only when WorkingDir is non-empty and not ".". This preserves default behaviour.

5. **TUI update** (`model.go`): Falls back gracefully to path when name is not available (backwards compatibility).

#### Edge Cases: PASS

1. `GenerateUniqueName()` handles nil exclusions, empty exclusions, and all combinations excluded.
2. `ValidateBranchName()` handles missing prefix, spaces, invalid characters, and corruption patterns.
3. `ValidateWorktreeName()` handles empty names, uppercase, leading/trailing hyphens.
4. Executor handles empty WorkingDir and "." correctly.

#### Code Quality: PASS

1. Follows Go conventions: exported functions have comments, uses table-driven tests, errors are wrapped with context.
2. Uses `fmt.Errorf("context: %w", err)` pattern consistently.
3. Constants defined for magic strings (`BranchPrefix`, `WorktreeDir`).
4. Tests are comprehensive and well-organized with subtests.

#### Test Coverage: PASS

1. `names_test.go`: Tests format, randomness, exclusion handling, fallback suffix, list validity.
2. `git_test.go`: Tests validation functions with various inputs, helper functions.
3. `executor_test.go`: Tests working directory configuration.
4. Integration tests for git commands correctly deferred (noted in test file).

#### Minor Observations (Non-blocking)

1. The `containsString` helper in `executor_test.go` (lines 381-386) duplicates `strings.Contains`. This is minor and could be cleaned up in a future commit.

2. The corruption patterns list in `ValidateBranchName()` could potentially miss other patterns, but the current list covers the documented failure modes.

### Verdict

**PASS** - No blocking issues. Changes are correct, well-tested, and follow project conventions.

---

## Iteration 2

### Completed Tickets

**Ticket 1.3: Integrate name generator into setup flow**
- Modified `runWorktreeSetup()` in `cmd/orbital/root.go` to generate name locally
- Uses `worktree.GenerateUniqueName()` to generate unique adjective-animal names
- Uses `worktree.CreateWorktree()` to create worktree directly with git commands
- No longer invokes Claude for setup phase (eliminates ~$0.01 per run)
- Preserves `--worktree-name` flag for manual override
- Updated state persistence to include the generated name

**Ticket 1.4: Remove obsolete setup code**
- Removed from `internal/worktree/setup.go`:
  - `buildSetupPrompt()` function
  - `extractWorktreePath()` function
  - `extractBranchName()` function
  - `extractMarker()` function
  - `Setup` struct and `NewSetup()` constructor
- Added `SetupDirect()` function for the new local setup flow
- Updated `setup_test.go` to test the new simplified code
- Kept `worktreeExecutorAdapter` (still used by merge phase)

**Ticket 2.2: Remove directory navigation from merge prompt**
- Simplified `buildMergePrompt()` in `internal/worktree/merge.go`
- Removed "Navigate to the worktree directory" instruction
- Removed "Navigate to the main repository" instruction
- Working directory is now set correctly via `cmd.Dir` in the executor
- Updated `merge_test.go` to verify navigation instructions are removed

### Technical Decisions

1. **SetupDirect function**: Created a new `SetupDirect()` function rather than modifying the existing `Setup.Run()` to make the API clearer and avoid breaking changes.

2. **worktreeExecutorAdapter retention**: Kept the adapter since the merge phase still invokes Claude to handle rebase and merge operations.

3. **Merge prompt simplification**: Removed only the navigation instructions, keeping the git rebase and merge commands since they still need to be executed.

### Test Results

All tests passing:
```
ok  github.com/flashingpumpkin/orbital/cmd/orbital
ok  github.com/flashingpumpkin/orbital/internal/worktree
```

Build successful.
