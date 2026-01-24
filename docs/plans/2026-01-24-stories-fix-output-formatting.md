# User Stories: Restore Output Formatting

## Problem Summary

The TUI integration replaced the previous text-based output, but non-TUI mode (`--minimal` or non-interactive terminals) lost the nice formatting that existed before.

**A sophisticated `Formatter` class exists in `internal/output/formatter.go` but was never integrated.** Instead, `root.go` uses hardcoded plain-text functions.

### What Was Lost

| Feature | Previously | Now |
|---------|-----------|-----|
| Banner | Coloured with config summary | Plain ASCII |
| Per-iteration stats | Coloured with duration, tokens, cost | Not shown in non-TUI mode |
| Summary | Coloured with totals | Plain ASCII |
| Spinner | Animated with messages | Not used |

### The Formatter Provides (But Unused)

- `PrintBanner()` - Cyan bold title, formatted config parameters
- `PrintIterationStart()` - Blue bold iteration marker
- `PrintIterationEnd()` - Duration, tokens, cost with colours
- `PrintSummary()` - Complete execution summary with colours
- `StartSpinner()` / `StopSpinner()` - Animated progress

### Files Involved

| File | Status |
|------|--------|
| `internal/output/formatter.go` | Created but abandoned |
| `internal/output/stream.go` | Still active for streaming |
| `cmd/orbit-cli/root.go` | Uses plain text instead of formatter |

---

## User Story 1: Use Formatter for Non-TUI Mode Banner

**As a** developer running orbit in non-TUI mode
**I want** to see a nicely formatted banner with configuration details
**So that** I can verify my settings before execution starts

### Acceptance Criteria

- [x] Non-TUI mode uses `Formatter.PrintBanner()` instead of plain text
- [x] Banner shows: spec files, model, iterations, budget, timeout, working directory
- [x] Colours are used (respecting NO_COLOR environment variable)
- [x] Banner matches the quality of TUI mode presentation

### Definition of Done

- [x] `root.go` calls `Formatter.PrintBanner()` for non-TUI mode
- [x] Visual inspection confirms coloured output
- [x] NO_COLOR=1 disables colours

---

## User Story 2: Show Per-Iteration Stats in Non-TUI Mode

**As a** developer running orbit in non-TUI mode
**I want** to see iteration stats (duration, tokens, cost) after each iteration
**So that** I can monitor progress without the TUI

### Acceptance Criteria

- [x] After each iteration, display: iteration number, duration, tokens (in/out), cost
- [x] Use `Formatter.PrintIterationStart()` and `Formatter.PrintIterationEnd()`
- [x] Status is colour-coded: green for complete, yellow for continuing, red for error
- [x] Output is clean and not mixed with Claude's streaming output

### Definition of Done

- [x] Iteration stats appear in non-TUI mode
- [x] Colours and formatting match formatter design
- [x] Unit tests for iteration callback display

---

## User Story 3: Use Formatter for Non-TUI Mode Summary

**As a** developer running orbit in non-TUI mode
**I want** to see a nicely formatted summary at the end
**So that** I can quickly understand the execution results

### Acceptance Criteria

- [x] Non-TUI mode uses `Formatter.PrintSummary()` instead of plain text
- [x] Summary shows: total iterations, total cost, total tokens, duration, final status
- [x] Status is colour-coded based on outcome
- [x] Summary is visually distinct from iteration output

### Definition of Done

- [x] `root.go` calls `Formatter.PrintSummary()` for non-TUI mode
- [x] Visual inspection confirms formatted output
- [x] Summary values match actual execution

---

## User Story 4: Integrate Spinner for Long Operations

**As a** developer running orbit in non-TUI mode
**I want** to see a spinner during Claude execution
**So that** I know the tool hasn't frozen

### Acceptance Criteria

- [x] Spinner displays during Claude CLI execution in non-TUI mode
- [x] Spinner message shows current iteration
- [x] Spinner stops cleanly when output starts streaming
- [x] Spinner respects NO_COLOR and terminal capabilities

### Definition of Done

- [x] Spinner appears during execution gaps
- [x] No visual glitches or leftover characters
- [x] Works in various terminal types

---

## Technical Notes

### Current State (root.go)

```go
if !useTUI {
    printBanner(cfg, sp, contextFiles, wf)  // Plain ASCII, not using Formatter
    // ...
    printSummary(loopState)  // Plain ASCII, not using Formatter
}
```

### Proposed Change

```go
if !useTUI {
    formatter := output.NewFormatter()
    formatter.PrintBanner(cfg, sp, contextFiles, wf)
    // ... in iteration callback ...
    formatter.PrintIterationStart(iteration, maxIterations)
    formatter.PrintIterationEnd(iteration, duration, tokensIn, tokensOut, cost, status)
    // ... at end ...
    formatter.PrintSummary(loopState)
}
```

### Formatter Methods to Use

From `internal/output/formatter.go`:

```go
func (f *Formatter) PrintBanner(...)
func (f *Formatter) PrintIterationStart(iteration, total int)
func (f *Formatter) PrintIterationEnd(iteration int, duration time.Duration, tokensIn, tokensOut int, cost float64, status string)
func (f *Formatter) PrintSummary(...)
func (f *Formatter) StartSpinner(msg string)
func (f *Formatter) StopSpinner()
```

### Streaming Output Integration

The `StreamProcessor` in `stream.go` handles real-time Claude output with colours. It should continue to work alongside the Formatter. The Formatter handles structural output (banner, iteration markers, summary) while StreamProcessor handles Claude's streaming content.

### Related Files

- `internal/output/formatter.go` - Wire up to root.go
- `cmd/orbit-cli/root.go` - Replace plain text functions with Formatter calls
- `internal/output/stream.go` - Ensure compatibility with Formatter
