# Continuous Improvement Notes

Session notes for autonomous improvement iterations.

## Iteration Log

Record each iteration's work here:

---

### 2026-01-26: Bubbles Component Library Research

**Objective**: Evaluate `github.com/charmbracelet/bubbles` for improving TUI code quality.

**Findings**:

The Bubbles library provides ready-made Bubbletea components. After analysing the current TUI implementation, the highest-value opportunity is replacing custom scroll logic with the `viewport` component.

Current scroll implementation in `model.go`:
- `wrapLine()`, `wrapAllOutputLines()`, `findBreakPoint()`, `truncateToWidth()`
- `scrollUp()`, `scrollDown()`, `scrollPageUp()`, `scrollPageDown()`
- `renderScrollArea()`, `renderFileContent()`
- Wrapped lines cache management (`updateWrappedLinesCache()`, `invalidateWrappedLinesCache()`)

This is 200+ lines that could be replaced by viewport.Model with:
- Built-in keyboard navigation
- Mouse wheel support
- High-performance mode
- Better-tested code

**Tailing mode consideration**: The current implementation auto-scrolls to bottom when new content arrives. Viewport doesn't have this built in, so we'd need a wrapper that calls `viewport.GotoBottom()` on content changes when in tailing mode.

Other Bubbles components (progress, list, help) are lower priority since the current implementations are simpler and sufficient for Orbital's needs.

Updated:
- `docs/research/2026-01-26-001750-bubbletea-testing-patterns.md` with Bubbles analysis
- `docs/specs/2026-01-25-223905-continuous-improvement.md` with integration tasks

---

### 2026-01-26: Golden File Edge Case Tests

**Task selected**: Add golden file tests for edge cases (long paths, Unicode content, ANSI sequences, narrow terminal)

**Why highest leverage**: This task directly follows the completed golden file testing work. Edge case tests catch rendering bugs that basic tests miss, and the test infrastructure is already in place.

**Tests added**:

1. **TestGoldenLongPaths**: Very long file paths in session panel. Exercises path truncation logic (`formatPath`, `formatPaths`) with deeply nested directory structures.

2. **TestGoldenUnicodeContent**: Tasks and output with Chinese, Russian, and emoji characters. Tests proper handling of wide characters in layout calculations.

3. **TestGoldenANSISequences**: Output containing ANSI escape codes (colours, bold). Tests that ANSI-aware truncation (`ansi.Truncate`) handles styled content.

4. **TestGoldenVeryNarrowTerminal**: 60-column terminal (below 80-column minimum). Confirmed the TUI correctly displays an error message for undersized terminals rather than attempting broken layout.

**Decision**: The TUI has a minimum width of 80 columns by design. The narrow terminal test captures this validation behaviour, which is correct.

---
