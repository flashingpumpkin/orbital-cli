# User Stories: Worktree Implementation Fixes

## Project Overview

Orbital CLI's worktree feature provides isolated development environments using git worktrees. An in-depth review identified critical issues that prevent worktrees from functioning correctly. The issues centre on two problems: Claude executes in the wrong directory during work phases, and branch names become corrupted during cleanup. This plan addresses these issues through a simplified architecture that uses deterministic naming instead of Claude-generated names.

**Target users**: Developers using Orbital CLI with the `--worktree` flag for isolated development work.

**Business value**: Reliable worktree isolation prevents accidental changes to the main repository and enables parallel development workflows.

## Story Mapping Overview

**Epic 1: Deterministic Worktree Naming** (P0)
Replace Claude-based naming with local adjective-animal generation to eliminate marker extraction bugs.

**Epic 2: Working Directory Enforcement** (P0)
Ensure Claude CLI executes in the worktree directory during work phases.

**Epic 3: Simplified Setup Phase** (P1)
Remove Claude invocation from setup, create worktrees directly with git commands.

**Epic 4: Error Handling Improvements** (P1)
Add validation and verbose error messages for debugging.

---

## Epic 1: Deterministic Worktree Naming

### [x] **Ticket 1.1: Create adjective-animal name generator**

**As a** developer using worktree mode
**I want** worktree names to be generated locally using memorable adjective-animal combinations
**So that** I can easily reference and discuss worktrees without relying on fragile marker extraction

**Context**: The current implementation asks Claude to generate worktree names, then parses markers from stream-json output. This parsing is fragile and causes branch name corruption (e.g., `orbit/fix-formattingsuccess`). Local generation eliminates this entire class of bugs.

**Description**: Create a new file `internal/worktree/names.go` containing two exported functions: `GenerateName()` which returns a random adjective-animal combination, and `GenerateUniqueName(excluded []string)` which returns a name not in the excluded list. Use crypto/rand for randomness.

**Implementation Requirements**:
- Create `internal/worktree/names.go` with 50 adjectives and 50 animals
- Implement `GenerateName()` returning format `adjective-animal`
- Implement `GenerateUniqueName(excluded []string)` with collision avoidance
- Use `crypto/rand` for secure random selection
- Add fallback suffix (e.g., `swift-falcon-2`) if retries exhausted

**Acceptance Criteria**:
- [x] Given the names package is imported, when `GenerateName()` is called, then it returns a string in format `adjective-animal` (lowercase, hyphenated)
- [x] Given the names package is imported, when `GenerateName()` is called multiple times, then it returns different values (randomness works)
- [x] Given an excluded list containing `["swift-falcon"]`, when `GenerateUniqueName(excluded)` is called, then it returns a name not in the excluded list
- [x] Given all 2500 combinations are excluded, when `GenerateUniqueName(excluded)` is called, then it returns a name with numeric suffix
- [x] Given no exclusions, when `GenerateUniqueName(nil)` is called, then it returns a valid adjective-animal name

**Definition of Done** (Single Commit):
- [x] `internal/worktree/names.go` created with all 50 adjectives and 50 animals
- [x] `internal/worktree/names_test.go` created with table-driven tests
- [x] All tests passing (`go test ./internal/worktree/...`)
- [x] Code follows project style (gofmt, descriptive names, exported function comments)

**Dependencies**: None

**Risks**: None (isolated new code)

**Notes**:
Adjectives: agile, bold, brave, bright, brisk, calm, clever, cool, cosmic, crisp, dapper, daring, deft, eager, epic, fair, fancy, fast, fierce, fleet, gentle, glad, golden, grand, great, happy, hardy, hasty, honest, humble, jolly, keen, kind, lively, lucky, merry, mighty, modest, noble, plucky, proud, quick, quiet, rapid, ready, sharp, sleek, smart, swift, wise

Animals: badger, bear, beaver, bison, bobcat, cheetah, condor, cougar, coyote, crane, deer, dolphin, eagle, elk, falcon, ferret, finch, fox, gazelle, gecko, hawk, heron, horse, husky, jaguar, kestrel, koala, lemur, leopard, lion, lynx, marten, moose, otter, owl, panther, parrot, pelican, puma, raven, salmon, seal, shark, sparrow, stork, swift, tiger, turtle, viper, wolf

