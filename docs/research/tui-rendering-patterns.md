# TUI Rendering Patterns Research

This document captures findings from researching Bubbletea and Lipgloss for fixing TUI rendering issues in Orbital CLI.

## Bubbletea Architecture

### Model-Update-View (Elm Architecture)

Bubbletea implements the Elm Architecture with three core components:

- **Init**: Returns an initial command for the application to run
- **Update**: Handles incoming events and updates the model accordingly
- **View**: Renders the UI based on data in the model

State flow is unidirectional: User input -> Update -> Model -> View -> Display

### Current Implementation Analysis

The Orbital TUI (`internal/tui/model.go`) correctly follows Elm Architecture:

- `Model` struct holds all state (lines 66-96)
- `Update()` handles all state mutations (lines 168-280)
- `View()` renders based on model state (lines 622-633)

### Best Practices (Applied/Needed)

| Practice | Current State | Notes |
|----------|--------------|-------|
| State changes in Update() | Yes | Model mutations happen correctly in Update |
| Handle WindowSizeMsg | Yes | Layout recalculates on resize (line 171) |
| No direct goroutines | Partial | Bridge uses goroutine with channel (acceptable) |
| Value receivers on Model | Yes | Using value receivers correctly |

### Terminal Resize Handling

Resize events arrive as `tea.WindowSizeMsg`. The current implementation:

1. Receives WindowSizeMsg (line 170)
2. Calls `CalculateLayout()` with new dimensions (line 171)
3. Invalidates wrapped lines cache (line 175)
4. Clamps scroll position (lines 178-195)

This pattern is correct.

## Lipgloss Layout Primitives

### Available Functions

**JoinVertical(pos Position, strs ...string) string**
- Joins strings vertically along a horizontal axis
- Position: 0.0 = left, 0.5 = centre, 1.0 = right
- Helper constants: `lipgloss.Left`, `lipgloss.Center`, `lipgloss.Right`

**JoinHorizontal(pos Position, strs ...string) string**
- Joins strings horizontally along their vertical axis
- Position: 0.0 = top, 0.5 = centre, 1.0 = bottom
- Helper constants: `lipgloss.Top`, `lipgloss.Center`, `lipgloss.Bottom`

**Place(width, height int, hPos, vPos Position, str string, opts ...WhitespaceOption) string**
- Places content within whitespace at specified position
- Useful for centring content in a fixed area

### Current Implementation Analysis

The current TUI uses manual string concatenation in `renderFull()` (lines 641-679):

```go
sections = append(sections, RenderTopBorder(m.layout.Width, m.styles.Border))
sections = append(sections, m.renderHeader())
// ... more appends ...
return strings.Join(sections, "\n")
```

This approach works but requires manual height accounting.

### Recommendation

For fixed header/footer with scrollable content, the pattern is:

```go
func (m Model) View() string {
    header := m.renderHeader()
    content := m.renderScrollArea()
    footer := m.renderFooter()

    // Join vertically with left alignment
    return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}
```

The current manual approach is acceptable given the complexity of the layout.

## Text Width and Measurement

### Lipgloss Functions

**Width(str string) int**
- Returns the visible cell width of a string
- Excludes ANSI escape sequences from count

**Height(str string) int**
- Returns the number of lines in a string

**Size(str string) (width, height int)**
- Shorthand for getting both dimensions

### Current Implementation

Uses `ansi.StringWidth()` from `github.com/charmbracelet/x/ansi`:

- Line width measurement (model.go:721-722)
- Tab bar width calculation (model.go:765, 787)
- Scroll area rendering (model.go:1027)
- File content rendering (model.go:879-883)

Both `lipgloss.Width()` and `ansi.StringWidth()` are ANSI-aware. Using `ansi.StringWidth()` directly is fine.

### Text Wrapping and Truncation

**MaxWidth(n int) style**
- Limits rendered output to n cells
- Truncates from the left (default behaviour)

**Inline(true) style**
- Forces rendering onto a single line

**ansi.Truncate(str, width, tail string) string**
- Truncates a string at a given width
- ANSI-aware (preserves escape sequences)
- Used in file content rendering (model.go:888)

### Current Manual Width Calculations

Places where manual width calculations occur:

1. **renderHeader** (model.go:719-727): Calculates padding between brand and metrics
2. **renderTabBar** (model.go:786-792): Calculates tab content padding
3. **renderMainContent/renderScrollArea** (model.go:1026-1032): Pads lines to content width
4. **renderTaskPanel** (model.go:1058-1062): Header padding
5. **renderTask** (model.go:1091-1106): Task line padding
6. **renderProgressPanel** (model.go:1143-1162): Line padding
7. **renderSessionPanel** (model.go:1221-1248): Line padding

These calculations follow a consistent pattern:
```go
contentWidth := m.layout.ContentWidth()
lineWidth := ansi.StringWidth(content)
padding := contentWidth - lineWidth
if padding < 0 { padding = 0 }
return border + content + strings.Repeat(" ", padding) + border
```

### Recommendation: Library Primitives vs Manual

**Keep manual approach when:**
- Need precise control over border characters
- Content has complex multi-segment structure
- Performance is critical (avoiding allocations)

