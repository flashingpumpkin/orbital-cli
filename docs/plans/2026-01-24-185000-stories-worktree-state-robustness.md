# User Stories: Worktree State File Robustness

## Problem Statement

The worktree state file (`worktree-state.json`) is the source of truth for tracking active worktrees. Analysis revealed critical issues that can cause data loss or complete feature failure:

1. **Race condition**: Parallel processes overwrite each other's changes (Load-Modify-Save without locking)
2. **No corruption recovery**: Invalid JSON causes all operations to fail
3. **Non-atomic writes**: Power failure during write corrupts the file

**Affected code:**
- `internal/worktree/state.go` - All state persistence methods
- `cmd/orbital/root.go:290` - State persistence after setup
- `cmd/orbital/continue.go:63-71` - State loading for resume

---

## User Story 1: Implement File Locking for State Operations

**As a** developer running multiple orbital instances in parallel
**I want** concurrent worktree state updates to be atomic
**So that** I don't lose worktree tracking information

### Context

The state file uses a Load-Modify-Save pattern without locking. Two parallel processes can race:

```
Process A: Load() -> state has [wt1]
Process B: Load() -> state has [wt1]
Process A: Add(wt2) -> Save([wt1, wt2])
Process B: Add(wt3) -> Save([wt1, wt3])  <- wt2 is LOST!
```

### Acceptance Criteria

- [ ] Given two parallel `orbital --worktree` commands, when both try to Add() a worktree, then both worktrees appear in the state file
- [ ] Given a lock is held by another process, when Add() is called, then it waits up to 5 seconds before failing
- [ ] Given lock wait times out, when Add() is called, then a clear error message is returned
- [ ] Given a stale lock file (process died), when Add() is called after 30 seconds, then the stale lock is broken
- [ ] Given normal operation, when Add() completes, then the lock file is removed

### Technical Notes

Create a file-based advisory lock mechanism:

```go
// internal/worktree/state.go

const lockTimeout = 5 * time.Second
const staleLockAge = 30 * time.Second

func (m *StateManager) acquireLock() (func(), error) {
    lockPath := m.StatePath() + ".lock"
    // Try to create lock file exclusively
    // If exists, check age for stale lock
    // Return release function
}

func (m *StateManager) Add(wt WorktreeState) error {
    release, err := m.acquireLock()
    if err != nil {
        return fmt.Errorf("failed to acquire state lock: %w", err)
    }
    defer release()

    // Existing Load-Modify-Save logic
}
```

### Definition of Done

- [ ] Lock acquisition implemented with timeout
- [ ] Stale lock detection and recovery implemented
- [ ] All state mutation methods (Add, Remove, UpdateSessionID) use locking
- [ ] Test for concurrent Add() operations (may use goroutines)
- [ ] All existing tests pass
- [ ] `go test -race ./internal/worktree/...` passes

**Effort Estimate**: M

---

## User Story 2: Add State File Corruption Recovery

**As a** developer whose state file got corrupted
**I want** orbital to recover gracefully
**So that** I can continue working without manual intervention

### Context

If `worktree-state.json` contains invalid JSON, `Load()` returns an error and all operations fail:

```go
// state.go:57-59
if err := json.Unmarshal(data, &state); err != nil {
    return nil, fmt.Errorf("failed to parse state file: %w", err)
}
```

This blocks `orbital continue`, `orbital status`, and all worktree operations.

### Acceptance Criteria

- [ ] Given state file contains invalid JSON, when Load() is called, then it attempts to restore from backup
- [ ] Given backup exists and is valid, when recovery runs, then state is restored and warning is logged
- [ ] Given no backup exists, when Load() fails, then filesystem is scanned for actual worktrees
- [ ] Given filesystem scan finds worktrees at `.orbital/worktrees/*`, when recovery runs, then state is rebuilt
- [ ] Given recovery succeeds, when operation completes, then a backup of the corrupt file is kept for debugging
- [ ] Given `orbital worktree repair` is run, when worktrees exist on disk but not in state, then state is rebuilt

### Technical Notes

