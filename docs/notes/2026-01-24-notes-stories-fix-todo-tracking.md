# Notes: Fix TODO Tracking

## Changes Made

### User Story 1: Extract TaskTracker from TUI Package
- Created `internal/tasks/tracker.go` with the shared `Tracker` type
- The tracker supports `TaskCreate`, `TaskUpdate`, and `TodoWrite` tools
- Added `IsTaskTool()` helper function to identify task-related tools
- Added `GetSummary()` method to get task count statistics
- Updated TUI package to use the shared tasks package via type alias

### User Story 2: Add Task Tracking to StreamProcessor
- StreamProcessor now integrates with `tasks.Tracker`
- Added `SetTracker()` method to allow sharing tracker across iterations
- Added `GetTracker()` method for external access
- Tasks are displayed inline when created/updated with `[Task]` prefix
- Status indicators: `✓` (completed), `→` (in_progress), `○` (pending)

### User Story 3: Fix --todos-only Flag
- Updated `printAssistant()` to use `tasks.IsTaskTool()` for filtering
- Now correctly shows `TaskCreate`, `TaskUpdate`, and `TodoWrite` output
- Suppresses all non-task tool output in todos-only mode

### User Story 4: Display Task Summary in Non-TUI Mode
- Added `PrintTaskSummary()` method to StreamProcessor
- Shows task count summary (e.g., "Tasks: 2/5 completed")
- Groups tasks by status: in-progress first, then pending, then completed
- Uses colour coding for status indicators

### User Story 5: Sync Task State Across Iterations
- Created shared `taskTracker` in root.go
- StreamProcessor uses the shared tracker via `SetTracker()`
- Tasks persist across iterations in the same session

## Technical Decisions

1. **Type alias approach**: Used `type Task = tasks.Task` in TUI to maintain backward compatibility while sharing the underlying type.

2. **TodoWrite handling**: The `TodoWrite` tool replaces all tasks (it's a full list replacement), while `TaskCreate`/`TaskUpdate` are incremental operations.

3. **Summary display order**: Tasks are shown in priority order: in-progress (most important to see), pending, then completed.

## Testing

All existing tests pass plus new tests added:
- `internal/tasks/tracker_test.go` - Full coverage of tracker functionality
- `internal/output/stream_test.go` - Tests for task tracking integration, todos-only mode, and summary printing