**Effort Estimate**: S

---

### [x] **Ticket 1.2: Create git worktree helper functions**

**As a** developer using worktree mode
**I want** worktree creation to happen via direct git commands
**So that** the setup phase does not require Claude invocation or marker parsing

**Context**: Currently, setup invokes Claude to run `git worktree add`. This requires parsing markers from output to extract the path and branch name. Direct git execution with known names eliminates parsing entirely.

**Description**: Create helper functions in `internal/worktree/git.go` that execute git commands directly: `CreateWorktree(dir, name string)` creates a worktree and branch, `RemoveWorktree(dir, path string)` removes a worktree, and `DeleteBranch(dir, branch string)` deletes a branch.

**Implementation Requirements**:
- Create `internal/worktree/git.go` with git helper functions
- Implement `CreateWorktree(dir, name string) error` running `git worktree add -b orbital/<name> .orbital/worktrees/<name> HEAD`
- Implement `RemoveWorktree(dir, path string) error` running `git worktree remove <path> --force`
- Implement `DeleteBranch(dir, branch string) error` running `git branch -d <branch>` with `-D` fallback
- Return wrapped errors with git output for debugging

**Acceptance Criteria**:
- [x] Given a valid git repository, when `CreateWorktree(dir, "swift-falcon")` is called, then a worktree is created at `.orbital/worktrees/swift-falcon` with branch `orbital/swift-falcon`
- [x] Given a worktree exists, when `RemoveWorktree(dir, path)` is called, then the worktree directory is removed
- [x] Given a branch exists, when `DeleteBranch(dir, branch)` is called, then the branch is deleted
- [x] Given a branch is not fully merged, when `DeleteBranch(dir, branch)` is called, then it falls back to force delete
- [x] Given git commands fail, when any function is called, then the error includes git output for debugging

**Definition of Done** (Single Commit):
- [x] `internal/worktree/git.go` created with helper functions
- [x] `internal/worktree/git_test.go` created (may skip integration tests requiring real git repo)
- [x] All tests passing
- [x] Error messages include git command output

**Dependencies**: None

**Risks**: Integration testing requires a real git repository; unit tests can mock exec.Command

**Notes**: These functions replace the current `Cleanup.Run()` logic in `merge.go` with reusable helpers.

**Effort Estimate**: S

---

### [x] **Ticket 1.3: Integrate name generator into setup flow**

**As a** developer using worktree mode
**I want** the setup phase to generate names locally and create worktrees directly
**So that** setup is faster, cheaper, and does not require marker parsing

**Context**: The current setup phase invokes Claude to generate a name and create the worktree, then parses markers from output. This story replaces that flow with local name generation and direct git commands.

**Description**: Modify `cmd/orbital/root.go` to use `worktree.GenerateUniqueName()` for name generation and `worktree.CreateWorktree()` for creation. Remove the Claude invocation from the setup phase. Keep the `--worktree-name` flag for manual override.

**Implementation Requirements**:
- Modify `runWorktreeSetup()` to generate name locally using `GenerateUniqueName()`
- Call `CreateWorktree()` directly instead of invoking Claude
- Load existing worktree names from state file to pass as exclusions
- Preserve `--worktree-name` flag functionality for manual override
- Update `SetupResult` to not require Claude execution results

**Acceptance Criteria**:
- [x] Given `--worktree` flag is used, when setup runs, then a worktree is created with an adjective-animal name without invoking Claude
- [x] Given `--worktree-name my-feature` is specified, when setup runs, then the worktree is created with name `my-feature`
- [x] Given existing worktrees `["swift-falcon", "calm-otter"]` in state, when setup runs, then the generated name is not in the exclusion list
- [x] Given worktree creation succeeds, when setup completes, then state is persisted with the generated name
- [x] Given worktree creation fails, when setup runs, then the error is propagated with context

**Definition of Done** (Single Commit):
- [x] `runWorktreeSetup()` modified to use local name generation
- [x] Claude invocation removed from setup phase
- [x] `--worktree-name` flag continues to work
- [x] Tests updated or added for new flow
- [x] All tests passing

