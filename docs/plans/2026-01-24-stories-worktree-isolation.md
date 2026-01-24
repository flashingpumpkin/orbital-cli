# User Stories: Worktree Isolation

## Overview

Add a `--worktree` flag that runs orbit in three phases:

1. **Setup**: Claude creates a worktree with a descriptive branch name
2. **Work**: The normal orbit loop runs in the worktree
3. **Merge**: Claude rebases and merges changes back to the original branch

State is persisted so `orbit continue` can resume interrupted worktree sessions.

---

## User Story 1: Setup Phase - Claude Creates Worktree

**As a** developer using orbit
**I want** orbit to invoke Claude to create a worktree before the main loop starts
**So that** the branch name is descriptive and relevant to the task

### Acceptance Criteria

- [x] `--worktree` flag added to CLI
- [x] When flag is set, orbit runs a separate Claude invocation before the loop
- [ ] Claude is given the spec content and asked to:
  1. Understand the task
  2. Choose a descriptive kebab-case name
  3. Create worktree at `.orbit/worktrees/<name>` with branch `orbit/<name>`
  4. Output the worktree path
- [ ] Orbit captures the worktree path from Claude's output
- [ ] If worktree creation fails, orbit exits with error
- [ ] `--worktree-name <name>` flag allows user to override Claude's name choice
- [ ] `--setup-model` flag configures which model to use for setup phase
- [ ] Setup phase cost does not count towards `--max-budget`

### Definition of Done

- [ ] Unit tests pass
- [ ] Setup phase implemented as separate executor call
- [ ] Error handling for non-git repositories
- [ ] Name override flag working

---

## User Story 2: Persist Worktree State

**As a** developer using orbit
**I want** orbit to persist worktree state
**So that** I can resume interrupted sessions with `orbit continue`

### Acceptance Criteria

- [ ] State stored in `.orbit/` directory
- [ ] State includes: worktree path, original branch, session ID, spec file path(s)
- [ ] Multiple worktrees can be tracked simultaneously (parallel instances)
- [ ] State uses spec filename as the work description
- [ ] State is created after successful setup phase
- [ ] State is removed after successful merge and cleanup

### Definition of Done

- [ ] State persistence implemented
- [ ] Unit tests for state read/write
- [ ] Multiple concurrent worktrees supported

---

## User Story 3: Work Phase - Execute Loop in Worktree

**As a** developer using orbit
**I want** the orbit loop to execute entirely within the created worktree
**So that** all changes are isolated from my main working directory

### Acceptance Criteria

- [ ] After setup phase, orbit sets `WorkingDir` to the worktree path
- [ ] All loop iterations run with worktree as working directory
- [ ] Session is created and resumed within the worktree context
- [ ] Progress output shows worktree path

### Definition of Done

- [ ] Unit tests pass
- [ ] Manual verification that changes appear only in worktree

---

## User Story 4: Merge Phase - Claude Rebases and Merges

**As a** developer using orbit
**I want** orbit to invoke Claude to rebase and merge when the loop completes
**So that** my changes integrate cleanly without merge commits

### Acceptance Criteria

- [ ] On successful completion (promise detected), orbit runs a separate Claude invocation
- [ ] Claude is instructed to:
  1. Rebase the worktree branch onto the original branch
  2. Resolve any conflicts that arise
  3. Fast-forward merge the rebased changes into the original branch
- [ ] No merge commits are created
- [ ] `--merge-model` flag configures which model to use for merge phase
- [ ] Merge phase cost does not count towards `--max-budget`
- [ ] If merge ultimately fails, orbit preserves the worktree and exits with error

### Definition of Done

- [ ] Unit tests for merge phase
- [ ] Integration test for full workflow
- [ ] Conflict resolution by Claude working

---

## User Story 5: Cleanup Worktree on Success

**As a** developer using orbit
**I want** orbit to automatically clean up the worktree after successful merge
**So that** I don't accumulate stale worktrees

### Acceptance Criteria

- [ ] After successful merge, the worktree is removed
- [ ] The worktree branch (`orbit/<name>`) is deleted
- [ ] State entry is removed
- [ ] Cleanup happens automatically, no flag needed

### Definition of Done

- [ ] Unit tests pass
- [ ] Cleanup verified in integration tests

---

## User Story 6: Preserve Worktree on Failure/Interrupt

**As a** developer using orbit
**I want** worktrees to be preserved when orbit is interrupted or fails
**So that** I can resume with `orbit continue`

### Acceptance Criteria

- [ ] On SIGINT/SIGTERM, worktree is not deleted
- [ ] On any error, worktree is not deleted
- [ ] State remains intact for resume capability
- [ ] Message printed with worktree location and how to continue

