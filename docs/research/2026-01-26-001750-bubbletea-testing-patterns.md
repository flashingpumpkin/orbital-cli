# Bubbletea Testing Patterns and Component Library Research

## Overview

This document captures research on testing patterns for Bubbletea TUI applications, with a focus on the `teatest` package, golden file testing approaches, and the Bubbles component library.

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

## Bubbles Component Library

The `github.com/charmbracelet/bubbles` library provides reusable TUI components designed to work with Bubbletea. These components follow the Elm architecture pattern and integrate seamlessly.

### Available Components

| Component | Description | Current Orbital Usage |
|-----------|-------------|----------------------|
| **Viewport** | Vertically scrollable content with keyboard/mouse support | Manual scroll logic in `model.go` |
| **Progress** | Animated progress bar with gradient support | Custom `RenderProgressBar` in `borders.go` |
| **List** | Feature-rich item browser with filtering and pagination | Custom task panel rendering |
| **Spinner** | Loading indicator with animation presets | Using `briandowns/spinner` externally |
| **Help** | Auto-generated keybinding help view | Manual help bar rendering |
| **Paginator** | Dot or numeric pagination indicator | None |
| **Table** | Columnar data display with scrolling | None |
| **Textinput** | Single-line text field | None |
| **Textarea** | Multi-line text input | None |
| **Filepicker** | Directory navigation and file selection | None |
| **Timer/Stopwatch** | Time tracking components | None |

### Opportunities for Orbital CLI

#### 1. Viewport for Output Scrolling (HIGH VALUE)

The current implementation in `model.go` has 200+ lines of manual scroll logic:
- `wrapAllOutputLines()`, `wrapLine()`, `findBreakPoint()`
- `scrollUp()`, `scrollDown()`, `scrollPageUp()`, `scrollPageDown()`
- `renderScrollArea()`, `renderFileContent()`
- Wrapped lines cache management

The Bubbles `viewport` component provides:
- Built-in keyboard navigation (up/down/pgup/pgdown/home/end)
- Mouse wheel support (already handled in Orbital)
- High-performance mode for alternate screen buffers
- Automatic content wrapping via `viewport.SetContent()`
- Width/height management

**Migration path:**
```go
import "github.com/charmbracelet/bubbles/viewport"

type Model struct {
    outputViewport viewport.Model
    fileViewports  map[string]viewport.Model
    // ... other fields
}

func NewModel() Model {
    vp := viewport.New(80, 20)
    vp.SetContent("Waiting for output...")
    return Model{
        outputViewport: vp,
        fileViewports:  make(map[string]viewport.Model),
        // ...
    }
}
```

**Risk**: The current tailing mode (auto-scroll to bottom) may need custom handling since viewport doesn't have this built in. Would need to track content changes and call `viewport.GotoBottom()`.

#### 2. Progress Component (MEDIUM VALUE)

Current `RenderProgressBar` in `borders.go` is functional but Bubbles progress offers:
- Smooth animation via Harmonica spring physics
- Gradient fills
- Multiple style presets

The current implementation is 30 lines and works well. Migration would be cosmetic rather than functional.

**Recommendation**: Keep custom implementation unless animation is desired.

#### 3. List Component for Tasks (MEDIUM VALUE)

The task panel currently renders manually with:
- Status icons (complete/in-progress/pending)
- Truncation logic
- Scroll indicator when tasks overflow

The Bubbles `list` component provides:
- Built-in pagination and scrolling
- Fuzzy filtering (could be useful for many tasks)
- Status line and help integration
- Customisable item rendering via `list.ItemDelegate`

**Considerations**:
- The task panel is capped at 6 items, so list's pagination may be overkill
- Custom item rendering is already clean
- List component is heavier than needed for this use case

**Recommendation**: Keep custom implementation for simplicity.

#### 4. Help Component (LOW VALUE)

The current help bar is static and simple:
```go
help := "  ↑/↓ scroll  ←/→ tab  1-9 jump  r reload  q quit"
```

Bubbles `help` auto-generates from `key.Binding` definitions:
```go
import "github.com/charmbracelet/bubbles/help"
import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
    Up    key.Binding
    Down  key.Binding
    Quit  key.Binding
}
```

**Recommendation**: Low priority. Current implementation is clear and maintenance-free.

### Integration Recommendations

| Priority | Component | Reason |
|----------|-----------|--------|
| **High** | Viewport | Removes 200+ lines of scroll logic, better tested, mouse support |
| Low | Progress | Cosmetic improvement only |
| Low | List | Overkill for 6-item task panel |
| Low | Help | Current implementation is simpler |

### Implementation Plan for Viewport

1. Add dependency:
   ```bash
   go get github.com/charmbracelet/bubbles/viewport
   ```

2. Replace `outputLines *RingBuffer` with `viewport.Model`

3. Modify `AppendOutput` to:
   - Append to internal buffer
   - Call `viewport.SetContent()` with joined lines
   - Call `viewport.GotoBottom()` if tailing

4. Update `renderScrollArea()` to simply return `viewport.View()`

5. Forward keyboard events to viewport:
   ```go
   case tea.KeyMsg:
       m.outputViewport, cmd = m.outputViewport.Update(msg)
   ```

6. Keep file content rendering separate (each file tab gets its own viewport)

### Testing Implications

Using Bubbles components improves testability:
- Components are well-tested upstream
- Reduces surface area for Orbital-specific bugs
- Golden file tests focus on integration, not scroll math
- Components handle edge cases (empty content, resize, wide chars)

## References

- [teatest package documentation](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest)
- [Writing Bubble Tea Tests](https://charm.land/blog/teatest/)
- [Writing Bubble Tea Tests (carlosbecker.com)](https://carlosbecker.com/posts/teatest/)
- [catwalk library](https://github.com/knz/catwalk)
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/)
- [Bubbles component library](https://github.com/charmbracelet/bubbles)
- [Bubbles viewport documentation](https://pkg.go.dev/github.com/charmbracelet/bubbles/viewport)
