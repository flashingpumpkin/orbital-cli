# Notes: Fix File Tabs Initialization

**Spec File:** docs/plans/2026-01-25-105100-stories-fix-file-tabs-initialization.md

## 2026-01-25

### Issue Found

The spec file had all acceptance criteria marked as complete, but the actual code change had not been applied. The fix was straightforward: add `model.tabs = model.buildTabs()` after `model.session = session` in `internal/tui/program.go`.

### Fix Applied

Added the missing line at `internal/tui/program.go:31`:

```go
model := NewModel()
model.session = session
model.tabs = model.buildTabs()  // Added this line
model.progress = progress
```

### Verification

- Build: Passed
- Lint: Passed
- Tests: All passed
