# Continuous Improvement Spec

This spec guides autonomous improvement iterations. Each iteration should pick the highest-leverage task, complete it fully, and check it off.

Explore the codebase to identify areas for improvement and add them to this spec file. 

## Code Quality

### Deduplication
- [x] Identify and extract repeated code patterns into shared functions (intToString/formatNumber extracted to internal/util)
- [ ] Consolidate similar test helpers across packages
- [ ] Remove redundant error handling patterns
- [ ] Unify string formatting approaches

### Refactoring
- [ ] Simplify complex functions (cyclomatic complexity > 10)
- [ ] Extract long functions into smaller, focused units
- [ ] Improve naming for unclear variables or functions
- [ ] Reduce parameter counts on functions with 4+ parameters
- [ ] Convert nested conditionals to early returns

### Error Handling
- [ ] Replace generic errors with typed errors where appropriate
- [ ] Add context to error wrapping (use fmt.Errorf with %w)
- [ ] Ensure errors are handled, not silently ignored
- [ ] Review panic usage and convert to errors where possible

## Bug Fixes

### TUI Rendering (HIGH PRIORITY)
Reference: docs/plans/broken-tui-rendering.png

#### Research (do this first)
- [x] Study Bubbletea architecture: Model, Update, View cycle and how state flows
- [x] Understand Lipgloss layout primitives: Place, JoinVertical, JoinHorizontal, and how dimensions are calculated
- [x] Research Bubbletea best practices for fixed headers/footers with scrollable content
- [x] Document how terminal resize events propagate through the model
- [x] **Text rendering deep dive**: Research how Lipgloss handles text width, wrapping, truncation, and padding automatically. Goal: eliminate manual line width calculations and string formatting in favour of library primitives
- [x] Investigate Lipgloss Width(), MaxWidth(), Inline(), and text measurement functions
- [x] Research how to let Lipgloss handle Unicode, ANSI escape sequences, and wide characters without manual intervention
- [x] Review existing internal/tui code to identify places where manual width/formatting calculations can be replaced with library functions
- [x] Create docs/research/tui-rendering-patterns.md with findings and recommended approach

#### Fixes (after research is complete)
- [ ] Fix duplicate status bars appearing at bottom of TUI (multiple token/cost lines stacked)
- [ ] Fix text truncation in Notes/State footer line (shows "...otes-223905-continuous-improvement.md" instead of full path)
- [ ] Fix overlapping UI sections between main content area and footer
- [ ] Ensure footer height calculation accounts for all status lines
- [ ] Verify terminal resize handling doesn't cause layout corruption
- [ ] Assess and fix any and all other rendering issues that lead to broken UI rendering

### Known Issues
- [ ] Review TODO comments and address actionable items
- [ ] Check for potential nil pointer dereferences
- [ ] Verify slice bounds are checked before access
- [ ] Ensure goroutines are properly synchronised

### Edge Cases
- [ ] Handle empty input gracefully across public functions
- [ ] Validate configuration values before use
- [ ] Check for integer overflow in arithmetic operations
- [ ] Handle file system errors (permissions, missing files)

## Test Coverage

### Missing Tests
- [ ] Add tests for error paths in existing code
- [ ] Cover edge cases not currently tested
- [ ] Add integration tests for component interactions
- [ ] Test concurrent access patterns

### Test Quality
- [ ] Replace magic numbers with named constants
- [ ] Improve test names to describe behaviour
- [ ] Add table-driven tests where loops exist
- [ ] Remove test duplication

## Documentation

### Code Comments
- [ ] Add godoc comments to exported functions missing them
- [ ] Remove stale comments that no longer match code
- [ ] Document non-obvious algorithm choices
- [ ] Add examples to complex public APIs

## Performance

### Efficiency
- [ ] Avoid unnecessary allocations in hot paths
- [ ] Use appropriate data structures (maps vs slices)
- [ ] Preallocate slices when size is known
- [ ] Review string concatenation in loops

## Completion Criteria

When all high-leverage improvements have been made and the codebase is in good shape, output:

<promise>COMPLETE</promise>