```go
func (m *StateManager) Load() (*StateFile, error) {
    data, err := os.ReadFile(m.StatePath())
    if os.IsNotExist(err) {
        return &StateFile{Worktrees: []WorktreeState{}}, nil
    }
    if err != nil {
        return nil, fmt.Errorf("failed to read state file: %w", err)
    }

    var state StateFile
    if err := json.Unmarshal(data, &state); err != nil {
        // Try recovery
        if recovered, recoverErr := m.recoverFromCorruption(data); recoverErr == nil {
            return recovered, nil
        }
        return nil, fmt.Errorf("failed to parse state file (recovery failed): %w", err)
    }
    return &state, nil
}

func (m *StateManager) recoverFromCorruption(corruptData []byte) (*StateFile, error) {
    // 1. Try backup file
    // 2. If no backup, scan filesystem
    // 3. Keep corrupt file as .corrupt for debugging
}
```

### Definition of Done

- [ ] Backup created before each Save() operation (`.json.bak`)
- [ ] Recovery from backup implemented
- [ ] Filesystem scan fallback implemented
- [ ] Corrupt file preserved as `.json.corrupt`
- [ ] Warning logged on recovery
- [ ] Tests for corruption scenarios
- [ ] All existing tests pass

**Effort Estimate**: M

---

## User Story 3: Implement Atomic State File Writes

**As a** developer
**I want** state file writes to be atomic
**So that** power failures don't corrupt my state

### Context

`os.WriteFile` is not atomic - power failure mid-write corrupts the file:

```go
// state.go:76
if err := os.WriteFile(m.StatePath(), data, 0644); err != nil {
```

### Acceptance Criteria

- [ ] Given Save() is called, when write completes, then file is atomically replaced
- [ ] Given power failure during write, when system restarts, then previous valid state remains
- [ ] Given temporary file creation fails, when Save() is called, then error includes context
- [ ] Given filesystem sync fails, when Save() is called, then error is returned (not silent corruption)

### Technical Notes

```go
func (m *StateManager) Save(state *StateFile) error {
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal state: %w", err)
    }

    // Write to temp file in same directory (same filesystem for atomic rename)
    dir := filepath.Dir(m.StatePath())
    tmpFile, err := os.CreateTemp(dir, "worktree-state-*.tmp")
    if err != nil {
        return fmt.Errorf("failed to create temp file: %w", err)
    }
    tmpPath := tmpFile.Name()
    defer os.Remove(tmpPath) // Cleanup on error

    if _, err := tmpFile.Write(data); err != nil {
        tmpFile.Close()
        return fmt.Errorf("failed to write temp file: %w", err)
    }

    // Sync to disk before rename
    if err := tmpFile.Sync(); err != nil {
        tmpFile.Close()
        return fmt.Errorf("failed to sync temp file: %w", err)
    }
    tmpFile.Close()

    // Atomic rename (POSIX guarantees atomicity)
    if err := os.Rename(tmpPath, m.StatePath()); err != nil {
        return fmt.Errorf("failed to rename temp file: %w", err)
    }

    return nil
}
```

### Definition of Done

- [ ] Atomic write using temp file + rename implemented
- [ ] File sync before rename
- [ ] Temp file cleanup on error
- [ ] Tests verify atomic behavior
- [ ] All existing tests pass

**Effort Estimate**: S

---

## Implementation Order

1. **Story 3** (Atomic writes) - Foundation for safe operations
2. **Story 2** (Corruption recovery) - Enables recovery from existing issues
3. **Story 1** (File locking) - Prevents future race conditions

## Verification

After implementation:

```bash
# Test concurrent operations
for i in {1..5}; do
    orbital --worktree spec.md &
done
wait

# Verify all worktrees tracked
orbital worktree list  # Should show 5 worktrees

# Test corruption recovery
echo "invalid json" > .orbital/worktree-state.json
orbital continue  # Should recover and continue
```

---

## Dependencies

- None (isolated to state management)

## Risks

- File locking behavior differs between OSes (test on Linux and macOS)
- Atomic rename may not work across filesystems (temp file must be in same directory)