**Dependencies**: Ticket 1.1, Ticket 1.2

**Risks**: Changing the setup flow affects worktree state persistence; ensure state format remains compatible

**Notes**: This eliminates the setup phase cost (~$0.01 per run) and removes the source of Issue 2 (branch name corruption).

**Effort Estimate**: S

---

### [x] **Ticket 1.4: Remove obsolete setup code**

**As a** maintainer of Orbital CLI
**I want** unused setup phase code removed
**So that** the codebase is simpler and easier to maintain

**Context**: After integrating local name generation, the Claude-based setup code (prompt building, marker extraction) is no longer needed.

**Description**: Remove `buildSetupPrompt()`, `extractWorktreePath()`, `extractBranchName()`, and `extractMarker()` from `internal/worktree/setup.go`. Remove or update associated tests. Keep the `Setup` struct if needed for other purposes, or remove entirely.

**Implementation Requirements**:
- Remove `buildSetupPrompt()` function
- Remove `extractWorktreePath()` function
- Remove `extractBranchName()` function
- Remove `extractMarker()` function
- Remove or update tests in `setup_test.go`
- Remove `worktreeExecutorAdapter` if no longer used

**Acceptance Criteria**:
- [x] Given the codebase, when searching for `buildSetupPrompt`, then no results are found
- [x] Given the codebase, when searching for `extractMarker`, then no results are found
- [x] Given `go build ./...`, when executed, then it succeeds without errors
- [x] Given `go test ./...`, when executed, then all tests pass

**Definition of Done** (Single Commit):
- [x] Obsolete functions removed from `setup.go`
- [x] Tests removed or updated in `setup_test.go`
- [x] No compilation errors
- [x] All remaining tests pass

**Dependencies**: Ticket 1.3

**Risks**: Ensure no other code paths depend on removed functions

**Notes**: This is cleanup work after the main integration is complete.

**Effort Estimate**: XS

---

## Epic 2: Working Directory Enforcement

### [x] **Ticket 2.1: Set cmd.Dir in executor**

**As a** developer using worktree mode
**I want** Claude CLI to execute in the worktree directory
**So that** all file operations happen in the isolated worktree, not the main repository

**Context**: The executor never sets `cmd.Dir`, so Claude CLI runs in Orbital's current working directory regardless of config. When `--worktree` is used, Claude should operate in the worktree directory.

**Description**: Modify `internal/executor/executor.go` to set `cmd.Dir` to `config.WorkingDir` before executing the command. This ensures Claude CLI operates in the correct directory during worktree mode.

**Implementation Requirements**:
- Modify `Execute()` to set `cmd.Dir = e.config.WorkingDir` when WorkingDir is set and not "."
- Ensure the directory exists before setting (or let exec fail with clear error)
- Add test verifying cmd.Dir is set correctly

**Acceptance Criteria**:
- [x] Given `config.WorkingDir = "/path/to/worktree"`, when `Execute()` is called, then Claude CLI runs with working directory `/path/to/worktree`
- [x] Given `config.WorkingDir = "."`, when `Execute()` is called, then Claude CLI runs in the current directory (default behaviour)
- [x] Given `config.WorkingDir = ""`, when `Execute()` is called, then Claude CLI runs in the current directory (default behaviour)
- [x] Given the working directory does not exist, when `Execute()` is called, then an error is returned

**Definition of Done** (Single Commit):
- [x] `executor.go` modified to set `cmd.Dir`
- [x] Test added verifying working directory is set
- [x] All tests passing
- [x] Manual verification with `--worktree` flag

**Dependencies**: None

**Risks**: Could affect non-worktree execution if WorkingDir is incorrectly set elsewhere; verify default behaviour is preserved

**Notes**: This is a small change with significant impact on worktree correctness.

**Effort Estimate**: XS

---

### [x] **Ticket 2.2: Remove directory navigation from merge prompt**

**As a** developer using worktree mode
**I want** the merge phase to rely on working directory, not prompt instructions
**So that** merge behaviour is consistent and not dependent on Claude following navigation instructions

**Context**: The merge prompt currently instructs Claude to navigate directories ("Navigate to the worktree directory", "Navigate to the main repository"). With working directory enforcement, this is unnecessary.

