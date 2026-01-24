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

### Iteration 3: Refactor worktree tests

**Refactoring performed**:
- Consolidated three separate test functions into a single `TestWorktreeFlag` function
- Flag lookup now happens once at the top, eliminating repetition
- Used subtests (`t.Run`) for the individual property checks
- Changed first check from `t.Error` to `t.Fatal` since subsequent subtests depend on the flag existing

**Before**: 3 separate functions with repeated `rootCmd.PersistentFlags().Lookup("worktree")` calls
**After**: 1 function with subtests, single flag lookup

**Test result**: PASS
```
=== RUN   TestWorktreeFlag
=== RUN   TestWorktreeFlag/is_bool_type
=== RUN   TestWorktreeFlag/defaults_to_false
--- PASS: TestWorktreeFlag (0.00s)
```

### Iteration 4: TDD Review Gate

**Review scope**: Test quality, implementation correctness, refactoring quality

**Test quality assessment**:
- Tests are focused and test a single concern (the flag registration)
- Subtest structure is well organised: parent test checks flag existence, subtests check properties
- Using `t.Fatal` correctly for the prerequisite check (flag must exist before checking its properties)
- Test assertions are clear and use descriptive error messages with actual vs expected format
- No external dependencies or mocking needed for this simple flag test

**Implementation correctness**:
- Flag declared at package level with other flag variables (line 52: `worktree bool`)
- Flag registered in `init()` using `BoolVar` with correct parameters (line 111)
- Help text is clear: "Enable worktree isolation mode"
- Default value of `false` is appropriate (opt-in behaviour)
- Flag is currently unused in `runOrbit` but that's expected; subsequent stories will wire it up

**Refactoring quality**:
- Consolidation from 3 separate test functions to 1 with subtests reduces duplication
- Single `Lookup` call at the top eliminates repeated flag lookups
- Subtest names are lowercase and descriptive
- Code is idiomatic Go testing style

**Findings**: No issues found. The TDD cycle was executed correctly:
1. Red: Test written first, failed as expected
2. Green: Minimal implementation to pass tests
3. Refactor: Tests consolidated without changing behaviour

**Action items**: None required

### Iteration 5: Add failing test for setup phase

**Test added**: `internal/worktree/setup_test.go`

**New package created**: `internal/worktree/`

**What the tests verify**:
- `TestSetupPhase_InvokesClaude`: Verifies that the setup phase invokes Claude with a prompt containing the spec content, and extracts the worktree path from Claude's output (expected format: `WORKTREE_PATH: <path>`)
- `TestSetupPhase_ReturnsErrorOnExecutionFailure`: Verifies setup returns an error when the executor fails
- `TestSetupPhase_ReturnsErrorWhenPathNotFound`: Verifies setup returns an error when Claude's output doesn't contain the `WORKTREE_PATH` marker

**Types and functions expected** (not yet implemented):
- `ExecutionResult` struct with `Output`, `CostUSD`, `TokensUsed` fields
- `NewSetup(executor)` constructor
- `Setup.Run(ctx, specContent)` method returning `(*SetupResult, error)`
- `SetupResult` struct with `WorktreePath` field

**Test result**: FAIL (as expected, build fails because types don't exist)
```
internal/worktree/setup_test.go:13:17: undefined: ExecutionResult
internal/worktree/setup_test.go:36:11: undefined: NewSetup
...
FAIL	github.com/flashingpumpkin/orbit-cli/internal/worktree [build failed]
```

This tests the acceptance criterion: "When flag is set, orbit runs a separate Claude invocation before the loop"