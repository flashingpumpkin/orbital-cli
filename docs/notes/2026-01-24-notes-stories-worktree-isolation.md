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

### Iteration 2: Implement --worktree flag

**Implementation**: Added the `--worktree` boolean flag to `cmd/orbit-cli/root.go`

**Changes**:
1. Added `worktree bool` variable to the flag variables block
2. Registered the flag with `rootCmd.PersistentFlags().BoolVar(&worktree, "worktree", false, "Enable worktree isolation mode")`

**Test result**: PASS
```
=== RUN   TestWorktreeFlag_Exists
--- PASS: TestWorktreeFlag_Exists (0.00s)
=== RUN   TestWorktreeFlag_IsBoolType
--- PASS: TestWorktreeFlag_IsBoolType (0.00s)
=== RUN   TestWorktreeFlag_DefaultsToFalse
--- PASS: TestWorktreeFlag_DefaultsToFalse (0.00s)
```

All tests pass. The flag is now registered but not yet wired to any functionality. The next story item will add the setup phase logic.
