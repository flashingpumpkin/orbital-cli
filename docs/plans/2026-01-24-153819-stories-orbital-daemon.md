# Orbital Daemon PRD

## Overview

Transform Orbital from a single-session CLI tool into a daemon-based session manager with a rich TUI. The daemon manages multiple concurrent spec sessions per project, allowing users to monitor, interact with, and control all running sessions from a unified interface.

## Goals

1. **Multi-session execution**: Run many spec sessions concurrently within a single project
2. **Unified management**: Single TUI to monitor and control all sessions
3. **Interactive exploration**: Drop into sessions to observe or interact with Claude
4. **Seamless workflow**: Auto-start daemon, natural navigation, intuitive controls
5. **Worktree integration**: Manage worktree merges from the TUI

## Non-Goals

- Remote access / multi-machine coordination (future consideration)
- Authentication/authorization (local-only, single-user)
- Session queuing or scheduling (all sessions run immediately)
- Shared budget pools across sessions

---

## Architecture

### Components

```
┌─────────────────────────────────────────────────────────────────┐
│                        Orbital Daemon                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │  Session 1  │  │  Session 2  │  │  Session N  │   ...       │
│  │  (spec-a)   │  │  (spec-b)   │  │  (spec-n)   │             │
│  │  [running]  │  │  [running]  │  │ [completed] │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    HTTP API Server                       │   │
│  │  POST /sessions      - Start new session                 │   │
│  │  GET  /sessions      - List all sessions                 │   │
│  │  GET  /sessions/:id  - Get session details               │   │
│  │  POST /sessions/:id/stop    - Stop session               │   │
│  │  POST /sessions/:id/merge   - Trigger worktree merge     │   │
│  │  GET  /sessions/:id/output  - Stream session output (SSE)│   │
│  │  POST /sessions/:id/chat    - Interactive Claude chat    │   │
│  │  POST /shutdown      - Shutdown daemon                   │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │ HTTP
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Orbital CLI / TUI Client                    │
│                                                                 │
│  orbital <spec>     → auto-start daemon if needed, start session│
│  orbital connect    → open manager TUI                          │
│  orbital status     → quick session list (no TUI)               │
│  orbital stop       → shutdown daemon                           │
└─────────────────────────────────────────────────────────────────┘
```

### Daemon Lifecycle

1. **Auto-start**: When `orbital <spec>` is run and no daemon is running, start one automatically
2. **Project-scoped**: One daemon per project (identified by `.orbital/` directory)
3. **Persistence**: Daemon runs until explicitly stopped or Ctrl+C
4. **State recovery**: On restart, daemon can resume sessions that were running

### Communication

- **Protocol**: HTTP/1.1 over Unix domain socket (`.orbital/daemon.sock`)
- **Streaming**: Server-Sent Events (SSE) for real-time output
- **Format**: JSON request/response bodies

---

## User Stories

### US-1: Start a Session with Auto-Daemon

**As a** developer
**I want to** run `orbital my-spec.md` and have it just work
**So that** I don't have to manually manage daemon lifecycle

#### Acceptance Criteria

- [ ] AC-1.1: If no daemon is running, `orbital <spec>` starts one automatically
- [ ] AC-1.2: The daemon binds to `.orbital/daemon.sock` in the project root
- [ ] AC-1.3: The session starts and output streams to the terminal
- [ ] AC-1.4: A PID file is created at `.orbital/daemon.pid`
- [ ] AC-1.5: Subsequent `orbital <spec>` commands connect to the existing daemon
- [ ] AC-1.6: If daemon is unhealthy, it is restarted automatically
- [ ] AC-1.7: The terminal shows a brief "Connected to daemon" message on auto-start

#### Technical Notes

- Check for daemon: attempt connection to socket with short timeout
- Daemon startup: fork background process, wait for socket to be ready
- Health check: GET `/health` endpoint with 1s timeout

---

### US-2: Connect to Manager TUI

**As a** developer
**I want to** run `orbital connect` to see all my running sessions
**So that** I can monitor and manage multiple concurrent tasks

#### Acceptance Criteria