**Description**: Simplify the merge prompt in `internal/worktree/merge.go` to remove directory navigation instructions. The executor will set the correct working directory, so Claude operates in the right location automatically.

**Implementation Requirements**:
- Modify `buildMergePrompt()` to remove navigation steps
- Keep rebase, merge, and success marker output instructions
- Ensure merge executor is configured with correct working directory (main repo for merge)

**Acceptance Criteria**:
- [x] Given the merge prompt, when inspected, then it does not contain "Navigate to" instructions
- [x] Given merge phase runs, when Claude executes git commands, then they operate in the correct directory without explicit navigation
- [x] Given merge succeeds, when checking output, then `MERGE_SUCCESS: true` marker is present

**Definition of Done** (Single Commit):
- [x] `buildMergePrompt()` simplified
- [x] Tests updated for new prompt format
- [x] All tests passing

**Dependencies**: Ticket 2.1

**Risks**: Merge phase must run in main repository directory, not worktree; verify executor config is correct

**Notes**: This simplifies the merge prompt and makes behaviour more predictable.

**Effort Estimate**: XS

---

## Epic 3: Simplified Setup Phase

### [x] **Ticket 3.1: Update state file format**

**As a** developer using worktree mode
**I want** the worktree state file to include the generated name
**So that** I can correlate worktrees with their adjective-animal identifiers

**Context**: The state file currently stores path, branch, and original branch. With deterministic naming, the state should also store the generated name for easy reference.

**Description**: Update `WorktreeState` struct in `internal/worktree/state.go` to include a `Name` field. Update state persistence and loading to handle the new field. Ensure backwards compatibility with existing state files.

**Implementation Requirements**:
- Add `Name string` field to `WorktreeState` struct
- Update `Add()` to persist the name
- Update state loading to handle missing Name field (backwards compatibility)
- Add `CreatedAt time.Time` field for debugging/display

**Acceptance Criteria**:
- [x] Given a new worktree is created, when state is saved, then the JSON includes `"name": "swift-falcon"`
- [x] Given state is loaded, when the Name field is accessed, then it returns the stored name
- [x] Given an old state file without Name field, when loaded, then Name is empty string (no error)
- [x] Given state is saved, when loaded, then CreatedAt timestamp is preserved

**Definition of Done** (Single Commit):
- [x] `WorktreeState` struct updated with Name and CreatedAt fields
- [x] State persistence updated
- [x] Backwards compatibility verified with test
- [x] All tests passing

**Dependencies**: None

**Risks**: Must maintain backwards compatibility with existing state files

**Notes**: JSON struct tags with `omitempty` ensure backwards compatibility.

**Effort Estimate**: XS

---

### [x] **Ticket 3.2: Display worktree name in TUI**

**As a** developer using worktree mode
**I want** the TUI to display the generated worktree name
**So that** I can easily identify which worktree I am working in

**Context**: The TUI currently shows worktree path and branch. With memorable names, displaying the name (e.g., "swift-falcon") provides better UX than showing the full path.

**Description**: Update the TUI worktree display to show the generated name prominently. Keep the path available for reference but make the name the primary identifier.

**Implementation Requirements**:
- Update `tui.WorktreeInfo` struct to include Name field
- Update TUI rendering to display name (e.g., "Worktree: swift-falcon")
- Update `root.go` to pass name to TUI when creating WorktreeInfo

**Acceptance Criteria**:
- [x] Given worktree mode is active, when TUI renders, then the worktree name is displayed prominently
- [x] Given TUI is rendering, when worktree info is shown, then format is "Worktree: swift-falcon" or similar
- [x] Given no worktree mode, when TUI renders, then no worktree info is displayed

**Definition of Done** (Single Commit):
- [x] `WorktreeInfo` struct updated
- [x] TUI rendering updated
- [x] Visual verification of TUI output
- [x] All tests passing

**Dependencies**: Ticket 3.1

**Risks**: TUI layout may need adjustment to fit new information

**Notes**: Keep it simple; the name should be visible but not dominate the display.

**Effort Estimate**: XS

---

## Epic 4: Error Handling Improvements

### [x] **Ticket 4.1: Add branch name validation**

