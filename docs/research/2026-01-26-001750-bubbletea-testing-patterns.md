# Bubbletea Testing Patterns Research

## Overview

This document captures research on testing patterns for Bubbletea TUI applications, with a focus on the `teatest` package and golden file testing approaches.

## Testing Approaches

### 1. Isolated Model Testing (Current Approach)

The current test suite in `internal/tui/model_test.go` uses isolated model testing:

- Create model with `NewModel()`
- Send messages directly via `model.Update(msg)`
- Assert state via type assertions on returned model
- Call `View()` and inspect string content

**Strengths:**
- Fast execution (no subprocess)
- Direct control over model state
- Simple to write and debug

**Weaknesses:**
- Bypasses full Bubbletea event loop
- Does not test integration with terminal I/O
- Manual `SetSession()` calls skip message handling (e.g., `buildTabs()`)

### 2. teatest Package

The `github.com/charmbracelet/x/exp/teatest` package provides helpers for full integration testing.

**Key Functions:**

```go
// Create test harness with terminal dimensions
tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

// Send messages or type text
tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
tm.Type("hello")

// Wait for specific output condition
teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
    return bytes.Contains(bts, []byte("expected"))
}, teatest.WithDuration(2*time.Second))

// Get final output for comparison
output := tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second))

// Assert against golden file
teatest.RequireEqualOutput(t, output)
```

**Strengths:**
- Full event loop integration
- Realistic terminal simulation
- Golden file comparison with `-update` flag
- Built-in timeout handling

**Weaknesses:**
- Experimental status (API not stable)
- Slower than isolated tests (subprocess overhead)
- Colour profile differences between local and CI environments
- Line ending issues with golden files

### 3. Golden File Testing

Golden file testing captures expected output and compares against it on subsequent runs.

**Workflow:**
1. Run tests with `-update` flag to generate golden files
2. Commit golden files to repository
3. Future test runs compare output to golden files
4. Any difference fails the test with a diff

**CI/CD Considerations:**
- Colour profiles may differ between environments
- Need `.gitattributes` to prevent line ending changes:
  ```
  *.golden binary
  ```
- Consider using `NO_COLOR=1` or `TERM=dumb` for deterministic output

### 4. catwalk Alternative

The `github.com/knz/catwalk` package offers datadriven testing for Bubbletea models.

**Features:**
- Table-driven test files with expected View() output
- State assertions after message sequences
- No subprocess overhead (like isolated testing)

## Recommendations for Orbital CLI

### Hybrid Approach

Given the current test suite and project needs, I recommend a hybrid approach:

1. **Keep isolated model tests** for core logic:
   - State transitions
   - Layout calculations
   - Edge cases and error conditions

2. **Add golden file tests** for key UI states:
   - Use teatest for full rendering verification
   - Create golden files for: empty state, with tasks, scrolling content
   - Run with deterministic colour settings

3. **Property-based assertions** for layout correctness:
   - Keep existing `TestRenderLineWidths` and `TestRenderTotalLineCount`
   - These verify invariants regardless of specific content

### Implementation Steps

1. Add `teatest` as a test dependency:
   ```
   go get github.com/charmbracelet/x/exp/teatest
   ```

2. Create test harness helper:
   ```go
   func createTestProgram(t *testing.T, opts ...teatest.TestOption) *teatest.TestModel {
       return teatest.NewTestModel(t, NewModel(), opts...)
   }
   ```

3. Add `.gitattributes` entry for golden files:
   ```
   testdata/*.golden binary
   ```

4. Create golden file tests with deterministic output:
   ```go
   func TestGoldenEmptyState(t *testing.T) {
       t.Setenv("NO_COLOR", "1")
       tm := createTestProgram(t, teatest.WithInitialTermSize(80, 24))
       tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
       teatest.RequireEqualOutput(t, tm.FinalOutput(t))
   }
   ```

5. Document update procedure for maintainers:
   ```
   go test -update ./internal/tui/...
   ```

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| teatest API changes | Pin version, monitor upstream |
| Colour differences | Use NO_COLOR=1 in tests |
| Line ending issues | Binary attribute in .gitattributes |
| Flaky timing | Use generous WaitFor timeouts |
| Snapshot maintenance burden | Limit golden tests to key states |

## References

- [teatest package documentation](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest)
- [Writing Bubble Tea Tests](https://charm.land/blog/teatest/)
- [Writing Bubble Tea Tests (carlosbecker.com)](https://carlosbecker.com/posts/teatest/)
- [catwalk library](https://github.com/knz/catwalk)
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/)
