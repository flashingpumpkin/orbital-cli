# PRD: Workflow Gate Reliability

**Status:** Draft
**Author:** Claude
**Created:** 2026-01-25
**Priority:** High

## Problem Statement

The workflow gate system fails to enforce review feedback. When using multi-step workflows (reviewed, tdd, fast), the review step's findings are frequently ignored, allowing code with identified issues to pass through to completion.

Users report that "the loop keeps ignoring the reviews produced in workflows and skipping ahead."

## Background

Orbital supports multi-step workflows where gate steps act as quality checkpoints. The current flow for the `reviewed` preset:

```
implement → review (gate) → [if FAIL, back to implement] → verification → done
```

The problem: looping back to `implement` is wrong. The implement step's job is to implement features from the spec. When review finds issues, we need a dedicated step that reads review feedback and addresses it.

## Root Cause Analysis

### 1. GateNotFound Handling Is Broken

**Location:** `internal/workflow/executor.go:223-230`

When the review step fails to output a gate signal (`<gate>PASS</gate>` or `<gate>FAIL</gate>`), the current behaviour:

1. Retries the **same step** (review) up to 3 times
2. Never loops back to the `on_fail` target
3. After max retries, returns `ErrMaxGateRetriesExceeded`
4. The outer loop catches this error and **continues to next iteration**
5. Review feedback is lost

**Current code:**
```go
case GateNotFound:
    gateRetries[step.Name]++
    if gateRetries[step.Name] >= r.workflow.EffectiveMaxGateRetries() {
        return result, fmt.Errorf(...)
    }
    // BUG: Does nothing - stepIndex not changed, step retried
```

**Contrast with GateFailed:**
```go
case GateFailed:
    gateRetries[step.Name]++
    if gateRetries[step.Name] >= r.workflow.EffectiveMaxGateRetries() {
        return result, fmt.Errorf(...)
    }
    if step.OnFail != "" {
        targetIndex := r.workflow.GetStepIndex(step.OnFail)
        stepIndex = targetIndex  // LOOPS BACK
    }
```

### 2. Wrong Loop Target

The current workflow presets loop back to `implement` on review failure:

```go
{
    Name:   "review",
    Gate:   true,
    OnFail: "implement",  // WRONG TARGET
}
```

The `implement` step is designed to implement the next spec item. When review fails, we don't want to implement more features; we want to **address the review feedback**.

Looping to `implement` causes:
- Implement sees all items already marked `[x]`, does nothing
- Review runs again on unchanged code
- Review passes (no new changes to review) or loops indefinitely

### The Fatal Flow

```
implement (marks items [x])
    ↓
review (finds issues, outputs FAIL or no signal)
    ↓
Loop back to implement (wrong target)
    ↓
implement: all items [x], nothing to do
    ↓
review: no changes, outputs PASS
    ↓
Verification passes (all items [x])
    ↓
Done - review issues NEVER ADDRESSED
```

## Proposed Solution: Introduce `address-review` Step

Add a dedicated step that runs only after review fails. This step reads the review feedback from the notes file and addresses the issues.

### New Workflow Structure

```
implement → review (gate)
                ↓ FAIL
            address-review → review (gate)
                                ↓ PASS
                            verification → done
```

The `address-review` step:
- Reads review feedback from the notes file
- Addresses the identified issues
- Does NOT implement new features
- Loops back to review for re-evaluation

## Requirements

### R1: GateNotFound Must Use on_fail Target

**Priority:** P0 (Critical)

When a gate step does not output a gate signal:

1. The workflow must loop back to the `on_fail` step (if specified)
2. The retry counter must still increment
3. After max retries, the workflow must fail with clear error message

**Acceptance Criteria:**
- [ ] `GateNotFound` loops back to `on_fail` step like `GateFailed`
- [ ] Unit tests verify `GateNotFound` triggers `on_fail` navigation
- [ ] Integration test: review without gate signal loops to `address-review`

### R2: Add `address-review` Step to Workflow Presets

**Priority:** P0 (Critical)

Update workflow presets to include a dedicated step for addressing review feedback:

1. Add `address-review` step between `implement` and `review`
2. Set review's `on_fail` to `address-review` (not `implement`)
3. `address-review` proceeds to `review` for re-evaluation

**Acceptance Criteria:**
- [ ] All gated presets (fast, reviewed, tdd) include `address-review` step
- [ ] `address-review` prompt reads notes file for feedback
- [ ] `address-review` prompt focuses on fixing issues, not implementing features
- [ ] Review's `on_fail` points to `address-review`

### R3: Add Gate Signal Fallback Warning

**Priority:** P2 (Medium)

When no gate signal is found:

1. Log a warning to the user
2. Treat as FAIL (conservative default)
3. Include guidance on what went wrong

**Acceptance Criteria:**
- [ ] Warning message printed when gate signal missing
- [ ] Message suggests checking review prompt output
- [ ] Behaviour defaults to FAIL, not silent retry

## Proposed Implementation

### Phase 1: Fix GateNotFound (P0)

**File:** `internal/workflow/executor.go`

```go
case GateNotFound:
    // Warn about missing gate signal
    fmt.Printf("Warning: step %q did not output gate signal\n", step.Name)

    gateRetries[step.Name]++
    if gateRetries[step.Name] >= r.workflow.EffectiveMaxGateRetries() {
        return result, fmt.Errorf("%w: step %q did not output gate signal after %d attempts",
            ErrMaxGateRetriesExceeded, step.Name, gateRetries[step.Name])
    }

    // Loop back to on_fail step (same as GateFailed)
    if step.OnFail != "" {
        targetIndex := r.workflow.GetStepIndex(step.OnFail)
        if targetIndex < 0 {
            return result, fmt.Errorf("step %q: on_fail target %q not found",
                step.Name, step.OnFail)
        }
        stepIndex = targetIndex
    }
    // If no on_fail, retry same step (existing behaviour)
```

