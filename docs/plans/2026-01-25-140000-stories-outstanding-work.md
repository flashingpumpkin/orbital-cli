# User Stories: Outstanding Work

**Date**: 2026-01-25
**Status**: Backlog

## Overview

Consolidated list of outstanding work items verified against the codebase. Each item represents genuinely missing functionality (not stale checkboxes from completed work).

## Dependency Graph

```
                    ┌─────────────────────┐
                    │  Wire Resume        │
                    │  Validation (XS)    │
                    └─────────────────────┘

                    ┌─────────────────────┐
                    │  Default Agents     │
                    │  Config (S)         │
                    └─────────────────────┘

                    ┌─────────────────────┐
                    │  Worktree List      │
                    │  Command (S)        │
                    └──────────┬──────────┘
                               │
           ┌───────────────────┼───────────────────┐
           │                   │                   │
           ▼                   ▼                   ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  Worktree Show  │  │ Worktree Remove │  │ Worktree Cleanup│
│  Command (S)    │  │ Command (S)     │  │ Command (M)     │
└─────────────────┘  └─────────────────┘  └────────┬────────┘
                                                   │
                                                   ▼
                                         ┌─────────────────┐
                                         │ Selection on    │
                                         │ Resume (S)      │
                                         └─────────────────┘
```

## Epic 1: Worktree Resume Robustness

### [x] **Ticket 1.1: Wire ValidateWorktree into continue command**

**As a** developer resuming a worktree session
**I want** the worktree to be validated before resuming
**So that** I get clear errors instead of cryptic failures

**Context**: The `ValidateWorktree()` function exists at `internal/worktree/state.go:441-470` but is never called. The continue command at `cmd/orbital/continue.go` uses the worktree path from state without verification.

**Description**: Call `ValidateWorktree()` in the continue command before setting `effectiveWorkingDir`. If validation fails, display the error and suggest running cleanup.

**Implementation Requirements**:
- Import worktree package in continue.go if not present
- Call `worktree.ValidateWorktree(wtState)` before using the worktree
- Display validation errors with remediation instructions
- Suggest `orbital worktree cleanup` when worktree is missing

**Acceptance Criteria**:
- [x] Given worktree path does not exist, when `orbital continue` runs, then error shows "Worktree directory not found"
- [x] Given worktree is not a git worktree, when `orbital continue` runs, then error explains the issue
- [x] Given worktree is valid, when `orbital continue` runs, then resume proceeds normally

**Definition of Done**:
- [x] Validation call added to continue.go
- [x] Error messages include remediation steps
- [x] All existing tests pass

**Note**: Already implemented at `cmd/orbital/continue.go:71-73`

**Dependencies**: None

**Files to Modify**:
- `cmd/orbital/continue.go`

**Effort Estimate**: XS

---

## Epic 2: Default Agent Configuration

### [x] **Ticket 2.1: Add default agents constant**

**As a** developer using Orbital CLI
**I want** the `general-purpose` agent included by default
**So that** Claude has exploration capabilities without manual configuration

**Context**: Currently agents are only passed when explicitly configured via TOML or CLI flag. The `general-purpose` built-in agent should be available by default.

**Description**: Add a `DefaultAgents` map in `internal/config/agents.go` containing the `general-purpose` agent definition.

**Implementation Requirements**:
- Add `DefaultAgents map[string]Agent` variable
- Include `general-purpose` with appropriate description and prompt
- Research Claude CLI documentation for built-in agent reference format

**Acceptance Criteria**:
- [x] Given no agent configuration, when inspecting DefaultAgents, then `general-purpose` is present
- [x] Given the code compiles, when running `go build ./...`, then no errors

**Definition of Done**:
- [x] DefaultAgents constant added to agents.go
- [x] Unit test verifies default includes general-purpose
- [x] Code passes `go test ./...`

**Dependencies**: None

**Files to Modify**:
- `internal/config/agents.go`

**Effort Estimate**: XS

---

### [x] **Ticket 2.2: Merge default agents with user config**

**As a** developer using Orbital CLI
**I want** my custom agents merged with defaults
**So that** I get both built-in and custom configurations

**Context**: Users may configure additional agents. These should merge with defaults, with user config taking precedence on name conflicts.

