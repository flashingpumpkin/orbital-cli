# User Stories: Autonomous Preset

## Project Overview

Orbital CLI runs Claude Code in a loop for autonomous iteration. This story set adds a new `autonomous` workflow preset that takes a self-directed approach: study context, pick the highest-leverage task, complete it, document decisions, commit, and exit for the next iteration.

The implementation requires extending the workflow engine to support deferred steps (steps that only execute when jumped to via a gate's OnFail) and adding the new preset with three steps: implement, fix (deferred), and review (gate).

## Story Mapping Overview

**Epic 1: Workflow Engine Enhancement** - Extend Step struct and executor for deferred steps

**Epic 2: Autonomous Preset** - Add the new preset with implement, fix, and review steps

**Epic 3: Documentation** - Update project documentation

---

## Epic 1: Workflow Engine Enhancement

### [x] **Ticket 1.1: Add Deferred Field to Step Struct**

**As a** workflow author
**I want** to mark steps as deferred
**So that** they are skipped during normal execution and only run when a gate jumps to them

**Context**: The current workflow engine executes all steps sequentially. For the autonomous preset, we need a "fix" step that only runs after a review gate fails, not on the initial pass through the workflow.

**Description**: Add a `Deferred bool` field to the `Step` struct in `internal/workflow/workflow.go`. This field indicates that the step should be skipped during normal sequential execution.

**Acceptance Criteria**:
- [x] Given a Step struct, when Deferred is set to true, then it serialises correctly to TOML
- [x] Given a Step struct, when Deferred is set to true, then it serialises correctly to JSON
- [x] Given a Step struct, when Deferred is false, then it is omitted from JSON output

**Definition of Done**:
- [x] Deferred field added to Step struct with `toml:"deferred"` and `json:"deferred,omitempty"` tags
- [x] Existing workflow_test.go tests still pass
- [x] Code compiles without errors

**Dependencies**: None

**Notes**: Data structure change only; execution behaviour comes in ticket 1.2.

**Effort Estimate**: XS

---

### [x] **Ticket 1.2: Executor Skips Deferred Steps**

**As a** workflow author
**I want** deferred steps to be skipped during normal execution
**So that** they only run when explicitly jumped to via OnFail

**Context**: With the Deferred field in place, the executor needs to respect it. Deferred steps should be skipped unless the executor arrived at them via a gate's OnFail jump.

**Description**: Modify the `Run` method in `internal/workflow/executor.go` to track whether the current step was reached via OnFail. Skip deferred steps unless arrived via OnFail.

**Acceptance Criteria**:
- [x] Given a workflow with a deferred step, when executed normally, then the deferred step is skipped
- [x] Given a workflow with a deferred step, when a gate fails and OnFail points to the deferred step, then the deferred step executes
- [x] Given a workflow with multiple deferred steps, when executed normally, then all deferred steps are skipped
- [x] Given a workflow with non-deferred steps only, when executed, then behaviour is unchanged

**Definition of Done**:
- [x] `arrivedViaOnFail` tracking variable added to Run method
- [x] Deferred steps skipped during normal execution
- [x] Deferred steps execute when jumped to via OnFail
- [x] New tests in executor_test.go cover all acceptance criteria
- [x] Existing executor tests still pass

**Dependencies**: Ticket 1.1

**Notes**: The `arrivedViaOnFail` flag must be reset after each step check to prevent incorrect persistence.

**Effort Estimate**: S

---

### [x] **Ticket 1.3: Validate Deferred Step Reachability**

**As a** workflow author
**I want** validation to error if deferred steps are unreachable
**So that** I do not accidentally create workflows where deferred steps never execute

**Context**: A deferred step that is not targeted by any gate's OnFail will never execute. This is likely a configuration mistake.

**Description**: Add validation in `Validate()` method to check that deferred steps are targeted by at least one OnFail.

**Acceptance Criteria**:
- [x] Given a workflow with a deferred step targeted by OnFail, when validated, then no error is returned
- [x] Given a workflow with a deferred step not targeted by any OnFail, when validated, then an error is returned
- [x] Given a workflow with no deferred steps, when validated, then behaviour is unchanged

**Definition of Done**:
- [x] Validation logic added to Validate() method
- [x] Error message clearly identifies the unreachable deferred step
- [x] New tests in workflow_test.go cover all acceptance criteria
- [x] Existing validation tests still pass

**Dependencies**: Ticket 1.1

**Notes**: Can be implemented in parallel with Ticket 1.2.

**Effort Estimate**: XS

---

## Epic 2: Autonomous Preset

### [x] **Ticket 2.1: Add PresetAutonomous Constant**

**As a** CLI user
**I want** to select the autonomous preset by name
**So that** I can use the self-directed workflow

**Context**: The preset system uses string constants for preset names. The new preset needs its constant and registration.

**Description**: Add `PresetAutonomous` constant to `internal/workflow/presets.go` and update related functions.

**Acceptance Criteria**:
- [x] Given the string "autonomous", when IsValidPreset is called, then it returns true
- [x] Given ValidPresets(), when called, then it includes PresetAutonomous
- [x] Given PresetDescriptions(), when called, then it includes autonomous with description "Self-directed task selection with review gate"

**Definition of Done**:
- [x] `PresetAutonomous PresetName = "autonomous"` constant added
- [x] ValidPresets() updated to include PresetAutonomous
- [x] PresetDescriptions() updated with autonomous entry
- [x] GetPreset() has case for PresetAutonomous (returns error until 2.2)
- [x] New tests in presets_test.go verify preset recognition

**Dependencies**: None (can be done in parallel with Epic 1)

**Notes**: GetPreset can return an error initially; the workflow is added in ticket 2.2.

**Effort Estimate**: XS

---

### [x] **Ticket 2.2: Implement autonomousPreset Function**

**As a** CLI user
**I want** the autonomous preset to provide a complete workflow
**So that** I can run self-directed iterations with review gates

**Context**: The autonomous preset needs three steps: implement (main work), fix (deferred, handles review feedback), and review (gate with OnFail pointing to fix).

**Description**: Implement the `autonomousPreset()` function in `internal/workflow/presets.go` that returns the complete workflow configuration.

**Acceptance Criteria**:
- [x] Given GetPreset(PresetAutonomous), when called, then it returns a valid workflow with no error
- [x] Given the returned workflow, when inspected, then it has three steps named "implement", "fix", "review"
- [x] Given the fix step, when inspected, then Deferred is true
- [x] Given the review step, when inspected, then Gate is true and OnFail is "fix"
- [x] Given the implement step prompt, when inspected, then it contains {{files}} placeholder
- [x] Given the workflow, when Validate() is called, then it passes

**Definition of Done**:
- [x] autonomousPreset() function implemented with three steps
- [x] implement step has prompt for studying spec, picking task, completing, documenting, committing
- [x] fix step marked deferred with prompt for addressing review feedback
- [x] review step uses rigorousReviewPrompt, marked as gate with OnFail="fix"
- [x] GetPreset(PresetAutonomous) returns the workflow
- [x] New tests in presets_test.go verify preset structure and prompts

**Dependencies**: Ticket 1.1 (Deferred field), Ticket 2.1 (constant)

**Notes**: Prompts should be task-agnostic, not specific to any particular type of work.

**Effort Estimate**: S

---

## Epic 3: Documentation

### [x] **Ticket 3.1: Update CLAUDE.md Workflow Presets Table**

**As a** developer working on the project
**I want** CLAUDE.md to document the autonomous preset
**So that** the project documentation is accurate

**Context**: CLAUDE.md contains a workflow presets table that needs updating.

**Description**: Add autonomous preset to the workflow presets table in CLAUDE.md.

**Acceptance Criteria**:
- [x] Given CLAUDE.md, when read, then it includes autonomous in the presets table
- [x] Given the table entry, then it shows: autonomous | implement, fix (deferred), review (gate) | Self-directed task selection with review gate

**Definition of Done**:
- [x] CLAUDE.md workflow presets table updated with autonomous row
- [x] Description matches actual preset behaviour

**Dependencies**: Ticket 2.2

**Effort Estimate**: XS

---

## Backlog Prioritisation

**Must Have:**
- Ticket 1.1: Add Deferred Field to Step Struct
- Ticket 1.2: Executor Skips Deferred Steps
- Ticket 2.1: Add PresetAutonomous Constant
- Ticket 2.2: Implement autonomousPreset Function

**Should Have:**
- Ticket 1.3: Validate Deferred Step Reachability
- Ticket 3.1: Update CLAUDE.md Workflow Presets Table

**Could Have:**
- None

**Won't Have:**
- README.md updates (no preset documentation exists there)

## Technical Considerations

The deferred step mechanism extends the existing workflow model minimally. It reuses the OnFail jump mechanism and adds a skip condition for deferred steps during normal execution. This keeps the change localised and backwards compatible.

The `arrivedViaOnFail` tracking variable is local to the Run method and requires no changes to the Runner struct or persistent state.

## Success Metrics

- `--preset autonomous` selects the new preset
- First iteration executes implement then review (skipping fix)
- If review fails, fix executes then review executes again
- Existing presets work unchanged
- All package tests pass