### Phase 2: Update Workflow Presets (P0)

**File:** `internal/workflow/presets.go`

#### New address-review prompt:

```go
const addressReviewPrompt = `The previous code review identified issues that must be addressed.

1. Read the notes file for the review feedback
2. For each issue identified:
   - Locate the problematic code
   - Implement the fix
   - Verify the fix addresses the concern
3. Run tests to ensure fixes don't break existing functionality
4. Update the notes file with what was fixed

Focus ONLY on addressing review feedback. Do not implement new features.
Do not mark any spec items as complete.
Do not output the completion promise.`
```

#### Updated reviewed preset:

```go
func reviewedPreset() *Workflow {
    return &Workflow{
        Name:   string(PresetReviewed),
        Preset: string(PresetReviewed),
        Steps: []Step{
            {
                Name: "implement",
                Prompt: `Continue implementing the requirements in {{files}}.
Focus on the next incomplete item.
Do not output completion promise yet.`,
            },
            {
                Name:   "review",
                Prompt: rigorousReviewPrompt,
                Gate:   true,
                OnFail: "address-review",  // NEW: loop to address-review, not implement
            },
            {
                Name:   "address-review",
                Prompt: addressReviewPrompt,
                // Not a gate - proceeds directly to review
            },
        },
    }
}
```

#### Updated fast preset:

```go
func fastPreset() *Workflow {
    return &Workflow{
        Name:   string(PresetFast),
        Preset: string(PresetFast),
        Steps: []Step{
            {
                Name: "implement",
                Prompt: `Implement as many requirements as possible from {{files}} in this iteration.
Do not work incrementally. Tackle multiple requirements at once.
Do not output completion promise yet.`,
            },
            {
                Name:   "review",
                Prompt: rigorousReviewPrompt,
                Gate:   true,
                OnFail: "address-review",
            },
            {
                Name:   "address-review",
                Prompt: addressReviewPrompt,
            },
        },
    }
}
```

#### Updated tdd preset:

```go
func tddPreset() *Workflow {
    return &Workflow{
        Name:   string(PresetTDD),
        Preset: string(PresetTDD),
        Steps: []Step{
            {
                Name:   "red",
                Prompt: `Write a failing test for the next requirement in {{files}}.
Run the test to confirm it fails.`,
            },
            {
                Name:   "green",
                Prompt: `Write the minimal code to make the failing test pass.
Run the test to confirm it passes.`,
            },
            {
                Name:   "refactor",
                Prompt: `Refactor the code while keeping tests green.
Run tests to confirm they still pass.`,
            },
            {
                Name:   "review",
                Prompt: rigorousReviewPrompt,
                Gate:   true,
                OnFail: "address-review",
            },
            {
                Name:   "address-review",
                Prompt: addressReviewPrompt,
            },
        },
    }
}
```

### Corrected Workflow Flow

After implementation:

```
implement
    ↓
review (gate)
    ↓ PASS → verification → done
    ↓ FAIL
address-review
    ↓
review (gate)
    ↓ PASS → verification → done
    ↓ FAIL
address-review
    ↓
... (up to max gate retries)
```

## Testing Strategy

### Unit Tests

1. **GateNotFound navigation:** Verify `GateNotFound` triggers `on_fail` step navigation
2. **Workflow structure:** Verify all gated presets have `address-review` step
3. **on_fail targets:** Verify review's `on_fail` points to `address-review`

### Integration Tests

1. **Review FAIL loops correctly:** Verify gate FAIL loops to `address-review`, not `implement`
2. **Review missing signal:** Verify warning printed and loops to `address-review`
3. **address-review proceeds to review:** Verify `address-review` step goes back to `review`
4. **Full workflow:** Verify reviewed preset completes only when review passes

### Manual Testing

1. Run `orbital --workflow reviewed spec.md` with code that has obvious issues
2. Verify review step outputs FAIL
3. Verify `address-review` step runs (not `implement`)
4. Verify `address-review` reads and addresses feedback
5. Verify review runs again after `address-review`
6. Verify completion only occurs after review passes

## Success Metrics

1. **Correct loop target:** 100% of review failures loop to `address-review`
2. **Issue resolution:** Code issues identified by review are addressed before completion
3. **No false completions:** Zero cases where review issues are ignored

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| `address-review` step doesn't read notes | Medium | High | Explicit prompt instructions to read notes file |
| Infinite loop between review and address-review | Low | Medium | Max gate retries limit already exists |
| Breaking existing custom workflows | Low | Medium | Only affects preset workflows, custom workflows unchanged |

## Timeline

| Phase | Scope | Estimate |
|-------|-------|----------|
| Phase 1 | Fix GateNotFound | 1-2 hours |
| Phase 2 | Update workflow presets | 2-3 hours |
| Testing | Unit + integration | 3-4 hours |
| **Total** | | **6-9 hours** |

## Open Questions

1. Should `address-review` be a gate step? Current proposal: No, it proceeds directly to review which is the gate.

2. Should TDD preset's `on_fail` go to `refactor` or `address-review`? Current proposal: `address-review` for consistency, since review issues may not be refactoring-related.

3. Should there be a separate retry counter for the `address-review → review` loop vs the initial review? Current proposal: Use existing gate retry counter.

## References

- `internal/workflow/executor.go` - Workflow runner implementation
- `internal/workflow/presets.go` - Workflow preset definitions
- `internal/workflow/gate.go` - Gate signal detection
- `cmd/orbital/root.go:736-973` - runWorkflowLoop implementation
