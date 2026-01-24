# Notes: Worktree Isolation Stories

## 2026-01-24

### Iteration 1: Add failing test for --worktree flag

**Test added**: `cmd/orbit-cli/worktree_test.go`

**What it tests**:
- `TestWorktreeFlag_Exists`: Verifies the `--worktree` flag is registered on rootCmd
- `TestWorktreeFlag_IsBoolType`: Verifies the flag is a boolean type
- `TestWorktreeFlag_DefaultsToFalse`: Verifies the flag defaults to false

**Test result**: FAIL (as expected)
```
--- FAIL: TestWorktreeFlag_Exists (0.00s)
    worktree_test.go:11: --worktree flag is not registered on rootCmd
```

The test fails because the `--worktree` flag has not been implemented yet. This is the first acceptance criterion for User Story 1.
