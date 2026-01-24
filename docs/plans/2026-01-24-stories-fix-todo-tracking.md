# User Stories: Fix TODO Tracking

## Problem Summary

TODO/task tracking only works in TUI mode. When using non-TUI modes (minimal, verbose, `--todos-only`), task state is never tracked or displayed.

**Root Cause**: `TaskTracker` is only instantiated in the TUI code path. The `StreamProcessor` used in non-TUI modes has no task tracking capability.

### Data Flow Analysis

**TUI Mode (Working)**:
```
Bridge.Write() → Parser → TaskTracker.ProcessToolUse() → TasksMsg → TUI Model → Display
```

**Non-TUI Mode (Broken)**:
```
StreamProcessor.Write() → Parser → NO TaskTracker → Tasks lost
```

### Additional Issue: Tool Name Mismatch

The code has two different task systems:
- `TodoWrite` tool - Expected by `StreamProcessor.SetTodosOnly()` and `formatTodoInput()`
- `TaskCreate`/`TaskUpdate` tools - Actually used by Claude, handled by `TaskTracker`

The `--todos-only` flag filters for `TodoWrite` but Claude uses `TaskCreate`/`TaskUpdate`.

### Files Involved

| File | Issue |
|------|-------|
| `cmd/orbit-cli/root.go:249-259` | Mode selection bypasses TaskTracker for non-TUI |
| `internal/output/stream.go:37-40` | `SetTodosOnly()` incomplete, no TaskTracker |
| `internal/tui/tasks.go` | TaskTracker only used in TUI mode |
| `internal/tui/bridge.go:58-61` | Task processing only in bridge |

---

## User Story 1: Extract TaskTracker from TUI Package

**As a** developer maintaining orbit
**I want** TaskTracker to be usable outside of TUI mode
**So that** task tracking works in all output modes

### Acceptance Criteria

- [x] Move `TaskTracker` to a shared package (e.g., `internal/tasks/`)
- [x] TaskTracker has no dependencies on bubbletea or TUI
- [x] Both TUI bridge and StreamProcessor can use the same TaskTracker
- [x] TaskTracker interface allows different consumers (TUI, CLI output)

### Definition of Done

- [x] TaskTracker moved to shared package
- [x] TUI still works with extracted TaskTracker
- [x] Unit tests pass

---

## User Story 2: Add Task Tracking to StreamProcessor

**As a** developer running orbit in non-TUI mode
**I want** tasks to be tracked and displayed
**So that** I can see task progress without the TUI

### Acceptance Criteria

- [x] StreamProcessor integrates with TaskTracker
- [x] Tasks are displayed inline when created/updated
- [x] Task status changes are shown with appropriate formatting
- [x] Output includes: task subject, status indicator (✓, →, ○), and any status change

### Technical Details

StreamProcessor should:
1. Create or receive a TaskTracker instance
2. Call `ProcessToolUse()` for TaskCreate/TaskUpdate events
3. Format and display task changes inline

### Definition of Done

- [x] Tasks appear in non-TUI output
- [x] Format matches existing `formatTodoInput()` style
- [x] Unit tests for StreamProcessor task display

---

## User Story 3: Fix --todos-only Flag

**As a** developer using orbit
**I want** `--todos-only` to show TaskCreate/TaskUpdate output
**So that** I can focus on task progress only

### Acceptance Criteria

- [x] `--todos-only` filters to show only task-related output
- [x] Works with TaskCreate and TaskUpdate tools (not just TodoWrite)
- [x] Shows task subject, status, and changes
- [x] Suppresses all other Claude output (text, other tools)

### Technical Details

Current implementation filters for `TodoWrite` tool which doesn't exist. Update to filter for:
- `TaskCreate` tool uses
- `TaskUpdate` tool uses
- Optionally `TaskList` and `TaskGet` if used

### Definition of Done

- [x] `--todos-only` shows task operations
- [x] Other output is suppressed
- [x] Manual test confirms behaviour

---

## User Story 4: Display Task Summary in Non-TUI Mode

**As a** developer running orbit in non-TUI mode
**I want** to see a task summary at the end of execution
**So that** I know the final state of all tasks

### Acceptance Criteria

- [x] At end of execution, display all tracked tasks with their final status
- [x] Group tasks by status: completed, in progress, pending
- [x] Show task count summary (e.g., "3/5 tasks completed")
- [x] Use colour coding for status (green=done, yellow=in progress, grey=pending)

### Definition of Done

- [x] Summary appears at end of non-TUI execution
- [x] Task counts are accurate
- [x] Colours work (respecting NO_COLOR)

---

## User Story 5: Sync Task State Across Iterations

**As a** developer using orbit
**I want** task state to persist across loop iterations
**So that** I can track progress throughout the entire session

### Acceptance Criteria

- [x] TaskTracker maintains state across multiple Claude invocations
- [x] Tasks created in iteration 1 are visible in iteration 2
- [x] Task updates correctly modify existing tasks
- [x] Task IDs are consistent across iterations

### Definition of Done

- [x] Multi-iteration test shows persistent task state
- [x] No duplicate tasks created
- [x] Updates apply to correct tasks

---

## Technical Notes

### Current Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│ TUI Mode                                                        │
│                                                                 │
│  Bridge ──▶ TaskTracker ──▶ TasksMsg ──▶ TUI Model ──▶ Display │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ Non-TUI Mode (Currently Broken)                                 │
│                                                                 │
│  StreamProcessor ──▶ stdout (NO task tracking)                  │
└─────────────────────────────────────────────────────────────────┘
```

### Proposed Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│ Shared Task Tracking                                            │
│                                                                 │
│  internal/tasks/tracker.go                                      │
│  - ProcessToolUse(name, input) []Task                           │
│  - GetTasks() []Task                                            │
│  - GetSummary() Summary                                         │
└─────────────────────────────────────────────────────────────────┘
        │                               │
        ▼                               ▼
┌───────────────────┐         ┌───────────────────┐
│ TUI Mode          │         │ Non-TUI Mode      │
│                   │         │                   │
│ Bridge            │         │ StreamProcessor   │
│   └─▶ TasksMsg    │         │   └─▶ stdout      │
│       └─▶ Model   │         │                   │
└───────────────────┘         └───────────────────┘
```

### Tool Name Mapping

| Expected by Code | Actually Used | Action |
|------------------|---------------|--------|
| `TodoWrite` | Not used | Remove or alias |
| `TaskCreate` | Yes | Handle in StreamProcessor |
| `TaskUpdate` | Yes | Handle in StreamProcessor |
| `TaskList` | Yes | Optionally display |
| `TaskGet` | Yes | Optionally display |

### Task Display Format (Non-TUI)

```
[Task] ○ Set up project structure
[Task] → Implementing user authentication
[Task] ✓ Create database schema
```

Status indicators:
- `✓` (green) - completed
- `→` (yellow) - in_progress
- `○` (dim) - pending

### Related Files

- `internal/tui/tasks.go` → Move to `internal/tasks/tracker.go`
- `internal/output/stream.go` - Add TaskTracker integration
- `cmd/orbit-cli/root.go` - Create shared TaskTracker instance
- `internal/output/formatter.go` - Add task summary formatting