**Use library primitives when:**
- Simple single-content panels
- Standard padding/centering
- Content doesn't need border integration

The current manual approach is appropriate because:
1. Each line has border characters (`║`) that integrate with content
2. Performance matters for streaming output
3. Layout calculations are already centralised in `CalculateLayout()`

However, the repetitive padding pattern could be extracted to a helper:

```go
func (m Model) padLine(content string) string {
    border := m.styles.Border.Render(BoxVertical)
    contentWidth := m.layout.ContentWidth()
    lineWidth := ansi.StringWidth(content)
    padding := contentWidth - lineWidth
    if padding < 0 { padding = 0 }
    return border + content + strings.Repeat(" ", padding) + border
}
```

## Unicode, ANSI, and Wide Characters

### Library Handling

Lipgloss and ansi packages handle:
- ANSI escape sequences (colours, formatting)
- Wide characters (CJK, emoji) via `runewidth`
- Tab characters (converted to 4 spaces)

### Current Implementation

The `wrapLine()` function (model.go:1308-1369) handles ANSI manually:

```go
// Track ANSI escape sequences
if r == '\x1b' {
    inEscape = true
    continue
}
if inEscape {
    if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
        inEscape = false
    }
    continue
}
```

### Recommendation

The manual ANSI tracking could potentially be replaced with library functions, but the current implementation is correct and tested. Changes here risk introducing regressions.

## Fixed Header/Footer with Scrollable Content

### Standard Pattern

The viewport component from `github.com/charmbracelet/bubbles/viewport` provides scrollable content with:
- Fixed dimensions
- Keyboard navigation (up, down, page up, page down)
- Mouse wheel support
- Scroll position tracking

### Current Implementation

Orbital implements custom scrolling:
- `RingBuffer` for bounded memory (ringbuffer.go)
- Manual scroll offset tracking (`outputScroll`, `outputTailing`)
- Wrapped lines cache for performance
- Custom scroll methods (`scrollUp`, `scrollDown`, etc.)

### Recommendation

The custom implementation is appropriate because:
1. RingBuffer provides memory bounds (1000 lines max)
2. Tailing mode auto-follows new content
3. Wrapped lines cache optimises performance
4. Custom styling integrates with border frame

Using viewport component would require significant refactoring without clear benefit.

## Identified Rendering Issues

From the screenshot (`docs/plans/broken-tui-rendering.png`):

### Issue 1: Duplicate Status Bars

Multiple token/cost lines stacked at bottom. Root cause investigation needed in:
- `renderProgressPanel()` - renders 2 lines (correct)
- `renderSessionPanel()` - renders 2 lines (correct)
- `renderFull()` - joins all sections (check for duplication)

Hypothesis: The issue may be in how sections are joined or in the height calculation not accounting for all panel heights.

### Issue 2: Text Truncation in Footer

Shows "...otes-223905-continuous-improvement.md" instead of full path.

Root cause: `formatPath()` (model.go:1254-1260) uses a fixed `maxLen := 40`:

```go
if len(path) > maxLen {
    path = "..." + path[len(path)-maxLen+3:]
}
```

This truncates from the left, which loses the meaningful prefix. Consider:
1. Truncating from the middle: "docs/.../notes.md"
2. Using more available width
3. Showing only filename when constrained

### Issue 3: Overlapping UI Sections

Content area overlaps with footer. Root cause in `CalculateLayout()`:

The `BorderHeight = 6` constant (layout.go:33) may not account for all borders:
- Top border (1)
- After header (1)
- After tab bar (1)
- After scroll area (1)
- After task panel (1, when visible)
- After progress panel (1)
- Bottom border (1)

That's 7 borders minimum, but `BorderHeight = 6`. When task panel is visible, an extra border is added (layout.go:108-110), making it 7. But the base constant seems off.

### Recommendations for Fixes

1. **Audit border count**: Trace `renderFull()` and count actual border lines vs constants
2. **Dynamic path truncation**: Use available width, not fixed 40 chars
3. **Test with various terminal sizes**: Ensure layout works at minimum (80x24) and larger
4. **Add layout debugging**: Temporary logging of calculated vs actual heights

## Summary

The TUI implementation follows Bubbletea best practices and uses ANSI-aware text handling correctly. The rendering issues appear to be:

1. **Height miscalculation**: `BorderHeight` constant may be incorrect
2. **Fixed truncation width**: Path truncation uses hardcoded width instead of available space
3. **Potential section duplication**: Need to trace `renderFull()` output

The fixes should focus on:
1. Correcting the border height constant in `layout.go`
2. Making path truncation dynamic in `formatPath()` and `formatPaths()`
3. Adding height assertions or debugging to catch layout errors

## References

- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/)
- [Bubbletea GitHub](https://github.com/charmbracelet/bubbletea)
- [Lipgloss GitHub](https://github.com/charmbracelet/lipgloss)
- [Lipgloss package docs](https://pkg.go.dev/github.com/charmbracelet/lipgloss)
- [Viewport package docs](https://pkg.go.dev/github.com/charmbracelet/bubbles/viewport)
- [Terminal UI with Bubble Tea](https://packagemain.tech/p/terminal-ui-bubble-tea)