### Definition of Done

- [ ] Signal handler updated
- [ ] Manual test confirms behaviour

---

## User Story 7: Continue Worktree Session

**As a** developer using orbit
**I want** to resume an interrupted worktree session with `orbit continue`
**So that** I can complete work that was interrupted

### Acceptance Criteria

- [ ] `orbit continue <spec-file>` finds the worktree associated with that spec
- [ ] If multiple worktrees exist for the same spec, show TUI to choose
- [ ] `orbit continue` (no spec file) with one active worktree auto-continues it
- [ ] `orbit continue` (no spec file) with multiple worktrees shows TUI to choose
- [ ] TUI displays spec filename as work description
- [ ] Resumed session continues in the worktree with existing Claude session

### Definition of Done

- [ ] Continue command updated for worktree support
- [ ] TUI selection implemented
- [ ] Unit tests for state lookup logic

---

## User Story 8: Handle Name Collisions

**As a** developer using orbit
**I want** Claude to handle worktree name collisions
**So that** setup doesn't fail if a name is already taken

### Acceptance Criteria

- [ ] During setup, Claude checks if chosen name already exists
- [ ] If collision, Claude picks a different name (e.g., appends suffix)
- [ ] Setup phase instructions tell Claude to handle this case

### Definition of Done

- [ ] Setup prompt includes collision handling instructions
- [ ] Manual test confirms Claude handles collisions

---

## Technical Notes

### Directory Structure

```
.orbit/
├── config.toml           # Existing config
├── state.json            # Worktree state tracking
└── worktrees/
    ├── add-user-auth/    # Worktree directory
    └── fix-payment-bug/  # Another worktree
```

### State Schema

```json
{
  "worktrees": [
    {
      "path": ".orbit/worktrees/add-user-auth",
      "branch": "orbit/add-user-auth",
      "originalBranch": "main",
      "specFiles": ["docs/plans/user-auth.md"],
      "sessionId": "abc123",
      "createdAt": "2026-01-24T10:30:00Z"
    }
  ]
}
```

### Branch Naming Convention

```
orbit/<descriptive-name>
```

Examples: `orbit/add-user-auth`, `orbit/fix-checkout-validation`

### Three-Phase Execution Flow

```
┌─────────────────────────────────────────────────────┐
│ Phase 1: Setup (separate Claude invocation)         │
│   Model: --setup-model (configurable)               │
│   Cost: Not counted towards --max-budget            │
│   → Creates .orbit/worktrees/<name>                 │
│   → Creates branch orbit/<name>                     │
│   → Persists state                                  │
└─────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│ Phase 2: Work Loop                                  │
│   WorkingDir = .orbit/worktrees/<name>              │
│   Normal orbit loop until promise detected          │
│   Cost: Counts towards --max-budget                 │
└─────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│ Phase 3: Merge (separate Claude invocation)         │
│   Model: --merge-model (configurable)               │
│   Cost: Not counted towards --max-budget            │
│   → Rebases orbit/<name> onto original branch       │
│   → Resolves conflicts if any                       │
│   → Fast-forward merges                             │
│   → Cleans up worktree and branch                   │
│   → Removes state entry                             │
└─────────────────────────────────────────────────────┘
```

### New CLI Flags

| Flag | Description |
|------|-------------|
| `--worktree` | Enable worktree isolation mode |
| `--worktree-name <name>` | Override Claude's branch name choice |
| `--setup-model <model>` | Model for setup phase |
| `--merge-model <model>` | Model for merge phase |

### Git Commands (executed by Claude)

```bash
# Setup phase
git worktree add -b orbit/<name> .orbit/worktrees/<name> HEAD

# Merge phase
cd .orbit/worktrees/<name>
git rebase <original-branch>
# resolve conflicts if needed
cd <main-repo>
git checkout <original-branch>
git merge --ff-only orbit/<name>

# Cleanup (executed by orbit after successful merge)
git worktree remove .orbit/worktrees/<name>
git branch -d orbit/<name>
```

### Error Scenarios

| Scenario | Behaviour |
|----------|-----------|
| Not a git repository | Exit with error before setup |
| Setup phase fails | Exit with error, no state created |
| Work loop interrupted | Preserve worktree and state, print continue instructions |
| Merge conflicts | Claude attempts resolution |
| Merge fails after retries | Preserve worktree and state, exit with error |
| Worktree name exists | Claude picks alternative name |

### New Package Structure

```
internal/
├── worktree/
│   ├── worktree.go       # Setup, merge, cleanup orchestration
│   ├── state.go          # State persistence
│   ├── prompts.go        # Setup and merge phase prompts
│   └── worktree_test.go
```