- [ ] AC-2.1: `orbital connect` opens the manager TUI
- [ ] AC-2.2: The TUI shows a tree view of all sessions grouped by status:
  ```
  ┌─ Orbital Session Manager ─────────────────────────────────┐
  │                                                           │
  │  ▼ Running (3)                                            │
  │    ├─ auth-feature.md      [iter 5/50]  $2.34   ●●●○○    │
  │    ├─ api-refactor.md      [iter 12/50] $8.21   ●●●●○    │
  │    └─ fix-bug-123.md       [iter 2/50]  $0.45   ●○○○○    │
  │                                                           │
  │  ▶ Completed (2)                                          │
  │                                                           │
  │  ▶ Failed (1)                                             │
  │                                                           │
  │ ─────────────────────────────────────────────────────────│
  │  [Enter] View  [i] Interactive  [s] Stop  [m] Merge      │
  │  [c] Chat      [q] Quit         [?] Help                 │
  └───────────────────────────────────────────────────────────┘
  ```
- [ ] AC-2.3: Arrow keys navigate the tree (up/down to move, left/right to collapse/expand)
- [ ] AC-2.4: Status shows: spec file, iteration progress, cost, visual progress bar
- [ ] AC-2.5: Sessions update in real-time (iteration count, cost, status changes)
- [ ] AC-2.6: Completed sessions move to "Completed" section automatically
- [ ] AC-2.7: Failed sessions (max iterations, budget exceeded) move to "Failed" section
- [ ] AC-2.8: If no daemon is running, show error: "No daemon running. Start a session with `orbital <spec>`"

#### Technical Notes

- Use Bubbletea for TUI
- Poll `/sessions` every 500ms for updates (or use SSE for push)
- Tree state (collapsed/expanded) is local to the TUI instance

---

### US-3: View Session Output

**As a** developer
**I want to** select a session and view its live output
**So that** I can see what Claude is doing in real-time

#### Acceptance Criteria

- [ ] AC-3.1: Pressing Enter on a session opens the session view
- [ ] AC-3.2: The session view shows:
  ```
  ┌─ auth-feature.md ─────────────────────────────────────────┐
  │ Status: Running   Iteration: 5/50   Cost: $2.34          │
  │ Worktree: swift-falcon   Branch: orbital/auth-feature    │
  ├───────────────────────────────────────────────────────────┤
  │                                                           │
  │ Claude: I'll implement the authentication middleware...   │
  │                                                           │
  │ [Reading src/auth/middleware.go]                         │
  │                                                           │
  │ The current implementation uses JWT tokens. I'll add     │
  │ refresh token support as specified...                    │
  │                                                           │
  │ [Editing src/auth/middleware.go]                         │
  │ + func (a *Auth) RefreshToken(ctx context.Context) {     │
  │ +     // Validate existing token...                      │
  │                                                           │
  ├───────────────────────────────────────────────────────────┤
  │ [b] Back  [s] Stop  [m] Merge  [c] Chat  [p] Pause       │
  └───────────────────────────────────────────────────────────┘
  ```
- [ ] AC-3.3: Output scrolls automatically (following mode)
- [ ] AC-3.4: Scroll up disables auto-follow; pressing `f` or `End` re-enables
- [ ] AC-3.5: Pressing `b` or `Esc` returns to the manager view
- [ ] AC-3.6: The session continues running when navigating away
- [ ] AC-3.7: Output is buffered (last 10,000 lines) so history is visible on connect
- [ ] AC-3.8: Tool calls (Read, Edit, Bash) are formatted distinctly

#### Technical Notes

- Use SSE endpoint `/sessions/:id/output` for streaming
- Maintain ring buffer of output lines per session in daemon
- Format stream-json events into readable output (reuse existing formatter)

---

### US-4: Stop a Running Session

**As a** developer
**I want to** stop a session that's going in the wrong direction
**So that** I can save budget and start fresh

#### Acceptance Criteria

- [ ] AC-4.1: Pressing `s` on a running session prompts for confirmation
- [ ] AC-4.2: Confirmation dialog: "Stop session 'auth-feature.md'? [y/N]"
- [ ] AC-4.3: On confirm, session is gracefully stopped (SIGINT to Claude process)
- [ ] AC-4.4: Session state is preserved (can be resumed with `orbital continue`)
- [ ] AC-4.5: Session moves to "Stopped" status (not Failed)
- [ ] AC-4.6: Stop is immediate (within 2 seconds)
- [ ] AC-4.7: If session is in verification/merge phase, wait for phase to complete

