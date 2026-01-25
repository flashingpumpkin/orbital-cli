# Continuous Improvement Notes

## Iteration 1 - 2026-01-25 22:43

### Task Selected
Extract `intToString()` to shared utility package - duplicated across 4 files.

### Why Highest Leverage
- Duplicated in 4 files (tui/model.go, tui/selector/model.go, tasks/tracker.go, tui/bridge.go)
- Used frequently throughout the codebase
- Low effort, high impact (eliminates ~40 lines of duplicated code)
- Creates foundation for future utility extractions

### Key Decisions
- Creating `internal/util/convert.go` for string conversion utilities
- Will also include `formatNumber()` for thousands separators since it's related

### Observations
- The codebase has several other deduplication opportunities identified:
  - `repeatString()` can be replaced with `strings.Repeat()`
  - Scroll offset calculation repeated 5 times
  - Padding calculation repeated 10+ times
- These are good candidates for future iterations

### Outcome
- Created `internal/util/convert.go` with `IntToString()` and `FormatNumber()` functions
- Created `internal/util/convert_test.go` with comprehensive tests
- Updated 4 source files to use the shared utilities:
  - `internal/tui/model.go`
  - `internal/tui/selector/model.go`
  - `internal/tasks/tracker.go`
  - `internal/tui/bridge.go`
- Updated 3 test files to use the shared utilities:
  - `internal/tui/model_test.go`
  - `internal/tui/selector/model_test.go`
  - `internal/tui/ringbuffer_test.go`
- All tests and linting pass

## Code Review - Iteration 1

### Security
No issues. The utility functions are pure integer-to-string converters with no external input vectors, no injection risks, and no sensitive data handling.

### Design
No issues. The refactoring follows good principles:
- Correct dependency direction (util has no internal dependencies)
- Good package cohesion (related formatting functions grouped together)
- Clear API design with proper documentation
- Reduces coupling by eliminating code duplication

### Logic
**_ISSUES_FOUND_**

1. **Critical: Stack overflow with `math.MinInt`** (`convert.go:13-14`)
   - `IntToString(math.MinInt)` causes infinite recursion because `-math.MinInt == math.MinInt` due to two's complement overflow

2. **High: `FormatNumber` produces malformed output for negative numbers** (`convert.go:32-45`)
   - `FormatNumber(-1234567)` produces `"-,123,456,7"` instead of `"-1,234,567"`
   - The minus sign is incorrectly included in length calculation for comma placement

3. **Medium: Missing test coverage for negative numbers in `FormatNumber`** (`convert_test.go`)

4. **Medium: `formatFloat` loses sign for negative values between -1 and 0** (`bridge.go:417-425`)
   - `formatFloat(-0.5)` returns `"0.5000"` instead of `"-0.5000"`

5. **Medium: `formatCurrency` produces malformed output for negative amounts** (`model.go:1277-1284`)

### Error Handling
No issues. These are pure functions that cannot fail in ways requiring error handling. Usage contexts are display/formatting where graceful degradation is appropriate.

### Data Integrity
**_ISSUES_FOUND_**

Same issues as Logic review, plus:

6. **Medium: Division by zero risk in `formatCost`** (`model.go:1188-1201`)
   - If `budget` is `0.0`, division produces `Inf`/`NaN`
   - Note: There is a guard in `renderProgressPanel` but not in `formatCost` itself

### Verdict
**FAIL**

The refactoring introduces edge case bugs with negative numbers and `math.MinInt`. While these edge cases may not occur in current usage (token counts, costs are positive), the utility functions should handle all valid integer inputs correctly.

**Required fixes before PASS:**
1. Handle `math.MinInt` in `IntToString` to prevent stack overflow
2. Handle negative numbers correctly in `FormatNumber`
3. Add test coverage for negative numbers and boundary values

## Iteration 2 - 2026-01-25 (Review Fixes)

### Issues Addressed

All 6 issues identified in the code review have been fixed:

1. **Critical: Stack overflow with `math.MinInt`** (convert.go:13-14)
   - **Fix**: Added explicit check for `math.MinInt` at the start of `IntToString`. Returns the hardcoded string "-9223372036854775808" since `-math.MinInt` overflows due to two's complement.
   - **Why**: Prevents infinite recursion that would crash the program.

2. **High: `FormatNumber` malformed output for negative numbers** (convert.go:32-45)
   - **Fix**: Added early return for negative numbers that formats the absolute value recursively and prepends minus sign. Also handles `math.MinInt` specially with hardcoded comma-separated string.
   - **Why**: The original code included the minus sign in length calculation for comma placement, producing malformed output like "-,123,456,7" instead of "-1,234,567".

3. **Medium: Missing test coverage for negative numbers** (convert_test.go)
   - **Fix**: Added test cases for `math.MinInt`, `math.MaxInt`, and various negative numbers in both `TestIntToString` and `TestFormatNumber`.
   - **Why**: Ensures edge cases are covered and prevents regressions.

4. **Medium: `formatFloat` loses sign for negative values between -1 and 0** (bridge.go:417-425)
   - **Fix**: Added early return for negative values that formats absolute value recursively and prepends minus sign.
   - **Why**: `int(-0.5)` truncates to 0, losing the sign. The fix preserves sign by handling negatives explicitly.

5. **Medium: `formatCurrency` malformed output for negative amounts** (model.go:1277-1284)
   - **Fix**: Added early return for negative amounts that formats absolute value recursively and prepends minus sign.
   - **Why**: Same issue as formatFloat - the sign was being lost during integer conversion.

6. **Medium: Division by zero risk in `formatCost`** (model.go:1188-1201)
   - **Fix**: Changed condition from `ratio > 0.8` to `budget > 0 && cost/budget > 0.8`.
   - **Why**: Prevents Inf/NaN from division when budget is zero, though the guard in renderProgressPanel makes this unlikely to trigger in practice.

### Verification

All tests pass, lint passes, build succeeds.