**Description**: Modify agent configuration loading to merge `DefaultAgents` with user-provided agents.

**Implementation Requirements**:
- Modify `AgentsToJSON` or config loading to merge defaults
- User agents override defaults with same name
- Handle both TOML and CLI flag sources

**Acceptance Criteria**:
- [x] Given no user config, when Orbital runs, then default agents are passed to Claude CLI
- [x] Given user agents in config, when Orbital runs, then both user and default agents are passed
- [x] Given user agent with same name as default, when Orbital runs, then user agent takes precedence

**Definition of Done**:
- [x] Merge logic implemented
- [x] Unit tests cover merge scenarios
- [x] Code passes `go test ./...`

**Dependencies**: Ticket 2.1

**Files to Modify**:
- `internal/config/agents.go`
- `cmd/orbital/root.go`

**Effort Estimate**: S

---

## Epic 3: Multi-Worktree Management

### [x] **Ticket 3.1: Add worktree list command**

**As a** developer
**I want** to see all tracked worktrees
**So that** I can understand what's active and identify issues

**Context**: No way to inspect worktree state without reading JSON manually. This is the foundation for other worktree commands.

**Description**: Add `orbital worktree list` command that displays all tracked worktrees with their status.

**Implementation Requirements**:
- Create `cmd/orbital/worktree.go` with parent command
- Add `list` subcommand showing: name, path, branch, original branch, spec files, status
- Check if paths exist on disk (mark MISSING if not)
- Check if registered with git (mark ORPHAN if not)
- Support `--json` flag for machine-readable output

**Acceptance Criteria**:
- [x] Given worktrees exist, when `orbital worktree list` runs, then all are shown with status
- [x] Given worktree path doesn't exist, when listed, then marked as "MISSING"
- [x] Given `--json` flag, when list runs, then output is valid JSON
- [x] Given no worktrees, when list runs, then message says "No active worktrees"

**Definition of Done**:
- [x] `orbital worktree list` command implemented
- [x] Status detection for MISSING/ORPHAN
- [x] JSON output option
- [x] Tests for list command

**Dependencies**: None

**Files to Create**:
- `cmd/orbital/worktree_cmd.go`

**Effort Estimate**: S

---

### [x] **Ticket 3.2: Add worktree show command**

**As a** developer
**I want** detailed status of a specific worktree
**So that** I can diagnose issues and understand its state

**Context**: List shows summary; sometimes more detail is needed for debugging.

**Description**: Add `orbital worktree show <name>` command displaying detailed worktree information.

**Implementation Requirements**:
- Add `show` subcommand to worktree command
- Display: basic info, git status, last commit, divergence from original branch
- Show uncommitted changes if present
- Handle missing worktree gracefully

**Acceptance Criteria**:
- [x] Given worktree name, when `orbital worktree show <name>` runs, then detailed info displayed
- [x] Given uncommitted changes, when shown, then changes are listed
- [x] Given diverged branches, when shown, then commit difference is shown
- [x] Given worktree not found, when show runs, then error says "worktree not found"

**Definition of Done**:
- [x] `orbital worktree show` command implemented
- [x] Displays git status and divergence
- [x] Tests for show command

**Dependencies**: Ticket 3.1 (shared worktree command infrastructure)

**Files to Modify**:
- `cmd/orbital/worktree_cmd.go`

**Effort Estimate**: S

---

### [x] **Ticket 3.3: Add worktree remove command**

**As a** developer
**I want** to manually remove a specific worktree
**So that** I can abandon work without completing it

**Context**: Currently worktrees can only be removed via merge completion. No way to abandon.

**Description**: Add `orbital worktree remove <name>` command to remove a specific worktree and its branch.

**Implementation Requirements**:
- Add `remove` subcommand to worktree command
- Check for uncommitted changes before removal
- Require confirmation (unless `--force`)
- Remove worktree, delete branch, update state file

**Acceptance Criteria**:
- [x] Given worktree name, when `orbital worktree remove <name>` runs, then worktree is removed
- [x] Given uncommitted changes, when remove runs, then confirmation required
- [x] Given `--force` flag, when remove runs, then no confirmation asked
- [x] Given removal succeeds, when complete, then state file is updated