#### Technical Notes

- POST `/sessions/:id/stop` triggers context cancellation
- Executor respects context and terminates Claude process
- State is saved via existing iteration callback

---

### US-5: Interactive Chat Session

**As a** developer
**I want to** chat with Claude about a session's specs and context
**So that** I can iterate on requirements without stopping the main session

#### Acceptance Criteria

- [ ] AC-5.1: Pressing `c` on any session opens an interactive chat
- [ ] AC-5.2: The chat is a separate Claude instance (not the running session)
- [ ] AC-5.3: The chat has access to:
  - The spec file(s) for that session
  - The notes file (if configured)
  - Context files (if configured)
  - Session output history
- [ ] AC-5.4: The chat UI shows:
  ```
  ┌─ Chat: auth-feature.md ───────────────────────────────────┐
  │ Context: spec.md, notes.md, src/auth/*                   │
  ├───────────────────────────────────────────────────────────┤
  │                                                           │
  │ You: Can we simplify the refresh token logic?            │
  │                                                           │
  │ Claude: Looking at the spec, the refresh token flow      │
  │ has three main steps. We could simplify by...            │
  │                                                           │
  │ You: Update the spec to reflect that                     │
  │                                                           │
  │ Claude: I'll update auth-feature.md to simplify the      │
  │ refresh token section...                                 │
  │ [Editing auth-feature.md]                                │
  │                                                           │
  ├───────────────────────────────────────────────────────────┤
  │ > _                                                       │
  └───────────────────────────────────────────────────────────┘
  ```
- [ ] AC-5.5: Claude can edit the spec files during chat
- [ ] AC-5.6: Changes to spec files are picked up by running session on next iteration
- [ ] AC-5.7: Chat history is preserved for the duration of the daemon
- [ ] AC-5.8: Pressing `Esc` or `q` exits chat and returns to previous view
- [ ] AC-5.9: Chat has its own independent budget (configurable, default $10)

#### Technical Notes

- Spawn separate Claude process with `--resume` for chat continuity
- System prompt includes: "You are helping iterate on specs for an Orbital session"
- Store chat session ID in session state for resume

---

### US-6: Trigger Worktree Merge

**As a** developer
**I want to** trigger a merge from a completed worktree session
**So that** I can integrate the work back into my main branch

#### Acceptance Criteria

- [ ] AC-6.1: Completed worktree sessions show `[m] Merge` option
- [ ] AC-6.2: Pressing `m` prompts: "Merge branch 'orbital/auth-feature' into 'main'? [y/N]"
- [ ] AC-6.3: Merge runs in the background (non-blocking)
- [ ] AC-6.4: Session status changes to "Merging"
- [ ] AC-6.5: User can observe merge progress in session view
- [ ] AC-6.6: On success, status changes to "Merged" with summary
- [ ] AC-6.7: On conflict, status changes to "Merge Conflict" with instructions
- [ ] AC-6.8: Merged sessions show branch and commit info
- [ ] AC-6.9: Worktree is cleaned up after successful merge

#### Technical Notes

- Reuse existing `worktree.Merge()` functionality
- Add "Merging" and "Merged" statuses to session state
- Store merge result (success/conflict) in session state

---

### US-7: Shutdown Daemon

**As a** developer
**I want to** cleanly shutdown the daemon
**So that** all sessions are properly saved

#### Acceptance Criteria

- [ ] AC-7.1: `orbital stop` sends shutdown signal to daemon
- [ ] AC-7.2: Daemon gracefully stops all running sessions (preserving state)
- [ ] AC-7.3: All sessions are saved and can be resumed later
- [ ] AC-7.4: Socket file is removed on clean shutdown
- [ ] AC-7.5: PID file is removed on clean shutdown
- [ ] AC-7.6: If sessions are running, prompt: "3 sessions running. Stop all? [y/N]"
- [ ] AC-7.7: `--force` flag skips confirmation
- [ ] AC-7.8: Ctrl+C in manager TUI does NOT stop daemon (just exits TUI)
- [ ] AC-7.9: Ctrl+C in daemon process (if attached) triggers graceful shutdown

