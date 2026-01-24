# Notes: Fix Output Formatting

## Session 2026-01-24

### Observations

The `Formatter` class in `internal/output/formatter.go` exists with good functionality but:
1. `PrintBanner()` accepts limited parameters (specPath, model, maxIterations, promise)
2. `printBanner()` in root.go shows more info: context files, workflow, checker model, timeout, working dir, notes file, etc.
3. `PrintSummary()` exists but `printSummary()` in root.go shows different data (iterations, cost, tokens, duration, status)

### Implementation Complete

All four user stories have been implemented:

1. **Banner formatting**: Added `BannerConfig` struct and `PrintRichBanner()` method to formatter.
   - Shows all config options with coloured output
   - Respects NO_COLOR environment variable
   - `printBanner()` in root.go now delegates to formatter

2. **Iteration stats**: Added `IterationStartCallback` to loop controller.
   - `SetIterationStartCallback()` method on controller
   - Formatter's `PrintIterationStart()` and `PrintIterationEnd()` called in non-TUI mode
   - Tracks iteration timing for duration display

3. **Summary formatting**: Added `LoopSummary` struct and `PrintLoopSummary()` method.
   - Shows iterations, cost, tokens, duration, status
   - Status colour-coded (green for complete, red for error/incomplete)
   - `printSummary()` in root.go now delegates to formatter

4. **Spinner integration**: Already implemented in formatter, now used via worktree phases.
   - `StartSpinner()` and `StopSpinner()` available
   - Used in worktree setup and merge phases
   - Respects NO_COLOR

### Files Changed

- `internal/output/formatter.go`: Added BannerConfig, LoopSummary structs; PrintRichBanner, PrintLoopSummary methods
- `internal/output/formatter_test.go`: Added comprehensive tests for new methods
- `internal/loop/controller.go`: Added IterationStartCallback type and setter
- `cmd/orbit-cli/root.go`: Updated printBanner/printSummary to use formatter; added iteration callbacks
- `cmd/orbit-cli/continue.go`: Updated to use formatter
- `internal/tui/tasks.go`: Removed duplicate intToString function

### Tests

All tests pass. New tests added for:
- PrintRichBanner with various configs
- PrintRichBanner quiet mode
- PrintRichBanner with session ID, dry run, debug flags
- PrintLoopSummary completed/error/not completed states