**Definition of Done**:
- [x] `orbital worktree remove` command implemented
- [x] Uncommitted change detection
- [x] Force flag support
- [x] Tests for remove command

**Dependencies**: Ticket 3.1 (shared worktree command infrastructure)

**Files to Modify**:
- `cmd/orbital/worktree_cmd.go`

**Effort Estimate**: S

---

### [x] **Ticket 3.4: Add worktree cleanup command**

**As a** developer
**I want** to find and remove orphan worktrees
**So that** I can clean up stale entries that accumulate

**Context**: If cleanup fails partway or worktrees are manually deleted, orphans remain. No way to identify and clean these up.

**Description**: Add `orbital worktree cleanup` command that detects and removes orphaned worktrees and branches.

**Implementation Requirements**:
- Add `cleanup` subcommand
- Detect: state entries for non-existent paths, git worktrees not in state, orphan `orbital/*` branches
- Support `--dry-run` to show what would be cleaned
- Support `--force` to skip confirmation
- Clean up orphans interactively by default

**Acceptance Criteria**:
- [x] Given orphan worktrees exist, when `orbital worktree cleanup` runs, then they are listed
- [x] Given state has entries for missing paths, when cleanup runs, then entries are removed
- [x] Given `--dry-run`, when cleanup runs, then actions shown but not executed
- [x] Given `--force`, when cleanup runs, then no confirmation asked

**Definition of Done**:
- [x] `orbital worktree cleanup` command implemented
- [x] Orphan detection for state, git, and branches
- [x] Dry-run and force flags
- [x] Tests for cleanup command

**Dependencies**: Ticket 3.1 (shared worktree command infrastructure)

**Files to Modify**:
- `cmd/orbital/worktree_cmd.go`

**Effort Estimate**: M

---

### [x] **Ticket 3.5: Add worktree selection on resume**

**As a** developer with multiple active worktrees
**I want** to choose which worktree to resume
**So that** I can work on multiple features in parallel

**Context**: `continue.go` always uses the first worktree when multiple exist. No selection mechanism.

**Description**: Modify `orbital continue` to prompt for worktree selection when multiple exist.

**Implementation Requirements**:
- Detect multiple worktrees in continue command
- Display selection prompt with name, branch, spec files
- Support `--continue-worktree` flag for direct selection
- Support `--non-interactive` flag that errors if selection needed
- Auto-select when only one worktree exists

**Acceptance Criteria**:
- [x] Given multiple worktrees, when `orbital continue` runs, then user prompted to select
- [x] Given `--continue-worktree` flag, when continue runs, then specified worktree used
- [x] Given one worktree, when continue runs, then auto-selected
- [x] Given `--non-interactive` with multiple, when continue runs, then error with list

**Definition of Done**:
- [x] Selection logic implemented in continue.go
- [x] Flag support for direct selection and non-interactive mode
- [x] Tests for selection logic

**Dependencies**: Ticket 3.1 (for display format consistency), Ticket 3.4 (cleanup should exist for error messages)

**Files to Modify**:
- `cmd/orbital/continue.go`
- `cmd/orbital/root.go`

**Effort Estimate**: S

---

## Backlog Prioritisation

**Sprint 1 (Quick Wins)**:
- [x] Ticket 1.1: Wire resume validation (XS)
- [x] Ticket 2.1: Add default agents constant (XS)
- [x] Ticket 3.1: Add worktree list command (S)

**Sprint 2 (Core Functionality)**:
- [x] Ticket 2.2: Merge default agents with user config (S)
- [x] Ticket 3.2: Add worktree show command (S)
- [x] Ticket 3.3: Add worktree remove command (S)

**Sprint 3 (Polish)**:
- [x] Ticket 3.4: Add worktree cleanup command (M)
- [x] Ticket 3.5: Add worktree selection on resume (S)

## Total Effort

| Estimate | Count | Description |
|----------|-------|-------------|
| XS | 2 | Resume validation, default agents constant |
| S | 5 | Agent merge, list, show, remove, selection |
| M | 1 | Cleanup command |

**Total**: ~3-4 days of focused work

## Success Metrics

- All worktree commands functional and tested
- Default agent available without configuration
- Resume validates worktree before proceeding
- Multi-worktree workflows fully supported
