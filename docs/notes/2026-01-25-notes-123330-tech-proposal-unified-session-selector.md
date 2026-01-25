# Session Notes: Unified Session Selector

## 2026-01-25 - Phase 1 Complete

### Completed
- Created `internal/session/` package with:
  - `session.go`: `Session` struct, `SessionType` enum, helper methods (`DisplayName`, `TypeLabel`, `Branch`, `Path`)
  - `collector.go`: `Collector` struct with `Collect()` and `ValidSessions()` methods
  - `session_test.go`: Unit tests for Session struct methods
  - `collector_test.go`: Unit tests for Collector including edge cases

### Design Decisions
- Used pointer fields (`*worktree.WorktreeState`, `*state.State`) to allow nil checks
- Collector returns all sessions (valid and invalid) to let UI show both
- Added synthetic "Queued files" session when queue has files but no regular session exists
- `ValidSessions` returns nil slice (not empty) when no valid sessions exist

### Test Notes
- Had to use PID 99999999 instead of 0 for testing stale sessions
- On Unix, PID 0 refers to the calling process's group, so `kill(0, 0)` succeeds
- Large PID ensures `IsStale()` returns true in tests

### Next Steps
- Phase 2: TUI Selector Component