#### Technical Notes

- POST `/shutdown` with optional `force` parameter
- Daemon sets shutdown flag, waits for sessions to save (max 30s)
- Sessions receive context cancellation

---

### US-8: Quick Status Check

**As a** developer
**I want to** quickly see session status without entering the TUI
**So that** I can script or quickly check progress

#### Acceptance Criteria

- [ ] AC-8.1: `orbital status` prints a summary table:
  ```
  Orbital Daemon (PID 12345)

  RUNNING (3)
    auth-feature.md      iter 5/50    $2.34    2m ago
    api-refactor.md      iter 12/50   $8.21    15m ago
    fix-bug-123.md       iter 2/50    $0.45    30s ago

  COMPLETED (2)
    user-profile.md      iter 8/50    $3.21    1h ago
    search-index.md      iter 23/50   $12.45   2h ago

  Total cost: $26.66
  ```
- [ ] AC-8.2: `orbital status --json` outputs JSON for scripting
- [ ] AC-8.3: `orbital status <session-id>` shows detailed single-session info
- [ ] AC-8.4: Exit code 0 if daemon running, 1 if not

#### Technical Notes

- GET `/sessions` with optional `format=json` query param
- Reuse existing status.go structure, extend for daemon mode

---

### US-9: Resume Sessions After Daemon Restart

**As a** developer
**I want to** resume sessions that were running when the daemon stopped
**So that** I don't lose progress on unexpected shutdowns

#### Acceptance Criteria

- [ ] AC-9.1: On startup, daemon scans `.orbital/sessions/` for saved state
- [ ] AC-9.2: Sessions with status "running" are shown as "Interrupted"
- [ ] AC-9.3: User can select interrupted sessions and resume them
- [ ] AC-9.4: Pressing `r` on interrupted session resumes from last iteration
- [ ] AC-9.5: Resumed sessions continue with same session ID and history
- [ ] AC-9.6: Interrupted sessions are NOT auto-resumed (require explicit action)

#### Technical Notes

- Store per-session state in `.orbital/sessions/<session-id>/state.json`
- Session state includes: spec files, iteration count, Claude session ID, output buffer
- On resume, use `--resume <claude-session-id>` to continue Claude session

---

### US-10: Multiple TUI Connections

**As a** developer
**I want to** connect from multiple terminals
**So that** I can monitor different sessions on different screens

#### Acceptance Criteria

- [ ] AC-10.1: Multiple `orbital connect` instances can run simultaneously
- [ ] AC-10.2: All instances see the same session list and statuses
- [ ] AC-10.3: Actions in one instance (stop, merge) are reflected in others
- [ ] AC-10.4: Each instance can view different sessions independently
- [ ] AC-10.5: Chat sessions are per-connection (not shared)
- [ ] AC-10.6: No locking or "already connected" errors

#### Technical Notes

- Each TUI connection is independent HTTP client
- SSE streams are per-connection
- Chat state is stored per-connection in client, not daemon

---

### US-11: Session Notifications

**As a** developer
**I want to** be notified when sessions complete or fail
**So that** I can take action without constantly watching

#### Acceptance Criteria

- [ ] AC-11.1: Completed sessions trigger a desktop notification (if supported)
- [ ] AC-11.2: Failed sessions trigger a notification with failure reason
- [ ] AC-11.3: Notifications include session name and summary
- [ ] AC-11.4: Notifications are configurable (on/off) in `.orbital/config.toml`
- [ ] AC-11.5: In TUI, status bar shows "Session completed" flash message
- [ ] AC-11.6: Sound notification (optional, configurable)

#### Technical Notes

- Use `beeep` or similar for cross-platform notifications
- Config: `[notifications] enabled = true, sound = false`
- TUI flash messages auto-dismiss after 5 seconds

---

### US-12: Start Session from TUI

**As a** developer
**I want to** start new sessions from within the manager TUI
**So that** I don't have to exit and run a new command

#### Acceptance Criteria