**As a** developer using worktree mode
**I want** branch names to be validated before git operations
**So that** invalid names are caught early with clear error messages

**Context**: Corrupted branch names (e.g., containing "success") cause confusing git errors. Validation prevents invalid names from reaching git commands.

**Description**: Add a `ValidateBranchName(name string) error` function that checks branch names match expected format. Call this function before any git branch operations.

**Implementation Requirements**:
- Create `ValidateBranchName(name string) error` in `internal/worktree/git.go`
- Validate: must start with `orbital/`, no spaces, no special characters except hyphen and slash
- Call validation in `DeleteBranch()` before executing git command
- Return descriptive error if validation fails

**Acceptance Criteria**:
- [x] Given branch name `orbital/swift-falcon`, when validated, then no error is returned
- [x] Given branch name `orbital/swift-falconsuccess`, when validated, then error indicates invalid format
- [x] Given branch name `swift-falcon` (missing prefix), when validated, then error indicates missing prefix
- [x] Given branch name with spaces, when validated, then error indicates invalid characters

**Definition of Done** (Single Commit):
- [x] `ValidateBranchName()` implemented
- [x] Called from `DeleteBranch()` and `CreateWorktree()`
- [x] Tests added for validation function
- [x] All tests passing

**Dependencies**: Ticket 1.2

**Risks**: Validation regex must not be overly restrictive

**Notes**: This is defence in depth; with local name generation, invalid names should not occur, but validation provides safety.

**Effort Estimate**: XS

---

### [x] **Ticket 4.2: Improve cleanup error messages**

**As a** developer using worktree mode
**I want** cleanup error messages to include the branch name and git output
**So that** I can diagnose cleanup failures

**Context**: Current error message "failed to delete branch: exit status 1" lacks context. Including the branch name and git output makes debugging easier.

**Description**: Update error messages in cleanup functions to include the branch name being deleted and the full git command output.

**Implementation Requirements**:
- Update `DeleteBranch()` error to include branch name
- Update `RemoveWorktree()` error to include worktree path
- Include git stderr/stdout in error messages
- Format errors clearly for terminal display

**Acceptance Criteria**:
- [x] Given branch deletion fails, when error is displayed, then it includes the branch name being deleted
- [x] Given branch deletion fails, when error is displayed, then it includes git command output
- [x] Given worktree removal fails, when error is displayed, then it includes the worktree path
- [x] Given errors are wrapped, when unwrapped, then original error is accessible

**Definition of Done** (Single Commit):
- [x] Error messages updated with context
- [x] Error format verified in tests
- [x] All tests passing

**Dependencies**: Ticket 1.2

**Risks**: None

**Notes**: Already partially implemented in the spec; this ensures consistent error formatting.

**Effort Estimate**: XS

---

## Backlog Prioritisation

**Must Have (Sprint 1):**
- Ticket 1.1: Create adjective-animal name generator
- Ticket 1.2: Create git worktree helper functions
- Ticket 1.3: Integrate name generator into setup flow
- Ticket 2.1: Set cmd.Dir in executor

**Should Have (Sprint 2):**
- Ticket 1.4: Remove obsolete setup code
- Ticket 2.2: Remove directory navigation from merge prompt
- Ticket 3.1: Update state file format
- Ticket 4.1: Add branch name validation
- Ticket 4.2: Improve cleanup error messages

**Could Have (Sprint 3):**
- Ticket 3.2: Display worktree name in TUI

**Won't Have:**
- State file locking for concurrent access (deferred)
- Separate cost tracking for setup/merge phases (deferred)
- Integration tests with real git repositories (deferred)

## Technical Considerations

**Backwards Compatibility**: State file changes must handle existing state files gracefully. Use `omitempty` JSON tags and handle missing fields on load.

**Testing Strategy**: Unit tests for name generation and git helpers. Integration tests for full worktree lifecycle can be added later with a test git repository fixture.

**Error Handling**: All git operations should wrap errors with context. Use `fmt.Errorf("operation failed: %w", err)` pattern consistently.

## Success Metrics

- Worktree cleanup succeeds without branch name corruption errors
- Claude CLI executes in the worktree directory (verified via git status in work output)
- Setup phase cost reduced to zero (no Claude invocation)
- No regression in existing worktree functionality
