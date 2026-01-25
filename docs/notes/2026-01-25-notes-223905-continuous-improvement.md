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

## Code Review - Iteration 2

### Security
No issues. The changes are defensive bug fixes to pure formatting functions. No external input vectors, no injection risks, no authentication concerns. The `math.MinInt` handling correctly addresses the two's complement overflow edge case.

### Design
No issues. The refactoring maintains good principles:
- Single Responsibility: Each function maintains a single purpose
- Correct placement of utility functions in shared package
- Consistent pattern (recursive handling of negatives) applied across all formatting functions
- No SOLID violations

### Logic
**_ISSUES_FOUND_**

1. **Division by zero in `renderHeader`** (model.go:694-695)
   - `iterRatio := float64(p.Iteration) / float64(p.MaxIteration)` produces `+Inf` when `MaxIteration` is 0
   - `costRatio := p.Cost / p.Budget` produces `+Inf`/`NaN` when `Budget` is 0
   - The fix was applied to `formatCost` (line 1193) but NOT to `renderHeader`

2. **Division by zero in `renderProgressPanel`** (model.go:1111)
   - Same issue: `iterRatio := float64(p.Iteration) / float64(p.MaxIteration)` when `MaxIteration` is 0
   - Inconsistent with the budget guard added on line 1141

3. **Floating point precision loss in `formatFloat`** (bridge.go:423-424)
   - `formatFloat(1.2345)` returns `"1.2344"` due to truncation instead of rounding
   - Uses `int()` truncation instead of rounding with `+0.5`

4. **Integer overflow in `formatFloat` for large values** (bridge.go:417-425)
   - When `f > math.MaxInt64`, `int(f)` overflows producing garbage output
   - Low severity as costs won't reach these values in practice

5. **Integer overflow in `formatCurrency` for large amounts** (model.go:1277-1288)
   - When `amount > math.MaxInt64/100`, overflow produces incorrect results
   - Low severity as budgets won't reach these values in practice

### Error Handling
No issues. The changes are to pure formatting functions with no failure modes requiring error handling.

### Data Integrity
**_ISSUES_FOUND_**

Same issues as Logic review. Additionally:

6. **Missing NaN/Inf handling** in `formatFloat` and `formatCurrency`
   - If upstream parsing produces NaN/Inf, these functions will produce unpredictable output
   - Low severity as this would indicate an upstream bug

### Verdict
**FAIL**

The division by zero issues (1 and 2) represent incomplete fixes. The same pattern was fixed in `formatCost` but not in `renderHeader` and `renderProgressPanel`. This inconsistency suggests the fix was incomplete.

**Required fixes before PASS:**
1. Add `MaxIteration > 0` guard before division in `renderHeader` (line 694)
2. Add `Budget > 0` guard before division in `renderHeader` (line 695)
3. Add `MaxIteration > 0` guard before division in `renderProgressPanel` (line 1111)

**Nice to have (low priority):**
4. Add rounding (+0.5) in `formatFloat` fractional calculation
5. Add overflow guards for extremely large values

## Iteration 3 - 2026-01-25 (Review Fixes Round 2)

### Issues Addressed

All 3 required fixes from Code Review - Iteration 2 have been addressed, plus 1 optional improvement:

1. **Division by zero in `renderHeader` (MaxIteration)** (model.go:694)
   - **Fix**: Added guard `if p.MaxIteration > 0` before division and before checking `iterRatio > 0.8`
   - **Why**: When MaxIteration is 0, division produces +Inf which causes incorrect warning styling

2. **Division by zero in `renderHeader` (Budget)** (model.go:695)
   - **Fix**: Added guard `if p.Budget > 0` before division and before checking `costRatio > 0.8`
   - **Why**: When Budget is 0, division produces +Inf/NaN which causes incorrect styling

3. **Division by zero in `renderProgressPanel` (MaxIteration)** (model.go:1111)
   - **Fix**: Added guard `if p.MaxIteration > 0` before division and before checking `iterRatio > 0.8`
   - **Why**: Same issue as renderHeader - inconsistent with the budget guard that was already present