- [ ] AC-12.1: Pressing `n` opens "New Session" dialog
- [ ] AC-12.2: Dialog shows file picker for spec files:
  ```
  ┌─ New Session ─────────────────────────────────────────────┐
  │ Select spec file:                                        │
  │                                                           │
  │   > specs/auth-feature.md                                │
  │     specs/api-v2.md                                      │
  │     specs/performance.md                                 │
  │     [Browse...]                                          │
  │                                                           │
  │ Options:                                                  │
  │   [x] Use worktree isolation                             │
  │   [ ] Use TDD workflow                                   │
  │   Budget: $50.00                                         │
  │                                                           │
  │ [Enter] Start  [Esc] Cancel                              │
  └───────────────────────────────────────────────────────────┘
  ```
- [ ] AC-12.3: Shows recent/common spec file locations
- [ ] AC-12.4: Can configure worktree mode, workflow preset, budget
- [ ] AC-12.5: Starting session returns to manager view with new session visible
- [ ] AC-12.6: Glob pattern support for finding spec files (`specs/**/*.md`)

#### Technical Notes

- File picker uses Glob internally
- Remember last-used settings in client-side state
- POST `/sessions` with configuration payload

---

## Data Model

### Session State

```go
type Session struct {
    ID             string           `json:"id"`              // Unique 8-byte hex
    SpecFiles      []string         `json:"spec_files"`      // Spec file paths
    Status         SessionStatus    `json:"status"`          // running, completed, failed, stopped, merging, merged
    WorkingDir     string           `json:"working_dir"`     // Project directory
    Worktree       *WorktreeInfo    `json:"worktree"`        // Worktree details (if enabled)
    Iteration      int              `json:"iteration"`       // Current iteration
    MaxIterations  int              `json:"max_iterations"`  // Configured max
    TotalCost      float64          `json:"total_cost"`      // Cumulative USD
    MaxBudget      float64          `json:"max_budget"`      // Configured budget
    StartedAt      time.Time        `json:"started_at"`
    CompletedAt    *time.Time       `json:"completed_at"`    // When finished
    ClaudeSession  string           `json:"claude_session"`  // Claude CLI session ID
    ChatSession    string           `json:"chat_session"`    // Interactive chat session ID
    Error          string           `json:"error"`           // Error message if failed
    Workflow       *WorkflowState   `json:"workflow"`        // Workflow progress
    NotesFile      string           `json:"notes_file"`
    ContextFiles   []string         `json:"context_files"`
}

type SessionStatus string

const (
    StatusRunning     SessionStatus = "running"
    StatusCompleted   SessionStatus = "completed"
    StatusFailed      SessionStatus = "failed"
    StatusStopped     SessionStatus = "stopped"
    StatusMerging     SessionStatus = "merging"
    StatusMerged      SessionStatus = "merged"
    StatusInterrupted SessionStatus = "interrupted"
    StatusConflict    SessionStatus = "conflict"
)

type WorktreeInfo struct {
    Name           string `json:"name"`
    Path           string `json:"path"`
    Branch         string `json:"branch"`
    OriginalBranch string `json:"original_branch"`
}
```

### Daemon State

```go
type DaemonState struct {
    PID        int                  `json:"pid"`
    StartedAt  time.Time            `json:"started_at"`
    ProjectDir string               `json:"project_dir"`
    Sessions   map[string]*Session  `json:"sessions"`
    Config     *DaemonConfig        `json:"config"`
}

type DaemonConfig struct {
    NotificationsEnabled bool    `json:"notifications_enabled"`
    NotificationSound    bool    `json:"notification_sound"`
    DefaultBudget        float64 `json:"default_budget"`
    DefaultWorkflow      string  `json:"default_workflow"`
    DefaultWorktree      bool    `json:"default_worktree"`
}
```

---

## API Specification

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/sessions` | List all sessions |
| POST | `/sessions` | Start new session |
| GET | `/sessions/:id` | Get session details |
| DELETE | `/sessions/:id` | Stop session |
| POST | `/sessions/:id/merge` | Trigger worktree merge |
| GET | `/sessions/:id/output` | Stream output (SSE) |
| POST | `/sessions/:id/chat` | Send chat message |
| GET | `/sessions/:id/chat` | Stream chat responses (SSE) |
| POST | `/shutdown` | Shutdown daemon |

### Example Requests

```bash
# Start session
curl -X POST http://localhost/sessions \
  -H "Content-Type: application/json" \
  -d '{"spec_files": ["auth-feature.md"], "worktree": true, "budget": 50.0}'

