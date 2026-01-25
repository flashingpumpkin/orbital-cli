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