4. **Floating point precision loss in `formatFloat`** (bridge.go:423-424) - OPTIONAL
   - **Fix**: Added rounding with `+0.5` before casting to int. Also added carry-over handling for edge case where rounding produces `frac >= 10000` (e.g., 1.99999 rounds to 2.0000)
   - **Why**: Truncation with `int()` loses precision. For example, `formatFloat(1.2345)` would return `"1.2344"` instead of the correctly rounded `"1.2345"`

### Verification

All tests pass, lint passes, build succeeds.

## Code Review - Iteration 3

### Security
No issues. The changes add defensive guards against division by zero and improve floating-point rounding precision. No injection risks, authentication issues, or data exposure vulnerabilities. The code is pure UI rendering with no external input vectors.

### Design
**_ISSUES_FOUND_**

1. **Inconsistent Division-by-Zero Pattern** (model.go:1150-1153)
   - The code at line 1150 divides BEFORE checking: `costRatio := p.Cost / p.Budget` then checks `if p.Budget == 0`
   - This violates the defensive pattern established in `renderHeader()` (lines 694-701) where checks occur BEFORE division
   - Creates inconsistency within the same file

2. **Repeated Ratio Threshold Logic** (model.go:704, 709, 1125)
   - The pattern `if p.MaxIteration > 0 && iterRatio > 0.8` is duplicated three times
   - Violates DRY principle; threshold changes require updating multiple locations

3. **Missing Abstraction for Safe Ratio Calculation** (model.go:696-698, 699-701, 1119-1121)
   - The pattern `if denominator > 0 { ratio = numerator / denominator }` repeats three times
   - Opportunity to extract a `safeRatio()` utility function

### Logic
**_ISSUES_FOUND_**

1. **Critical: Division occurs before guard in `renderProgressPanel`** (model.go:1150-1153)
   - Code: `costRatio := p.Cost / p.Budget` followed by `if p.Budget == 0 { costRatio = 0 }`
   - When `p.Budget == 0` and `p.Cost > 0`, division produces `+Inf` BEFORE the check
   - `RenderProgressBar(costRatio, ...)` at line 1154 receives infinity
   - This is inconsistent with the correct pattern in `renderHeader()` (lines 699-701) where guard precedes division

### Error Handling
No issues. The changes are defensive programming improvements that prevent undefined numeric behaviour. No swallowed errors, missing error propagation, or resource leaks.

### Data Integrity
**_ISSUES_FOUND_**

1. **Division by zero produces infinity before null check** (model.go:1150-1153)
   - Same issue as Logic review
   - When `p.Budget == 0`, the division `p.Cost / p.Budget` produces `+Inf`
   - The subsequent check sets `costRatio = 0` but infinity has already been computed
   - Data corruption risk: `RenderProgressBar()` receives infinity on line 1154

### Verdict
**FAIL**

The changes in `renderHeader()` and `renderProgressPanel()` for `iterRatio` are correct, following the guard-before-division pattern. However, there is an existing bug at line 1150-1153 where `costRatio` division occurs BEFORE the budget check. This was not introduced by this iteration but remains unfixed while the same pattern was correctly applied elsewhere.

**Required fixes before PASS:**
1. Fix line 1150-1153 to match the defensive pattern:
   ```go
   var costRatio float64
   if p.Budget > 0 {
       costRatio = p.Cost / p.Budget
   }
   ```

**Note:** This bug exists in existing code (not introduced by this iteration), but the review agents correctly identified that the same file now has inconsistent patterns after the fixes were applied to other locations.

## Iteration 4 - 2026-01-25 (Review Fixes Round 3)

### Task Selected
Fix division-by-zero bug in `renderProgressPanel` at line 1150-1153.

### Why Highest Leverage
This was the sole required fix from Iteration 3's code review to achieve a PASS verdict. The bug caused division to happen before the guard check, producing `+Inf` when `p.Budget == 0`. Fixing this completes the defensive pattern consistently across all ratio calculations in `model.go`.

### Key Decisions
Applied the same guard-before-division pattern used in `renderHeader()`:
```go
var costRatio float64
if p.Budget > 0 {
    costRatio = p.Cost / p.Budget
}
```

### Verification
- `make check` passes (lint and tests)
- `make build` succeeds

### Outcome
Fixed the inconsistent division-by-zero handling in `renderProgressPanel`. All ratio calculations in `model.go` now follow the same defensive pattern: guard before division.