# List sessions
curl http://localhost/sessions

# Stream output
curl -N http://localhost/sessions/abc123/output

# Stop session
curl -X DELETE http://localhost/sessions/abc123

# Shutdown daemon
curl -X POST http://localhost/shutdown
```

---

## File Structure Changes

```
.orbital/
├── config.toml              # Existing config (add daemon settings)
├── daemon.sock              # Unix socket (new)
├── daemon.pid               # PID file (new)
├── daemon-state.json        # Daemon state (new)
├── sessions/                # Per-session state (new)
│   ├── abc123/
│   │   ├── state.json       # Session state
│   │   ├── output.log       # Output buffer
│   │   └── chat-history.json # Chat history
│   └── def456/
│       └── ...
├── worktree-state.json      # Existing
└── state/                   # Legacy (single-session) - deprecated
    └── state.json
```

---

## Configuration Extensions

```toml
# .orbital/config.toml

# Existing settings...

[daemon]
# Auto-start daemon when running orbital commands
auto_start = true

# Default settings for new sessions
default_budget = 50.0
default_workflow = "spec-driven"
default_worktree = false

[notifications]
enabled = true
sound = false
# Custom command to run on completion (optional)
on_complete = "say 'Orbital session completed'"

[chat]
# Budget for interactive chat sessions
budget = 10.0
# Model for chat (can differ from main sessions)
model = "sonnet"
```

---

## Implementation Phases

### Phase 1: Core Daemon Infrastructure
- Daemon process lifecycle (start, stop, health check)
- HTTP API server with Unix socket
- Session registry and state persistence
- Auto-start on `orbital <spec>` command
- `orbital stop` command

### Phase 2: Basic TUI Manager
- Session list view with status grouping
- Real-time status updates
- Session output viewing
- Navigation (tree view, session details)
- Stop session action

### Phase 3: Session Management
- Start session from TUI
- Multiple TUI connections
- Session state recovery on daemon restart
- `orbital status` CLI command

### Phase 4: Interactive Features
- Interactive chat with Claude
- Worktree merge triggering
- Merge progress observation
- Desktop notifications

### Phase 5: Polish & Configuration
- Configuration file extensions
- Keyboard shortcut help
- Error handling improvements
- Performance optimization

---

## Success Metrics

1. **Usability**: User can manage 5+ concurrent sessions without confusion
2. **Reliability**: Daemon runs for 24+ hours without memory leaks or crashes
3. **Responsiveness**: TUI updates within 200ms of state changes
4. **Recovery**: Sessions can be resumed after daemon restart with no data loss

---

## Open Questions

1. **Session retention**: How long to keep completed sessions before cleanup?
   - Proposal: Keep indefinitely until explicit clear, or configurable retention (e.g., 7 days)

2. **Resource limits**: Should there be optional limits on concurrent sessions?
   - Proposal: No hard limits, but show warning if >10 sessions running

3. **Log persistence**: Should output logs persist after daemon restart?
   - Proposal: Yes, store in `.orbital/sessions/<id>/output.log`

4. **Cross-project visibility**: Should `orbital status` show sessions from other projects?
   - Proposal: No, keep project-scoped. Add `orbital status --global` later if needed.

---

## Appendix: TUI Navigation Reference

| Context | Key | Action |
|---------|-----|--------|
| Manager | ↑/↓ | Navigate sessions |
| Manager | ←/→ | Collapse/expand group |
| Manager | Enter | View session output |
| Manager | n | New session |
| Manager | s | Stop selected session |
| Manager | m | Merge (if applicable) |
| Manager | c | Open chat |
| Manager | r | Resume (if interrupted) |
| Manager | q | Quit TUI |
| Manager | ? | Show help |
| Session View | b/Esc | Back to manager |
| Session View | f/End | Follow output |
| Session View | s | Stop session |
| Session View | c | Open chat |
| Session View | m | Merge (if applicable) |
| Chat | Esc/q | Exit chat |
| Chat | Enter | Send message |
| Chat | ↑/↓ | Scroll history |
| All | Ctrl+C | Exit TUI (not daemon) |
